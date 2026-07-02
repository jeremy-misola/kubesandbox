package kubernetes

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// liveSession builds a claimed/owned session claim.
func liveSession(name, owner string, created time.Time) *unstructured.Unstructured {
	obj := poolMember(name, created, true)
	labels := obj.GetLabels()
	labels[poolLabel] = poolClaimed
	labels[ownerLabel] = ownerHash(owner)
	obj.SetLabels(labels)
	_ = unstructured.SetNestedField(obj.Object, owner, "spec", "ownerRef")
	return obj
}

// ownerMarkerFixture builds a per-owner marker ConfigMap with a set age.
func ownerMarkerFixture(owner string, created time.Time) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":              markerName(owner),
			"namespace":         "playground",
			"creationTimestamp": created.UTC().Format(time.RFC3339),
			"labels": map[string]interface{}{
				managedByLabel: managedByValue,
				markerLabel:    "true",
			},
		},
		"data": map[string]interface{}{
			markerKeyOwner:  owner,
			markerKeyMember: "",
		},
	}}
}

// poolStateCounts tallies managed claims by pool label.
func poolStateCounts(t *testing.T, svc *SessionService) (available, claimed, total int) {
	t.Helper()
	claims, err := svc.listManaged(context.Background())
	if err != nil {
		t.Fatalf("listManaged: %v", err)
	}
	for i := range claims {
		total++
		switch poolState(&claims[i]) {
		case poolAvailable:
			available++
		case poolClaimed:
			claimed++
		}
	}
	return
}

func newTestPool(t *testing.T, cfg PoolConfig, now time.Time, objs ...*unstructured.Unstructured) (*PoolManager, *SessionService, *AssignQueue) {
	t.Helper()
	ros := make([]runtime.Object, len(objs))
	for i := range objs {
		ros[i] = objs[i]
	}
	svc, _ := newFakeService(t, ros...)
	svc.now = func() time.Time { return now }
	queue := NewAssignQueue()
	pm := NewPoolManager(svc, queue, cfg)
	pm.now = func() time.Time { return now }
	return pm, svc, queue
}

func TestPoolRefillsToTarget(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	pm, svc, _ := newTestPool(t, PoolConfig{TargetWarm: 3, MaxTotal: 60}, now)

	if err := pm.reconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	available, _, total := poolStateCounts(t, svc)
	if available != 3 || total != 3 {
		t.Fatalf("available=%d total=%d, want 3/3", available, total)
	}

	// Level-based: a second pass must not over-provision.
	if err := pm.reconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile2: %v", err)
	}
	if available, _, total = poolStateCounts(t, svc); available != 3 || total != 3 {
		t.Fatalf("after resync: available=%d total=%d, want 3/3", available, total)
	}
}

func TestPoolReplacesClaimedMembers(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	pm, svc, _ := newTestPool(t, PoolConfig{TargetWarm: 2, MaxTotal: 60}, now,
		liveSession("s-pool-claimed1", "alice", now.Add(-time.Hour)),
		poolMember("s-pool-warm1", now.Add(-time.Hour), true),
	)

	if err := pm.reconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	available, claimed, total := poolStateCounts(t, svc)
	if available != 2 || claimed != 1 || total != 3 {
		t.Fatalf("available=%d claimed=%d total=%d, want 2/1/3", available, claimed, total)
	}
}

func TestPoolRespectsConcurrentCeiling(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	// Ceiling 4, already 3 live sessions -> only ONE warm slot despite target 3.
	pm, svc, _ := newTestPool(t, PoolConfig{TargetWarm: 3, MaxTotal: 4}, now,
		liveSession("s-live1", "u1", now.Add(-time.Hour)),
		liveSession("s-live2", "u2", now.Add(-time.Hour)),
		liveSession("s-live3", "u3", now.Add(-time.Hour)),
	)

	if err := pm.reconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	available, _, total := poolStateCounts(t, svc)
	if available != 1 || total != 4 {
		t.Fatalf("available=%d total=%d, want warm+live capped at 4 (1 warm)", available, total)
	}
}

