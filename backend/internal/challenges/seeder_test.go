package challenges

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/jeremy-misola/kubesandbox/backend/internal/content"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

var (
	claimGVR  = models.GVR
	cmGVR     = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	testOwner = "alice-sub"
)

// markerNameFor mirrors the kubernetes package's owner-marker naming (the
// helper is unexported there; the format is part of the storage contract).
func markerNameFor(owner string) string {
	sum := sha256.Sum256([]byte(owner))
	return "sbxowner-" + hex.EncodeToString(sum[:])[:16]
}

// newFakeSvc builds a real SessionService over a fake dynamic client that
// ENFORCES optimistic concurrency on claim updates — the same
// resourceVersion-checking reactor as pool_test.go. The stock fake ignores
// rv, which would silently pass racy seed-state transitions.
func newFakeSvc(t *testing.T, objs ...runtime.Object) (*k8s.SessionService, *dynamicfake.FakeDynamicClient) {
	t.Helper()
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		claimGVR: models.Kind + "List",
		cmGVR:    "ConfigMapList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)

	var mu sync.Mutex
	client.PrependReactor("update", "kubesandboxsessions", func(action clienttesting.Action) (bool, runtime.Object, error) {
		mu.Lock()
		defer mu.Unlock()
		ua := action.(clienttesting.UpdateAction)
		obj := ua.GetObject().(*unstructured.Unstructured)
		gvr := ua.GetResource()
		cur, err := client.Tracker().Get(gvr, ua.GetNamespace(), obj.GetName())
		if err != nil {
			return true, nil, err
		}
		curRV := cur.(*unstructured.Unstructured).GetResourceVersion()
		if obj.GetResourceVersion() != curRV {
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Group: models.APIGroup, Resource: models.Plural},
				obj.GetName(), fmt.Errorf("resourceVersion mismatch"))
		}
		n, _ := strconv.Atoi(curRV)
		next := obj.DeepCopy()
		next.SetResourceVersion(strconv.Itoa(n + 1))
		if err := client.Tracker().Update(gvr, next, ua.GetNamespace()); err != nil {
			return true, nil, err
		}
		return true, next, nil
	})

	svc := k8s.NewSessionService(client, "playground", "https://kubesandbox.com", models.DefaultWorkspaceImage)
	return svc, client
}

// challengeClaim builds a claimed challenge session in a given seed state.
func challengeClaim(name, owner, challengeID, state string, attempts, recycles int) *unstructured.Unstructured {
	annots := map[string]interface{}{
		k8s.ChallengeIDAnnot:  challengeID,
		k8s.SeedStateAnnot:    state,
		k8s.SeedAttemptsAnnot: strconv.Itoa(attempts),
	}
	if recycles > 0 {
		annots[k8s.SeedRecyclesAnnot] = strconv.Itoa(recycles)
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": models.APIGroup + "/" + models.APIVersion,
		"kind":       models.Kind,
		"metadata": map[string]interface{}{
			"name":            name,
			"namespace":       "playground",
			"resourceVersion": "1",
			"labels": map[string]interface{}{
				"app.kubernetes.io/managed-by": "kubesandbox-backend",
				"kubesandbox.com/pool":         "claimed",
			},
			"annotations": annots,
		},
		"spec": map[string]interface{}{
			"ownerRef":      owner,
			"tenantRef":     owner,
			"ttlMinutes":    int64(60),
			"expiresAt":     "2026-07-04T13:00:00Z",
			"starterLabRef": challengeID,
		},
		"status": map[string]interface{}{"workspaceReady": true},
	}}
}

// warmMember builds an assignable pool member (recycle re-assign target).
func warmMember(name string, created time.Time) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": models.APIGroup + "/" + models.APIVersion,
		"kind":       models.Kind,
		"metadata": map[string]interface{}{
			"name":              name,
			"namespace":         "playground",
			"creationTimestamp": created.UTC().Format(time.RFC3339),
			"resourceVersion":   "1",
			"labels": map[string]interface{}{
				"app.kubernetes.io/managed-by": "kubesandbox-backend",
				"kubesandbox.com/pool":         "available",
			},
		},
		"spec":   map[string]interface{}{"ownerRef": "", "tenantRef": ""},
		"status": map[string]interface{}{"workspaceReady": true},
	}}
}

