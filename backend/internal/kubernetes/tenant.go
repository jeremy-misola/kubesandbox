package kubernetes

import (
	"container/list"
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// The tenant-vcluster client (design §5) — the one new capability the
// challenge feature adds. The composition publishes each session vcluster's
// admin kubeconfig as Secret vc-{ns}-{name}-vcluster in the session
// namespace, with exportKubeConfig.server set to the in-cluster Service DNS,
// so the backend can use the kubeconfig AS-IS: fetch Secret → RESTConfig →
// dynamic client. No port-forwarding, no URL rewriting. Verified live against
// prod-k3s (2026-07-04): a pod in the backend namespace listed tenant pods
// through that exact path.
//
// RBAC (§5.2): reading the Secret is authorized by a per-session
// Role/RoleBinding composed into each session namespace — not a cluster-wide
// secrets grant.

// secretGVR addresses the composed kubeconfig Secrets.
var secretGVR = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}

// TenantClient is a built client for one session's virtual API server.
type TenantClient struct {
	// Dynamic talks to the tenant vcluster with the exported admin identity.
	Dynamic dynamic.Interface
	// config is retained for impersonation copies (subjectCan checks).
	config *rest.Config
}

// ImpersonateServiceAccount returns a dynamic client that acts as the given
// tenant ServiceAccount — used by subjectCan/subjectCannot grading checks to
// test the EFFECT of RBAC, not the object shape. Reads only by construction
// of the check evaluators.
func (t *TenantClient) ImpersonateServiceAccount(namespace, name string) (dynamic.Interface, error) {
	cfg := rest.CopyConfig(t.config)
	cfg.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("system:serviceaccount:%s:%s", namespace, name),
	}
	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build impersonating client: %w", err)
	}
	return client, nil
}

// TenantClientFactory builds and caches tenant clients keyed by session claim
// name. The cache is a small LRU (grading a session repeatedly must not
// re-read the Secret every time, §5.1) and is invalidated on session delete.
// Per-replica by design: duplication across replicas is just memory (§15).
type TenantClientFactory struct {
	host      dynamic.Interface
	namespace string // claim namespace (sessions live in {namespace}-{name})

	mu      sync.Mutex
	max     int
	order   *list.List // front = most recently used; values are cache keys
	entries map[string]*tenantCacheEntry

	// buildClient is injectable for tests (the default builds a real client
	// from kubeconfig bytes).
	buildClient func(kubeconfig []byte) (*TenantClient, error)
}

type tenantCacheEntry struct {
	client *TenantClient
	elem   *list.Element
}

// NewTenantClientFactory constructs a factory reading kubeconfig Secrets via
// the host client. maxEntries <= 0 defaults to 32.
func NewTenantClientFactory(host dynamic.Interface, claimNamespace string, maxEntries int) *TenantClientFactory {
	if maxEntries <= 0 {
		maxEntries = 32
	}
	return &TenantClientFactory{
		host:        host,
		namespace:   claimNamespace,
		max:         maxEntries,
		order:       list.New(),
		entries:     map[string]*tenantCacheEntry{},
		buildClient: buildTenantClient,
	}
}

// ForSession returns the tenant client for a session claim (by claim name),
// building and caching it on first use.
func (f *TenantClientFactory) ForSession(ctx context.Context, claimName string) (*TenantClient, error) {
	f.mu.Lock()
	if e, ok := f.entries[claimName]; ok {
		f.order.MoveToFront(e.elem)
		f.mu.Unlock()
		return e.client, nil
	}
	f.mu.Unlock()

	// Build outside the lock: Secret read + TLS setup must not serialize all
	// tenant traffic. A racing duplicate build is harmless (last one wins).
	kubeconfig, err := f.fetchKubeconfig(ctx, claimName)
	if err != nil {
		return nil, err
	}
	client, err := f.buildClient(kubeconfig)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if e, ok := f.entries[claimName]; ok {
		f.order.MoveToFront(e.elem)
		return e.client, nil
	}
	elem := f.order.PushFront(claimName)
	f.entries[claimName] = &tenantCacheEntry{client: client, elem: elem}
	for f.order.Len() > f.max {
		oldest := f.order.Back()
		f.order.Remove(oldest)
		delete(f.entries, oldest.Value.(string))
	}
	return client, nil
}

// Invalidate drops a session's cached client. Call on session delete/recycle;
// a stale client would otherwise dial a torn-down vcluster.
func (f *TenantClientFactory) Invalidate(claimName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e, ok := f.entries[claimName]; ok {
		f.order.Remove(e.elem)
		delete(f.entries, claimName)
	}
}

// fetchKubeconfig reads the composed vc-{ns}-{name}-vcluster Secret from the
// session namespace and returns the kubeconfig bytes (key "config").
func (f *TenantClientFactory) fetchKubeconfig(ctx context.Context, claimName string) ([]byte, error) {
	sessionNS := f.namespace + "-" + claimName
	secretName := "vc-" + sessionNS + "-vcluster"
	obj, err := f.host.Resource(secretGVR).Namespace(sessionNS).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get tenant kubeconfig secret %s/%s: %w", sessionNS, secretName, err)
	}
	kc, err := secretValue(obj, "config")
	if err != nil {
		return nil, fmt.Errorf("secret %s/%s: %w", sessionNS, secretName, err)
	}
	return kc, nil
}

// buildTenantClient turns kubeconfig bytes into a live client.
func buildTenantClient(kubeconfig []byte) (*TenantClient, error) {
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("parse tenant kubeconfig: %w", err)
	}
	// Modest client-side limits: seeding applies a handful of objects and
	// grading is a handful of GETs — nothing here should ever be able to
	// hammer a tenant API server.
	cfg.QPS = 20
	cfg.Burst = 40
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build tenant dynamic client: %w", err)
	}
	return &TenantClient{Dynamic: dyn, config: cfg}, nil
}

// secretValue extracts and decodes one key of a dynamic-client Secret.
// Unstructured Secrets carry data as base64 strings.
func secretValue(obj *unstructured.Unstructured, key string) ([]byte, error) {
	data, ok, _ := unstructured.NestedStringMap(obj.Object, "data")
	if !ok {
		return nil, fmt.Errorf("no data")
	}
	b64, ok := data[key]
	if !ok || b64 == "" {
		return nil, fmt.Errorf("missing key %q", key)
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode key %q: %w", key, err)
	}
	return raw, nil
}