func TestPoolRecyclesStaleMembers(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	pm, svc, _ := newTestPool(t, PoolConfig{TargetWarm: 1, MaxTotal: 60, MaxWarmAge: 24 * time.Hour}, now,
		poolMember("s-pool-stale", now.Add(-25*time.Hour), true),
	)

	if err := pm.reconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	claims, _ := svc.listManaged(context.Background())
	for i := range claims {
		if claims[i].GetName() == "s-pool-stale" {
			t.Fatalf("stale member should have been recycled")
		}
	}
	available, _, _ := poolStateCounts(t, svc)
	if available != 1 {
		t.Fatalf("available=%d, want 1 fresh replacement", available)
	}
}

func TestPoolTrimsOvershoot(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	pm, svc, _ := newTestPool(t, PoolConfig{TargetWarm: 2, MaxTotal: 60}, now,
		poolMember("s-pool-1", now.Add(-4*time.Hour), true),
		poolMember("s-pool-2", now.Add(-3*time.Hour), true),
		poolMember("s-pool-3", now.Add(-2*time.Hour), true),
		poolMember("s-pool-4", now.Add(-1*time.Hour), false), // youngest, not ready
	)

	if err := pm.reconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	claims, _ := svc.listManaged(context.Background())
	names := map[string]bool{}
	for i := range claims {
		names[claims[i].GetName()] = true
	}
	if len(claims) != 2 {
		t.Fatalf("total=%d, want 2 after trim (got %v)", len(claims), names)
	}
	// Oldest members survive (FIFO keeps churn low); youngest trimmed first.
	if !names["s-pool-1"] || !names["s-pool-2"] {
		t.Fatalf("oldest members should survive, got %v", names)
	}
}

func TestPoolAdmitsQueuedRequests(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	pm, svc, queue := newTestPool(t, PoolConfig{TargetWarm: 2, MaxTotal: 60}, now,
		poolMember("s-pool-ready", now.Add(-10*time.Minute), true),
	)

	queue.Enqueue("queued-user", models.CreateSessionRequest{TTLMinutes: 60})
	ch, unsub, ok := queue.Subscribe("queued-user")
	if !ok {
		t.Fatalf("subscribe failed")
	}
	defer unsub()

	if err := pm.reconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if queue.Len() != 0 {
		t.Fatalf("queue should be drained, len=%d", queue.Len())
	}
	sessions, err := svc.List(context.Background(), "queued-user")
	if err != nil || len(sessions) != 1 {
		t.Fatalf("queued user sessions = %d (%v), want 1", len(sessions), err)
	}
	if sessions[0].Name != "s-pool-ready" {
		t.Fatalf("assigned %q, want the ready member", sessions[0].Name)
	}

	// Subscriber got a terminal "assigned" event.
	var last QueueEvent
	for ev := range ch {
		last = ev
		if ev.Type == "assigned" || ev.Type == "error" {
			break
		}
	}
	if last.Type != "assigned" || last.Session == nil || last.Session.Name != "s-pool-ready" {
		t.Fatalf("terminal event = %+v, want assigned s-pool-ready", last)
	}
}

func TestPoolQueueFIFOWhenScarce(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	pm, svc, queue := newTestPool(t, PoolConfig{TargetWarm: 1, MaxTotal: 60}, now,
		poolMember("s-pool-only", now.Add(-10*time.Minute), true),
	)

	queue.Enqueue("first", models.CreateSessionRequest{})
	queue.Enqueue("second", models.CreateSessionRequest{})

	if err := pm.reconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got, _ := svc.List(context.Background(), "first"); len(got) != 1 {
		t.Fatalf("head of queue should be admitted first")
	}
	if got, _ := svc.List(context.Background(), "second"); len(got) != 0 {
		t.Fatalf("second should still be waiting")
	}
	if pos, ok := queue.Position("second"); !ok || pos != 1 {
		t.Fatalf("second position = %d/%v, want 1/true", pos, ok)
	}
}