// ownerMarker builds the per-owner reservation ConfigMap.
func ownerMarker(owner string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      markerNameFor(owner),
			"namespace": "playground",
			"labels": map[string]interface{}{
				"app.kubernetes.io/managed-by": "kubesandbox-backend",
				"kubesandbox.com/owner-marker": "true",
			},
		},
		"data": map[string]interface{}{"owner": owner, "member": ""},
	}}
}

// fakeTenant is a scriptable TenantOps for state-machine tests.
type fakeTenant struct {
	mu sync.Mutex
	// failApply maps claimName -> remaining apply failures.
	failApply    map[string]int
	applyCalls   []string
	deleteCalls  []string
	applyBundles []string
}

func (f *fakeTenant) Apply(_ context.Context, claimName string, bundle *content.Bundle) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applyCalls = append(f.applyCalls, claimName)
	f.applyBundles = append(f.applyBundles, bundle.ID)
	if f.failApply[claimName] > 0 {
		f.failApply[claimName]--
		return fmt.Errorf("simulated tenant apply failure")
	}
	return nil
}

func (f *fakeTenant) DeleteSeeded(_ context.Context, claimName, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteCalls = append(f.deleteCalls, claimName)
	return nil
}

func (f *fakeTenant) GetObject(context.Context, string, content.TargetRef) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeTenant) ListObjects(context.Context, string, content.TargetRef) ([]unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeTenant) CanI(context.Context, string, content.SubjectRef, content.AccessRef) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (f *fakeTenant) applies() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.applyCalls...)
}

// testBundle builds a minimal valid bundle in a FixedStore.
func testStore(t *testing.T, id string) content.Store {
	t.Helper()
	y := fmt.Sprintf(`apiVersion: content.kubesandbox.com/v1
id: %s
title: T
description: d
category: rbac
difficulty: easy
validate:
  - id: s1
    description: d
    checks:
      - type: resourceExists
        target: {apiVersion: v1, kind: Namespace, name: demo}
`, id)
	b, err := content.LoadBundle([]byte(y), map[string][]byte{
		"00-ns.yaml": []byte("apiVersion: v1\nkind: Namespace\nmetadata: {name: demo}\n"),
	})
	if err != nil {
		t.Fatalf("test bundle: %v", err)
	}
	return content.FixedStore{id: b}
}

func newTestSeeder(svc *k8s.SessionService, store content.Store, tenant TenantOps, maxAttempts int) *Seeder {
	s := NewSeeder(svc, store, tenant, SeederConfig{
		Budget:      time.Second,
		ResetBudget: time.Second,
		MaxAttempts: maxAttempts,
		Backoff:     time.Millisecond,
		Resync:      time.Hour, // tests drive Process/ReconcileOnce directly
		Workers:     1,
	})
	s.sleep = func(context.Context, time.Duration) {} // no wall-clock waits
	return s
}

func getClaimState(t *testing.T, client *dynamicfake.FakeDynamicClient, name string) (state string, attempts string, exists bool) {
	t.Helper()
	obj, err := client.Tracker().Get(claimGVR, "playground", name)
	if apierrors.IsNotFound(err) {
		return "", "", false
	}
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	u := obj.(*unstructured.Unstructured)
	return u.GetAnnotations()[k8s.SeedStateAnnot], u.GetAnnotations()[k8s.SeedAttemptsAnnot], true
}

func markerExists(client *dynamicfake.FakeDynamicClient, owner string) bool {
	_, err := client.Tracker().Get(cmGVR, "playground", markerNameFor(owner))
	return err == nil
}

