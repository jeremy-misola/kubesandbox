package content

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/jeremy-misola/kubesandbox/backend/internal/telemetry"
)

// Store is the read interface the API and the seeder/grader consume. The
// implementation is swappable (§4: content storage lives behind this
// interface, so a move to a dedicated content repo or another delivery
// mechanism never touches consumers).
type Store interface {
	// Get returns the bundle by id.
	Get(id string) (*Bundle, bool)
	// List returns catalog metadata, sorted by id.
	List() []Meta
}

var configMapGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

// ConfigMapStore watches one-ConfigMap-per-challenge bundles (label
// kubesandbox.com/challenge-bundle=true) in a single namespace and keeps an
// in-memory catalog. It is level-triggered like the pool manager: a watch
// pokes a full rebuild from a fresh LIST, with a periodic resync backstop, so
// missed events are harmless and a restart self-heals. (Same mechanism as a
// namespace-scoped informer, in the codebase's existing idiom.)
//
// Invalid bundles are QUARANTINED: skipped, logged, and surfaced through the
// content_bundle_invalid gauge — never a crash, and never failing the whole
// catalog (§4). This includes bundles with an unknown apiVersion (schema skew
// during a rollout).
type ConfigMapStore struct {
	client    dynamic.Interface
	namespace string
	resync    time.Duration

	mu      sync.RWMutex
	bundles map[string]*Bundle
	invalid map[string]string // ConfigMap name → reason

	metrics *telemetry.Metrics
	poke    chan struct{}
}

// NewConfigMapStore constructs a store watching namespace. A non-positive
// resync defaults to 5 minutes.
func NewConfigMapStore(client dynamic.Interface, namespace string, resync time.Duration) *ConfigMapStore {
	if resync <= 0 {
		resync = 5 * time.Minute
	}
	return &ConfigMapStore{
		client:    client,
		namespace: namespace,
		resync:    resync,
		bundles:   map[string]*Bundle{},
		invalid:   map[string]string{},
		poke:      make(chan struct{}, 1),
	}
}

// SetMetrics injects the telemetry instrument set (nil is a valid no-op) and
// wires the content_bundle_invalid gauge to the quarantine set.
func (s *ConfigMapStore) SetMetrics(m *telemetry.Metrics) {
	s.metrics = m
	m.RegisterContentInvalid(func() map[string]int64 {
		s.mu.RLock()
		defer s.mu.RUnlock()
		out := make(map[string]int64, len(s.invalid))
		for name := range s.invalid {
			out[name] = 1
		}
		return out
	})
}

// Get implements Store.
func (s *ConfigMapStore) Get(id string) (*Bundle, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bundles[id]
	return b, ok
}

