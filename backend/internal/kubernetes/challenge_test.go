package kubernetes

import (
	"context"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// asUnstructured casts a tracker object.
func asUnstructured(t *testing.T, obj runtime.Object) *unstructured.Unstructured {
	t.Helper()
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("tracker object is %T, want *unstructured.Unstructured", obj)
	}
	return u
}

func TestAssignStampsChallengeAtomically(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	svc, client := newFakeService(t, poolMember("s-pool-aaa", now.Add(-10*time.Minute), true))
	svc.now = func() time.Time { return now }

	var notified []string
	svc.SetSeedNotifier(func(name string) { notified = append(notified, name) })

	sess, err := svc.Assign(context.Background(), "alice-sub", models.CreateSessionRequest{
		TTLMinutes:  60,
		ChallengeID: "troubleshoot-rbac-permissions-for-a-failing-deployment",
	})
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}

	// The claim carries ownership AND seed intent from the same Update: there
	// is no window where a challenge session exists without it (§6.1).
	obj, err := client.Tracker().Get(models.GVR, "playground", "s-pool-aaa")
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	u := asUnstructured(t, obj)
	if got, _, _ := unstructured.NestedString(u.Object, "spec", "starterLabRef"); got != "troubleshoot-rbac-permissions-for-a-failing-deployment" {
		t.Fatalf("spec.starterLabRef = %q (challenge selection must use the existing field, no XRD change)", got)
	}
	annots := u.GetAnnotations()
	if annots[ChallengeIDAnnot] != "troubleshoot-rbac-permissions-for-a-failing-deployment" {
		t.Fatalf("challenge-id annotation = %q", annots[ChallengeIDAnnot])
	}
	if annots[SeedStateAnnot] != SeedStatePending {
		t.Fatalf("seed-state = %q, want pending", annots[SeedStateAnnot])
	}
	if annots[SeedAttemptsAnnot] != "0" {
		t.Fatalf("seed-attempts = %q, want 0", annots[SeedAttemptsAnnot])
	}

	// Fast path poked exactly once with the claimed member.
	if len(notified) != 1 || notified[0] != "s-pool-aaa" {
		t.Fatalf("seed notifier calls = %v, want [s-pool-aaa]", notified)
	}

	// Session payload: challenge block + synthetic Seeding phase (§6.4).
	if sess.Challenge == nil || sess.Challenge.ID == "" || sess.Challenge.SeedState != SeedStatePending {
		t.Fatalf("session challenge block = %+v", sess.Challenge)
	}
	if sess.Phase != "Seeding" {
		t.Fatalf("phase = %q, want synthetic Seeding", sess.Phase)
	}
}

func TestAssignWithoutChallengeUnchanged(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	svc, client := newFakeService(t, poolMember("s-pool-aaa", now.Add(-10*time.Minute), true))
	svc.now = func() time.Time { return now }

	sess, err := svc.Assign(context.Background(), "alice-sub", models.CreateSessionRequest{TTLMinutes: 60})
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if sess.Challenge != nil || sess.Phase == "Seeding" {
		t.Fatalf("plain sessions must be untouched by challenge plumbing: %+v", sess)
	}
	obj, _ := client.Tracker().Get(models.GVR, "playground", "s-pool-aaa")
	u := asUnstructured(t, obj)
	if _, ok := u.GetAnnotations()[ChallengeIDAnnot]; ok {
		t.Fatalf("challenge annotations must not exist on plain sessions")
	}
}