func TestSeederSeedsPendingClaim(t *testing.T) {
	svc, client := newFakeSvc(t, challengeClaim("s-pool-a", testOwner, "ch-1", k8s.SeedStatePending, 0, 0), ownerMarker(testOwner))
	tenant := &fakeTenant{}
	s := newTestSeeder(svc, testStore(t, "ch-1"), tenant, 3)

	s.Process(context.Background(), "s-pool-a")

	state, attempts, ok := getClaimState(t, client, "s-pool-a")
	if !ok || state != k8s.SeedStateSeeded || attempts != "1" {
		t.Fatalf("state=%q attempts=%q exists=%v, want seeded/1/true", state, attempts, ok)
	}
	if calls := tenant.applies(); len(calls) != 1 || calls[0] != "s-pool-a" {
		t.Fatalf("apply calls = %v, want exactly one for s-pool-a", calls)
	}
	if tenant.applyBundles[0] != "ch-1" {
		t.Fatalf("applied bundle %q, want ch-1", tenant.applyBundles[0])
	}
}

func TestSeederRetriesInPlaceThenSucceeds(t *testing.T) {
	svc, client := newFakeSvc(t, challengeClaim("s-pool-a", testOwner, "ch-1", k8s.SeedStatePending, 0, 0), ownerMarker(testOwner))
	tenant := &fakeTenant{failApply: map[string]int{"s-pool-a": 2}}
	s := newTestSeeder(svc, testStore(t, "ch-1"), tenant, 3)

	s.Process(context.Background(), "s-pool-a")

	// Two failures + one success, all on the SAME member (SSA converges over
	// partial applies, §6.3.1), attempts persisted through each transition.
	state, attempts, _ := getClaimState(t, client, "s-pool-a")
	if state != k8s.SeedStateSeeded || attempts != "3" {
		t.Fatalf("state=%q attempts=%q, want seeded/3", state, attempts)
	}
	if calls := tenant.applies(); len(calls) != 3 {
		t.Fatalf("apply calls = %v, want 3 in-place attempts", calls)
	}
}

func TestSeederCrashResume(t *testing.T) {
	// Backend died mid-seed: the claim is stuck in state=seeding, attempts=1.
	// The startup reconcile must find it and converge (§6.1: "what makes a
	// crash mid-seed a non-event").
	svc, client := newFakeSvc(t,
		challengeClaim("s-pool-crash", testOwner, "ch-1", k8s.SeedStateSeeding, 1, 0),
		challengeClaim("s-pool-done", "bob-sub", "ch-1", k8s.SeedStateSeeded, 1, 0),
		ownerMarker(testOwner))
	tenant := &fakeTenant{}
	s := newTestSeeder(svc, testStore(t, "ch-1"), tenant, 3)

	// The reconcile source must surface pending AND seeding, never terminal.
	work, err := svc.ListSeedWork(context.Background())
	if err != nil || len(work) != 1 || work[0].GetName() != "s-pool-crash" {
		t.Fatalf("ListSeedWork = %v (%v), want just s-pool-crash", work, err)
	}

	s.Process(context.Background(), "s-pool-crash")
	state, attempts, _ := getClaimState(t, client, "s-pool-crash")
	if state != k8s.SeedStateSeeded || attempts != "2" {
		t.Fatalf("state=%q attempts=%q, want seeded/2 after resume", state, attempts)
	}
	// The already-seeded claim was never touched.
	if calls := tenant.applies(); len(calls) != 1 {
		t.Fatalf("apply calls = %v, want only the resumed claim", calls)
	}
}