func TestPoolMarkerGC(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	pm, svc, queue := newTestPool(t, PoolConfig{TargetWarm: 1, MaxTotal: 60}, now,
		liveSession("s-pool-held", "owner-with-claim", now.Add(-time.Hour)),
		ownerMarkerFixture("owner-with-claim", now.Add(-time.Hour)),   // valid: claim exists
		ownerMarkerFixture("crashed-owner", now.Add(-10*time.Minute)), // orphan: no claim, old
		ownerMarkerFixture("fresh-owner", now.Add(-10*time.Second)),   // in-flight: too young to GC
		ownerMarkerFixture("queued-owner", now.Add(-10*time.Minute)),  // protected: still queued
	)
	queue.Enqueue("queued-owner", models.CreateSessionRequest{})

	if err := pm.reconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	markers, err := svc.listOwnerMarkers(context.Background())
	if err != nil {
		t.Fatalf("listOwnerMarkers: %v", err)
	}
	got := map[string]bool{}
	for i := range markers {
		owner, _, _ := unstructured.NestedString(markers[i].Object, "data", markerKeyOwner)
		got[owner] = true
	}
	if got["crashed-owner"] {
		t.Fatalf("orphaned marker should be GC'd: %v", got)
	}
	if !got["owner-with-claim"] || !got["fresh-owner"] {
		t.Fatalf("valid/fresh markers must survive: %v", got)
	}
	// queued-owner's marker survives OR the owner was admitted (queue had a
	// warm member available this pass) — either way they must not be wedged.
	if !got["queued-owner"] {
		if sessions, _ := svc.List(context.Background(), "queued-owner"); len(sessions) == 0 {
			if _, stillQueued := queue.Position("queued-owner"); stillQueued {
				t.Fatalf("queued owner lost marker while still waiting")
			}
		}
	}
}

func TestPoolEmptyAssignFallsBackToQueueFlow(t *testing.T) {
	// End-to-end shape of the empty-pool path at the service level: Assign ->
	// ErrPoolEmpty (handler queues) -> refill -> reconcile admits.
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	pm, svc, queue := newTestPool(t, PoolConfig{TargetWarm: 1, MaxTotal: 60}, now)

	_, err := svc.Assign(context.Background(), "walk-in", models.CreateSessionRequest{})
	if err != ErrPoolEmpty {
		t.Fatalf("err = %v, want ErrPoolEmpty", err)
	}
	queue.Enqueue("walk-in", models.CreateSessionRequest{})

	// First reconcile: nothing Ready yet -> refill provisions a member.
	if err := pm.reconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if queue.Len() != 1 {
		t.Fatalf("not admitted before member is Ready")
	}

	// Simulate the member becoming Ready (Crossplane would set this). Also
	// stamp creationTimestamp: the real API server sets it, the fake doesn't,
	// and a zero timestamp reads as ancient (stale) to the freshness check.
	claims, _ := svc.listManaged(context.Background())
	for i := range claims {
		if poolState(&claims[i]) == poolAvailable {
			m := claims[i].DeepCopy()
			_ = unstructured.SetNestedField(m.Object, now.Format(time.RFC3339), "metadata", "creationTimestamp")
			_ = unstructured.SetNestedField(m.Object, true, "status", "workspaceReady")
			if _, err := svc.resource().Update(context.Background(), m, metav1.UpdateOptions{}); err != nil {
				t.Fatalf("mark ready: %v", err)
			}
		}
	}

	if err := pm.reconcileOnce(context.Background()); err != nil {
		t.Fatalf("reconcile2: %v", err)
	}
	if queue.Len() != 0 {
		t.Fatalf("waiter should be admitted once a member is Ready")
	}
	if sessions, _ := svc.List(context.Background(), "walk-in"); len(sessions) != 1 {
		t.Fatalf("walk-in should own 1 session")
	}
}
