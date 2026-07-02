package kubernetes

import (
	"context"
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

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// poolMember builds a warm pool-member claim.
func poolMember(name string, created time.Time, ready bool) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": models.APIGroup + "/" + models.APIVersion,
		"kind":       models.Kind,
		"metadata": map[string]interface{}{
			"name":              name,
			"namespace":         "playground",
			"creationTimestamp": created.UTC().Format(time.RFC3339),
			"resourceVersion":   "1",
			"labels": map[string]interface{}{
				managedByLabel: managedByValue,
				poolLabel:      poolAvailable,
			},
		},
		"spec": map[string]interface{}{
			"ownerRef":  "",
			"tenantRef": "",
		},
		"status": map[string]interface{}{
			"workspaceReady": ready,
		},
	}
	return &unstructured.Unstructured{Object: obj}
}

// newFakeService builds a SessionService over a fake dynamic client that (a)
// knows the claim and ConfigMap list kinds and (b) ENFORCES optimistic
// concurrency on claim updates — the default fake tracker ignores
// resourceVersion, which would silently pass racy code.
func newFakeService(t *testing.T, objs ...runtime.Object) (*SessionService, *dynamicfake.FakeDynamicClient) {
	t.Helper()
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		models.GVR:   models.Kind + "List",
		configMapGVR: "ConfigMapList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)

	var mu sync.Mutex // makes check-and-swap atomic within the fake
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

	svc := NewSessionService(client, "playground", "https://kubesandbox.com", models.DefaultWorkspaceImage)
	return svc, client
}

func TestAssignClaimsMemberAndStartsTTL(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	svc, client := newFakeService(t, poolMember("s-pool-aaa", now.Add(-10*time.Minute), true))
	svc.now = func() time.Time { return now }

	sess, err := svc.Assign(context.Background(), "alice-sub", models.CreateSessionRequest{TTLMinutes: 90})
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if sess.OwnerRef != "alice-sub" || sess.TenantRef != "alice-sub" {
		t.Fatalf("owner/tenant not set: %+v", sess)
	}
	// TTL starts at ASSIGNMENT: expiresAt = now + 90m, not creation + ttl.
	wantExp := now.Add(90 * time.Minute).Format(time.RFC3339)
	if sess.ExpiresAt != wantExp {
		t.Fatalf("expiresAt = %q, want %q", sess.ExpiresAt, wantExp)
	}

	obj, err := client.Tracker().Get(models.GVR, "playground", "s-pool-aaa")
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	u := obj.(*unstructured.Unstructured)
	if u.GetLabels()[poolLabel] != poolClaimed {
		t.Fatalf("pool label = %q, want claimed", u.GetLabels()[poolLabel])
	}
	if u.GetLabels()[ownerLabel] != ownerHash("alice-sub") {
		t.Fatalf("owner label not set")
	}

	// The per-owner marker exists and records the member.
	cm, err := client.Tracker().Get(configMapGVR, "playground", markerName("alice-sub"))
	if err != nil {
		t.Fatalf("marker missing: %v", err)
	}
	member, _, _ := unstructured.NestedString(cm.(*unstructured.Unstructured).Object, "data", markerKeyMember)
	if member != "s-pool-aaa" {
		t.Fatalf("marker member = %q", member)
	}

	// Owner-scoped reads now see the session; authz passes for the owner and
	// fails for a non-owner (assignment -> authz visibility).
	if err := svc.Authorize(context.Background(), sess.ID, "alice-sub"); err != nil {
		t.Fatalf("owner authz: %v", err)
	}
	if err := svc.Authorize(context.Background(), sess.ID, "bob-sub"); err != ErrNotFound {
		t.Fatalf("non-owner authz = %v, want ErrNotFound", err)
	}
}

func TestAssignPoolEmpty(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	// Only a not-ready member — not assignable.
	svc, client := newFakeService(t, poolMember("s-pool-notready", now.Add(-time.Minute), false))
	svc.now = func() time.Time { return now }

	_, err := svc.Assign(context.Background(), "alice-sub", models.CreateSessionRequest{})
	if err != ErrPoolEmpty {
		t.Fatalf("err = %v, want ErrPoolEmpty", err)
	}
	// The reservation marker must have been rolled back so the user can queue.
	if _, err := client.Tracker().Get(configMapGVR, "playground", markerName("alice-sub")); !apierrors.IsNotFound(err) {
		t.Fatalf("marker should be rolled back, got %v", err)
	}
}

func TestAssignSkipsStaleMembers(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	svc, _ := newFakeService(t, poolMember("s-pool-old", now.Add(-48*time.Hour), true))
	svc.now = func() time.Time { return now }
	svc.SetMaxWarmAge(24 * time.Hour)

	if _, err := svc.Assign(context.Background(), "alice-sub", models.CreateSessionRequest{}); err != ErrPoolEmpty {
		t.Fatalf("stale member must not be handed out; err = %v, want ErrPoolEmpty", err)
	}
}