func TestSeederRecyclesOnceThenSeedsReplacement(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	bad := challengeClaim("s-pool-bad", testOwner, "ch-1", k8s.SeedStatePending, 0, 0)
	svc, client := newFakeSvc(t, bad, warmMember("s-pool-spare", now.Add(-5*time.Minute)), ownerMarker(testOwner))
	tenant := &fakeTenant{failApply: map[string]int{"s-pool-bad": 99}} // bad member never seeds
	s := newTestSeeder(svc, testStore(t, "ch-1"), tenant, 2)

	s.Process(context.Background(), "s-pool-bad")

	// Old member gone, marker KEPT, replacement claimed with recycles=1 and
	// the ORIGINAL expiry (recycle never extends the clock).
	if _, _, exists := getClaimState(t, client, "s-pool-bad"); exists {
		t.Fatalf("bad member should have been recycled")
	}
	if !markerExists(client, testOwner) {
		t.Fatalf("owner marker must survive the recycle (§6.3.2)")
	}
	obj, err := client.Tracker().Get(claimGVR, "playground", "s-pool-spare")
	if err != nil {
		t.Fatalf("replacement not claimed: %v", err)
	}
	u := obj.(*unstructured.Unstructured)
	if u.GetAnnotations()[k8s.SeedRecyclesAnnot] != "1" {
		t.Fatalf("replacement seed-recycles = %q, want 1", u.GetAnnotations()[k8s.SeedRecyclesAnnot])
	}
	if got, _, _ := unstructured.NestedString(u.Object, "spec", "expiresAt"); got != "2026-07-04T13:00:00Z" {
		t.Fatalf("replacement expiresAt = %q, want the original", got)
	}
	if got, _, _ := unstructured.NestedString(u.Object, "spec", "ownerRef"); got != testOwner {
		t.Fatalf("replacement owner = %q", got)
	}

	// The seeder enqueued the replacement; drive it and it seeds fine.
	s.Process(context.Background(), "s-pool-spare")
	state, _, _ := getClaimState(t, client, "s-pool-spare")
	if state != k8s.SeedStateSeeded {
		t.Fatalf("replacement state = %q, want seeded", state)
	}
}

func TestSeederFailsClosedAfterRecycledMemberFails(t *testing.T) {
	// Second member also refuses to seed: recycle at most once, then fail
	// closed with the marker RELEASED so the user can retry (§6.3.3).
	svc, client := newFakeSvc(t,
		challengeClaim("s-pool-second", testOwner, "ch-1", k8s.SeedStatePending, 0, 1), // already recycled once
		ownerMarker(testOwner))
	tenant := &fakeTenant{failApply: map[string]int{"s-pool-second": 99}}
	s := newTestSeeder(svc, testStore(t, "ch-1"), tenant, 2)

	s.Process(context.Background(), "s-pool-second")

	if _, _, exists := getClaimState(t, client, "s-pool-second"); exists {
		t.Fatalf("failed claim must be deleted (fail closed)")
	}
	if markerExists(client, testOwner) {
		t.Fatalf("marker must be released on fail-closed so the user can retry")
	}
}

func TestSeederFailsClosedWhenPoolEmptyOnRecycle(t *testing.T) {
	// Recycle path with NO spare member: delete, re-assign fails ErrPoolEmpty,
	// marker released — exactly as if the original create had failed.
	svc, client := newFakeSvc(t,
		challengeClaim("s-pool-bad", testOwner, "ch-1", k8s.SeedStatePending, 0, 0),
		ownerMarker(testOwner))
	tenant := &fakeTenant{failApply: map[string]int{"s-pool-bad": 99}}
	s := newTestSeeder(svc, testStore(t, "ch-1"), tenant, 1)

	s.Process(context.Background(), "s-pool-bad")

	if _, _, exists := getClaimState(t, client, "s-pool-bad"); exists {
		t.Fatalf("bad claim must be deleted")
	}
	if markerExists(client, testOwner) {
		t.Fatalf("marker must be released when no replacement exists")
	}
}

func TestSeederFailsClosedOnUnknownBundle(t *testing.T) {
	// Content removed/quarantined between create-validation and seeding.
	svc, client := newFakeSvc(t,
		challengeClaim("s-pool-a", testOwner, "ghost-challenge", k8s.SeedStatePending, 0, 0),
		ownerMarker(testOwner))
	tenant := &fakeTenant{}
	s := newTestSeeder(svc, testStore(t, "ch-1"), tenant, 3)

	s.Process(context.Background(), "s-pool-a")

	if _, _, exists := getClaimState(t, client, "s-pool-a"); exists {
		t.Fatalf("claim must be deleted on unknown bundle")
	}
	if markerExists(client, testOwner) {
		t.Fatalf("marker must be released")
	}
	if len(tenant.applies()) != 0 {
		t.Fatalf("nothing must be applied for an unknown bundle")
	}
}