// List implements Store.
func (s *ConfigMapStore) List() []Meta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Meta, 0, len(s.bundles))
	for _, b := range s.bundles {
		out = append(out, b.Meta())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Run rebuilds the catalog until ctx is cancelled: once immediately, on every
// watch event, and on the resync tick.
func (s *ConfigMapStore) Run(ctx context.Context) {
	log.Printf("content: store started (namespace=%s resync=%s)", s.namespace, s.resync)
	go s.watchLoop(ctx)

	ticker := time.NewTicker(s.resync)
	defer ticker.Stop()
	for {
		if err := s.RebuildOnce(ctx); err != nil {
			log.Printf("content: rebuild error: %v", err)
		}
		select {
		case <-ctx.Done():
			log.Printf("content: store stopped")
			return
		case <-ticker.C:
		case <-s.poke:
		}
	}
}

// Poke requests an immediate rebuild (non-blocking; coalesced).
func (s *ConfigMapStore) Poke() {
	select {
	case s.poke <- struct{}{}:
	default:
	}
}

// watchLoop pokes the rebuild on any bundle-ConfigMap event, reconnecting
// with a small backoff when the watch drops.
func (s *ConfigMapStore) watchLoop(ctx context.Context) {
	for {
		w, err := s.client.Resource(configMapGVR).Namespace(s.namespace).Watch(ctx, metav1.ListOptions{
			LabelSelector: BundleConfigMapLabel + "=true",
		})
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
		for range w.ResultChan() {
			s.Poke()
		}
		w.Stop()
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// RebuildOnce replaces the catalog from a fresh LIST. Exported for tests and
// for a synchronous initial fill at startup.
func (s *ConfigMapStore) RebuildOnce(ctx context.Context) error {
	list, err := s.client.Resource(configMapGVR).Namespace(s.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: BundleConfigMapLabel + "=true",
	})
	if err != nil {
		return fmt.Errorf("list bundle configmaps: %w", err)
	}

	bundles := map[string]*Bundle{}
	invalid := map[string]string{}
	for i := range list.Items {
		cm := &list.Items[i]
		b, err := bundleFromConfigMap(cm)
		if err != nil {
			// Quarantine: skip + log + metric, never crash (§4).
			invalid[cm.GetName()] = err.Error()
			log.Printf("content: quarantined bundle ConfigMap %s: %v", cm.GetName(), err)
			continue
		}
		if prev, dup := bundles[b.ID]; dup {
			invalid[cm.GetName()] = fmt.Sprintf("duplicate bundle id %q (already served by another ConfigMap)", b.ID)
			log.Printf("content: quarantined %s: duplicate id %q (keeping %q)", cm.GetName(), b.ID, prev.ID)
			continue
		}
		bundles[b.ID] = b
	}

	s.mu.Lock()
	changed := len(bundles) != len(s.bundles) || len(invalid) != len(s.invalid)
	if !changed {
		for id := range bundles {
			if _, ok := s.bundles[id]; !ok {
				changed = true
				break
			}
		}
	}
	s.bundles = bundles
	s.invalid = invalid
	s.mu.Unlock()

	if changed {
		log.Printf("content: catalog rebuilt (%d bundles, %d quarantined)", len(bundles), len(invalid))
	}
	return nil
}

// bundleFromConfigMap parses one delivery ConfigMap: key "challenge.yaml"
// plus one key per seed file basename.
func bundleFromConfigMap(cm *unstructured.Unstructured) (*Bundle, error) {
	data, _, _ := unstructured.NestedStringMap(cm.Object, "data")
	challengeYAML, ok := data["challenge.yaml"]
	if !ok {
		return nil, fmt.Errorf("missing challenge.yaml key")
	}
	seed := map[string][]byte{}
	raw := len(challengeYAML)
	for k, v := range data {
		if k == "challenge.yaml" {
			continue
		}
		seed[k] = []byte(v)
		raw += len(v)
	}
	b, err := LoadBundle([]byte(challengeYAML), seed)
	if err != nil {
		return nil, err
	}
	// The ConfigMap name must be the id-derived name — catches copy-paste
	// drift between directory (chart-derived name) and challenge.yaml id.
	if want := BundleConfigMapPrefix + b.ID; cm.GetName() != want {
		return nil, fmt.Errorf("ConfigMap name %q does not match bundle id (want %q)", cm.GetName(), want)
	}
	if w := b.SizeWarning(raw); w != "" {
		log.Printf("content: %s", w)
	}
	return b, nil
}

// FixedStore is a Store over a static bundle set (tests, and potentially a
// baked-in fallback). Not used on the production path.
type FixedStore map[string]*Bundle

// Get implements Store.
func (f FixedStore) Get(id string) (*Bundle, bool) { b, ok := f[id]; return b, ok }

// List implements Store.
func (f FixedStore) List() []Meta {
	out := make([]Meta, 0, len(f))
	for _, b := range f {
		out = append(out, b.Meta())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ValidateDir is a convenience for the CLI: it lints raw bundle files and
// returns the parsed bundle. dirName is the challenge directory basename and
// must equal the bundle id.
func ValidateDir(dirName string, challengeYAML []byte, seedFiles map[string][]byte) (*Bundle, []string) {
	b, err := LoadBundle(challengeYAML, seedFiles)
	if err != nil {
		return nil, []string{err.Error()}
	}
	var errs []string
	if b.ID != dirName {
		errs = append(errs, fmt.Sprintf("id %q does not match directory name %q", b.ID, dirName))
	}
	raw := len(challengeYAML)
	for _, v := range seedFiles {
		raw += len(v)
	}
	if w := b.SizeWarning(raw); w != "" {
		// Size is a warning, not an error; the CLI prints it separately.
		log.Printf("warning: %s", w)
	}
	if len(errs) > 0 {
		return b, errs
	}
	return b, nil
}

// String renders the quarantine set for diagnostics.
func (s *ConfigMapStore) Quarantined() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.invalid))
	for k, v := range s.invalid {
		out[k] = v
	}
	return out
}

var _ Store = (*ConfigMapStore)(nil)
var _ Store = (FixedStore)(nil)
