package kubernetes

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// claim builds a managed KubeSandboxSession with the given creation time and,
// optionally, ttlMinutes / status.expiresAt / deletionTimestamp.
func claim(name string, created time.Time, ttlMinutes int, expiresAt string, deleting bool) *unstructured.Unstructured {
	meta := map[string]interface{}{
		"name":              name,
		"namespace":         "playground",
		"creationTimestamp": created.UTC().Format(time.RFC3339),
		"labels": map[string]interface{}{
			managedByLabel: managedByValue,
		},
	}
	if deleting {
		meta["deletionTimestamp"] = created.UTC().Format(time.RFC3339)
		// A finalizer is required for the fake store to retain an object that
		// carries a deletionTimestamp.
		meta["finalizers"] = []interface{}{"kubesandbox.com/test"}
	}
	obj := map[string]interface{}{
		"apiVersion": models.APIGroup + "/" + models.APIVersion,
		"kind":       models.Kind,
		"metadata":   meta,
		"spec": map[string]interface{}{
			"ownerRef":   "alice@example.com",
			"ttlMinutes": int64(ttlMinutes),
		},
	}
	if expiresAt != "" {
		obj["status"] = map[string]interface{}{"expiresAt": expiresAt}
	}
	return &unstructured.Unstructured{Object: obj}
}

func newTTLService(objs ...runtime.Object) *SessionService {
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{models.GVR: models.Kind + "List"}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)
	return NewSessionService(client, "playground", "https://kubesandbox.com", 3, models.DefaultWorkspaceImage)
}

func TestClaimExpiry(t *testing.T) {
	base := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)

	t.Run("prefers status.expiresAt", func(t *testing.T) {
		c := claim("s-1", base, 60, "2026-06-26T12:30:00Z", false)
		got, ok := claimExpiry(c)
		if !ok || !got.Equal(time.Date(2026, 6, 26, 12, 30, 0, 0, time.UTC)) {
			t.Fatalf("got (%v, %v)", got, ok)
		}
	})

	t.Run("falls back to creation + ttlMinutes", func(t *testing.T) {
		c := claim("s-2", base, 45, "", false)
		got, ok := claimExpiry(c)
		if !ok || !got.Equal(base.Add(45*time.Minute)) {
			t.Fatalf("got (%v, %v)", got, ok)
		}
	})

	t.Run("defaults ttl when missing", func(t *testing.T) {
		c := claim("s-3", base, 0, "", false)
		got, ok := claimExpiry(c)
		if !ok || !got.Equal(base.Add(time.Duration(models.DefaultTTLMinutes)*time.Minute)) {
			t.Fatalf("got (%v, %v)", got, ok)
		}
	})
}

func TestReconcileOnce(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)

	expiredByStatus := claim("s-expired-status", now.Add(-2*time.Hour), 60, "2026-06-26T11:00:00Z", false)
	expiredByTTL := claim("s-expired-ttl", now.Add(-90*time.Minute), 30, "", false) // expired 60m ago
	fresh := claim("s-fresh", now.Add(-5*time.Minute), 60, "", false)               // 55m left
	terminating := claim("s-terminating", now.Add(-3*time.Hour), 30, "", true)      // expired but already deleting

	svc := newTTLService(expiredByStatus, expiredByTTL, fresh, terminating)
	ctrl := NewTTLController(svc, time.Minute)
	ctrl.now = func() time.Time { return now }

	deleted, err := ctrl.reconcileOnce(context.Background())
	if err != nil {
		t.Fatalf("reconcileOnce: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2 (the two expired, not the fresh or terminating)", deleted)
	}

	remaining, err := svc.listManaged(context.Background())
	if err != nil {
		t.Fatalf("listManaged: %v", err)
	}
	got := map[string]bool{}
	for _, r := range remaining {
		got[r.GetName()] = true
	}
	if got["s-expired-status"] || got["s-expired-ttl"] {
		t.Fatalf("expired claims should be gone: %v", got)
	}
	if !got["s-fresh"] {
		t.Fatalf("fresh claim must survive: %v", got)
	}
}
