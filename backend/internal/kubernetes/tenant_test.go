package kubernetes

import (
	"context"
	"encoding/base64"
	"strings"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// kubeconfigSecret fixture: the composed vc-{ns}-{name}-vcluster Secret shape.
func kubeconfigSecret(claimName, kubeconfig string) *unstructured.Unstructured {
	sessionNS := "playground-" + claimName
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      "vc-" + sessionNS + "-vcluster",
			"namespace": sessionNS,
		},
		"data": map[string]interface{}{
			"config": base64.StdEncoding.EncodeToString([]byte(kubeconfig)),
		},
	}}
}

func newTestFactory(t *testing.T, maxEntries int, objs ...runtime.Object) (*TenantClientFactory, *int) {
	t.Helper()
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{secretGVR: "SecretList"}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)

	f := NewTenantClientFactory(client, "playground", maxEntries)
	builds := 0
	var mu sync.Mutex
	f.buildClient = func(kubeconfig []byte) (*TenantClient, error) {
		mu.Lock()
		builds++
		mu.Unlock()
		if !strings.HasPrefix(string(kubeconfig), "kubeconfig-for-") {
			t.Fatalf("unexpected kubeconfig payload %q", kubeconfig)
		}
		return &TenantClient{}, nil
	}
	return f, &builds
}

func TestTenantFactoryCachesPerSession(t *testing.T) {
	f, builds := newTestFactory(t, 8,
		kubeconfigSecret("s-a", "kubeconfig-for-a"),
	)
	ctx := context.Background()

	c1, err := f.ForSession(ctx, "s-a")
	if err != nil {
		t.Fatalf("ForSession: %v", err)
	}
	c2, err := f.ForSession(ctx, "s-a")
	if err != nil {
		t.Fatalf("ForSession(cached): %v", err)
	}
	if c1 != c2 {
		t.Fatalf("expected the cached client on the second call")
	}
	if *builds != 1 {
		t.Fatalf("builds = %d, want 1 — repeated grading must not re-read the Secret (§5.1)", *builds)
	}
}

func TestTenantFactoryInvalidateForcesRebuild(t *testing.T) {
	f, builds := newTestFactory(t, 8, kubeconfigSecret("s-a", "kubeconfig-for-a"))
	ctx := context.Background()

	if _, err := f.ForSession(ctx, "s-a"); err != nil {
		t.Fatalf("ForSession: %v", err)
	}
	f.Invalidate("s-a")
	if _, err := f.ForSession(ctx, "s-a"); err != nil {
		t.Fatalf("ForSession after invalidate: %v", err)
	}
	if *builds != 2 {
		t.Fatalf("builds = %d, want 2 after invalidation", *builds)
	}
}

func TestTenantFactoryLRUEviction(t *testing.T) {
	f, builds := newTestFactory(t, 2,
		kubeconfigSecret("s-a", "kubeconfig-for-a"),
		kubeconfigSecret("s-b", "kubeconfig-for-b"),
		kubeconfigSecret("s-c", "kubeconfig-for-c"),
	)
	ctx := context.Background()

	// Fill: a, b. Touch a (now MRU). Add c → b evicted, a retained.
	for _, n := range []string{"s-a", "s-b", "s-a", "s-c"} {
		if _, err := f.ForSession(ctx, n); err != nil {
			t.Fatalf("ForSession(%s): %v", n, err)
		}
	}
	if *builds != 3 {
		t.Fatalf("builds = %d, want 3 (a, b, c)", *builds)
	}
	if _, err := f.ForSession(ctx, "s-a"); err != nil {
		t.Fatalf("ForSession(a): %v", err)
	}
	if *builds != 3 {
		t.Fatalf("a must still be cached (LRU keeps recently-used), builds = %d", *builds)
	}
	if _, err := f.ForSession(ctx, "s-b"); err != nil {
		t.Fatalf("ForSession(b): %v", err)
	}
	if *builds != 4 {
		t.Fatalf("b must have been evicted, builds = %d, want 4", *builds)
	}
}

func TestTenantFactoryMissingSecret(t *testing.T) {
	f, _ := newTestFactory(t, 8)
	if _, err := f.ForSession(context.Background(), "s-ghost"); err == nil {
		t.Fatalf("expected error for a session with no kubeconfig Secret (composition drift, §9)")
	}
}