func TestReassignForRecycleKeepsMarkerAndCounts(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	svc, client := newFakeService(t,
		poolMember("s-pool-old", now.Add(-20*time.Minute), true),
		poolMember("s-pool-new", now.Add(-10*time.Minute), true),
	)
	svc.now = func() time.Time { return now }

	// Original challenge assignment.
	sess, err := svc.Assign(context.Background(), "alice-sub", models.CreateSessionRequest{
		TTLMinutes: 90, ChallengeID: "some-challenge",
	})
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if sess.Name != "s-pool-old" {
		t.Fatalf("expected oldest member first, got %s", sess.Name)
	}
	origExpiry := sess.ExpiresAt

	// Recycle: delete the bad member but KEEP the marker, then re-assign.
	if err := svc.DeleteClaimKeepMarker(context.Background(), "s-pool-old"); err != nil {
		t.Fatalf("DeleteClaimKeepMarker: %v", err)
	}
	if _, err := client.Tracker().Get(configMapGVR, "playground", markerName("alice-sub")); err != nil {
		t.Fatalf("marker must survive the recycle: %v", err)
	}

	sess2, err := svc.ReassignForRecycle(context.Background(), "alice-sub",
		models.CreateSessionRequest{TTLMinutes: 90, ChallengeID: "some-challenge"}, origExpiry, 1)
	if err != nil {
		t.Fatalf("ReassignForRecycle: %v", err)
	}
	if sess2.Name != "s-pool-new" {
		t.Fatalf("re-assigned %q, want the next warm member", sess2.Name)
	}
	// The user's clock is preserved: a recycle never extends the session.
	if sess2.ExpiresAt != origExpiry {
		t.Fatalf("expiresAt %q, want original %q", sess2.ExpiresAt, origExpiry)
	}
	obj, _ := client.Tracker().Get(models.GVR, "playground", "s-pool-new")
	u := asUnstructured(t, obj)
	if u.GetAnnotations()[SeedRecyclesAnnot] != "1" {
		t.Fatalf("seed-recycles = %q, want 1 (recycle-at-most-once bookkeeping)", u.GetAnnotations()[SeedRecyclesAnnot])
	}
	if u.GetAnnotations()[SeedStateAnnot] != SeedStatePending {
		t.Fatalf("re-assigned member seed-state = %q, want pending", u.GetAnnotations()[SeedStateAnnot])
	}
}

func TestUpdateClaimEnforcesCAS(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	svc, _ := newFakeService(t, poolMember("s-pool-aaa", now.Add(-10*time.Minute), true))
	svc.now = func() time.Time { return now }

	obj, err := svc.GetClaim(context.Background(), "s-pool-aaa")
	if err != nil {
		t.Fatalf("GetClaim: %v", err)
	}

	// Two writers race with the same observed resourceVersion: exactly one
	// wins, the loser gets a Conflict (the seed-state lease semantics, §6.1).
	a := obj.DeepCopy()
	annotsA := map[string]string{SeedStateAnnot: SeedStateSeeding}
	a.SetAnnotations(annotsA)
	b := obj.DeepCopy()
	annotsB := map[string]string{SeedStateAnnot: SeedStateFailed}
	b.SetAnnotations(annotsB)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, o := range []*unstructured.Unstructured{a, b} {
		wg.Add(1)
		go func(i int, o *unstructured.Unstructured) {
			defer wg.Done()
			_, errs[i] = svc.UpdateClaim(context.Background(), o)
		}(i, o)
	}
	wg.Wait()

	conflicts, wins := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case apierrors.IsConflict(err):
			conflicts++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins=%d conflicts=%d, want 1/1 — every transition must be rv-guarded", wins, conflicts)
	}
}

func TestTTLReapSkipsNothingForChallengeSessions(t *testing.T) {
	// §9: TTL reaping a challenge session needs nothing special — state lives
	// in the vcluster and annotations, both die with the claim. This guards
	// that cleanup still reaps expired challenge sessions like any other.
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	expired := liveSession("s-pool-exp", "alice-sub", now.Add(-3*time.Hour))
	annots := expired.GetAnnotations()
	if annots == nil {
		annots = map[string]string{}
	}
	annots[ChallengeIDAnnot] = "some-challenge"
	annots[SeedStateAnnot] = SeedStateSeeded
	expired.SetAnnotations(annots)
	_ = unstructured.SetNestedField(expired.Object, now.Add(-time.Hour).Format(time.RFC3339), "spec", "expiresAt")

	svc, _ := newFakeService(t, expired, ownerMarkerFixture("alice-sub", now.Add(-3*time.Hour)))
	svc.now = func() time.Time { return now }
	ttl := NewTTLController(svc, time.Minute)
	ttl.now = func() time.Time { return now }

	n, err := ttl.reconcileOnce(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("reaped %d (%v), want 1", n, err)
	}
}