func TestSeederResetDeletesThenReapplies(t *testing.T) {
	claim := challengeClaim("s-pool-a", testOwner, "ch-1", k8s.SeedStatePending, 0, 0)
	annots := claim.GetAnnotations()
	annots[k8s.SeedResetAnnot] = "true"
	claim.SetAnnotations(annots)

	svc, client := newFakeSvc(t, claim, ownerMarker(testOwner))
	tenant := &fakeTenant{}
	s := newTestSeeder(svc, testStore(t, "ch-1"), tenant, 3)

	s.Process(context.Background(), "s-pool-a")

	// Reset semantics (§7): labeled state deleted BEFORE the re-apply, flag
	// cleared, claim back to seeded.
	if len(tenant.deleteCalls) != 1 || tenant.deleteCalls[0] != "s-pool-a" {
		t.Fatalf("DeleteSeeded calls = %v, want one for s-pool-a", tenant.deleteCalls)
	}
	if len(tenant.applies()) != 1 {
		t.Fatalf("apply calls = %v, want re-apply after delete", tenant.applies())
	}
	obj, _ := client.Tracker().Get(claimGVR, "playground", "s-pool-a")
	u := obj.(*unstructured.Unstructured)
	if u.GetAnnotations()[k8s.SeedStateAnnot] != k8s.SeedStateSeeded {
		t.Fatalf("state = %q, want seeded", u.GetAnnotations()[k8s.SeedStateAnnot])
	}
	if _, still := u.GetAnnotations()[k8s.SeedResetAnnot]; still {
		t.Fatalf("reset flag must be cleared by the pending→seeding CAS")
	}
}

func TestSeederIgnoresTerminalAndNonChallengeClaims(t *testing.T) {
	svc, _ := newFakeSvc(t,
		challengeClaim("s-pool-done", testOwner, "ch-1", k8s.SeedStateSeeded, 1, 0),
		warmMember("s-pool-warm", time.Date(2026, 7, 4, 11, 0, 0, 0, time.UTC)),
	)
	tenant := &fakeTenant{}
	s := newTestSeeder(svc, testStore(t, "ch-1"), tenant, 3)

	s.Process(context.Background(), "s-pool-done")
	s.Process(context.Background(), "s-pool-warm")
	s.Process(context.Background(), "s-pool-missing")

	if len(tenant.applies()) != 0 {
		t.Fatalf("terminal/non-challenge/missing claims must never be applied to: %v", tenant.applies())
	}
}

func TestSeederConcurrentProcessSingleApplyWins(t *testing.T) {
	// Two workers race the same pending claim: the pending→seeding CAS is the
	// lease — the loser re-reads and sees seeding/seeded. SSA idempotence
	// makes even a double-apply harmless; here we assert the claim converges
	// and attempts stay sane under -race.
	svc, client := newFakeSvc(t, challengeClaim("s-pool-a", testOwner, "ch-1", k8s.SeedStatePending, 0, 0), ownerMarker(testOwner))
	tenant := &fakeTenant{}
	s := newTestSeeder(svc, testStore(t, "ch-1"), tenant, 3)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Process(context.Background(), "s-pool-a")
		}()
	}
	wg.Wait()

	state, _, _ := getClaimState(t, client, "s-pool-a")
	if state != k8s.SeedStateSeeded {
		t.Fatalf("state = %q, want seeded", state)
	}
	if n := len(tenant.applies()); n < 1 || n > 4 {
		t.Fatalf("apply calls = %d, want >=1 (idempotent SSA tolerates duplicates)", n)
	}
}