func TestAssignOnePerUserUnderConcurrency(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	svc, _ := newFakeService(t,
		poolMember("s-pool-a", now.Add(-10*time.Minute), true),
		poolMember("s-pool-b", now.Add(-9*time.Minute), true),
		poolMember("s-pool-c", now.Add(-8*time.Minute), true),
	)
	svc.now = func() time.Time { return now }

	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = svc.Assign(context.Background(), "same-user", models.CreateSessionRequest{})
		}(i)
	}
	wg.Wait()

	ok, dup := 0, 0
	for _, err := range results {
		switch err {
		case nil:
			ok++
		case ErrAlreadyExists:
			dup++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || dup != n-1 {
		t.Fatalf("one-per-user violated: %d successes, %d rejections", ok, dup)
	}

	// Exactly one member owned by the user.
	sessions, err := svc.List(context.Background(), "same-user")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("user holds %d sessions, want 1", len(sessions))
	}
}

func TestAssignDistinctUsersGetDistinctMembers(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	svc, _ := newFakeService(t,
		poolMember("s-pool-a", now.Add(-10*time.Minute), true),
		poolMember("s-pool-b", now.Add(-9*time.Minute), true),
		poolMember("s-pool-c", now.Add(-8*time.Minute), true),
		poolMember("s-pool-d", now.Add(-7*time.Minute), true),
	)
	svc.now = func() time.Time { return now }

	const n = 4
	var wg sync.WaitGroup
	names := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sess, err := svc.Assign(context.Background(), fmt.Sprintf("user-%d", i), models.CreateSessionRequest{})
			errs[i] = err
			if err == nil {
				names[i] = sess.Name
			}
		}(i)
	}
	wg.Wait()

	seen := map[string]int{}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("user-%d: %v", i, errs[i])
		}
		seen[names[i]]++
	}
	for name, count := range seen {
		if count != 1 {
			t.Fatalf("member %s assigned to %d users — double assignment", name, count)
		}
	}
}

func TestAssignRejectsLegacySessionHolder(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	// A pre-pool session with the owner-derived name and no marker.
	legacy := poolMember("s-"+ownerHash("alice-sub")[:16], now.Add(-time.Hour), true)
	legacy.SetLabels(map[string]string{managedByLabel: managedByValue, ownerLabel: ownerHash("alice-sub")})
	_ = unstructured.SetNestedField(legacy.Object, "alice-sub", "spec", "ownerRef")

	svc, client := newFakeService(t, legacy, poolMember("s-pool-a", now.Add(-10*time.Minute), true))
	svc.now = func() time.Time { return now }

	if _, err := svc.Assign(context.Background(), "alice-sub", models.CreateSessionRequest{}); err != ErrAlreadyExists {
		t.Fatalf("err = %v, want ErrAlreadyExists (legacy session holder)", err)
	}
	if _, err := client.Tracker().Get(configMapGVR, "playground", markerName("alice-sub")); !apierrors.IsNotFound(err) {
		t.Fatalf("marker should be rolled back, got %v", err)
	}
}

func TestDeleteReleasesMarker(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	svc, client := newFakeService(t, poolMember("s-pool-a", now.Add(-10*time.Minute), true))
	svc.now = func() time.Time { return now }

	sess, err := svc.Assign(context.Background(), "alice-sub", models.CreateSessionRequest{})
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if err := svc.Delete(context.Background(), sess.ID, "alice-sub"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := client.Tracker().Get(configMapGVR, "playground", markerName("alice-sub")); !apierrors.IsNotFound(err) {
		t.Fatalf("marker should be gone after delete, got %v", err)
	}
	// User can immediately create again (fresh member available? none left ->
	// pool empty, NOT AlreadyExists).
	if _, err := svc.Assign(context.Background(), "alice-sub", models.CreateSessionRequest{}); err != ErrPoolEmpty {
		t.Fatalf("err = %v, want ErrPoolEmpty (slot released)", err)
	}
}

func TestUnclaimedMembersInvisibleToOwnerPaths(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	svc, _ := newFakeService(t, poolMember("s-pool-a", now.Add(-time.Minute), true))

	// An empty ownerRef (whatever produces it) must never see pool members.
	if sessions, _ := svc.List(context.Background(), ""); len(sessions) != 0 {
		t.Fatalf("empty owner lists %d sessions", len(sessions))
	}
	if err := svc.Authorize(context.Background(), "playground-s-pool-a", ""); err != ErrNotFound {
		t.Fatalf("authz with empty owner = %v, want ErrNotFound", err)
	}
	// A real user doesn't own an unclaimed member either.
	if err := svc.Authorize(context.Background(), "playground-s-pool-a", "someone"); err != ErrNotFound {
		t.Fatalf("authz on unclaimed member = %v, want ErrNotFound", err)
	}
}
