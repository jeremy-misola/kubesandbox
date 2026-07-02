package kubernetes

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// Sentinel errors returned by the service and mapped to HTTP codes by handlers.
var (
	ErrNotFound      = errors.New("session not found")
	ErrAlreadyExists = errors.New("session already exists for this user")
	ErrInvalidID     = errors.New("invalid session id")
	// ErrPoolEmpty means no warm pool member is Ready for assignment. Handlers
	// queue the request (Phase E); they must NOT fall back to a synchronous
	// cold build on the request path (docs/08 §2.2).
	ErrPoolEmpty = errors.New("no warm sandbox available")
)

const (
	ownerLabel     = "kubesandbox.com/owner"
	ownerRefAnnot  = "kubesandbox.com/owner-ref"
	managedByLabel = "app.kubernetes.io/managed-by"
	managedByValue = "kubesandbox-backend"

	// poolLabel marks hot-pool members on the claim. "available" = warm,
	// unclaimed, hand-out candidate; "claimed" = assigned to an owner.
	// Claims without the label are legacy direct-created sessions.
	poolLabel     = "kubesandbox.com/pool"
	poolAvailable = "available"
	poolClaimed   = "claimed"

	// warmNamePrefix prefixes generated pool-member claim names. Warm claims
	// cannot use the owner-derived name (owner is unknown at warm time), so
	// they get "s-pool-<rand>"; the one-per-user guarantee moves to the
	// per-owner marker object (see markers below).
	warmNamePrefix = "s-pool-"

	// markerNamePrefix prefixes per-owner marker ConfigMaps. The marker name is
	// derived from the owner, so a duplicate create fails AlreadyExists at the
	// API server — this restores the atomic one-sandbox-per-user guarantee that
	// the deterministic claim name used to provide.
	markerNamePrefix = "sbxowner-"
	markerLabel      = "kubesandbox.com/owner-marker"

	markerKeyOwner  = "owner"
	markerKeyMember = "member"
)

// configMapGVR is used for per-owner marker objects.
var configMapGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

// SessionService performs CRUD on KubeSandboxSession claims for a single owner
// at a time, plus hot warm-pool operations (warm provisioning, atomic
// assignment, per-owner markers). It is safe for concurrent use (the dynamic
// client is).
type SessionService struct {
	client       dynamic.Interface
	namespace    string
	baseURL      string
	defaultImage string

	// maxWarmAge: an available member older than this is never handed out
	// (freshness, Phase E); the pool manager recycles it instead. Zero
	// disables the freshness check.
	maxWarmAge time.Duration

	// now is injectable for tests.
	now func() time.Time
}

// NewSessionService constructs a SessionService.
func NewSessionService(client dynamic.Interface, namespace, baseURL string, defaultImage string) *SessionService {
	return &SessionService{
		client:       client,
		namespace:    namespace,
		baseURL:      strings.TrimRight(baseURL, "/"),
		defaultImage: defaultImage,
		now:          time.Now,
	}
}

// SetMaxWarmAge configures the freshness ceiling for hand-outs.
func (s *SessionService) SetMaxWarmAge(d time.Duration) { s.maxWarmAge = d }

func (s *SessionService) resource() dynamic.ResourceInterface {
	return s.client.Resource(models.GVR).Namespace(s.namespace)
}

func (s *SessionService) configmaps() dynamic.ResourceInterface {
	return s.client.Resource(configMapGVR).Namespace(s.namespace)
}

// clampTTL applies the XRD's TTL bounds.
func clampTTL(ttl int) int {
	if ttl == 0 {
		ttl = models.DefaultTTLMinutes
	}
	if ttl < models.MinTTLMinutes {
		ttl = models.MinTTLMinutes
	}
	if ttl > models.MaxTTLMinutes {
		ttl = models.MaxTTLMinutes
	}
	return ttl
}

// ---------------------------------------------------------------------------
// Assignment (Phase C+D): claim a warm pool member for an owner.
// ---------------------------------------------------------------------------

// Assign hands an available, Ready, fresh pool member to ownerRef.
//
// Atomicity:
//
//   - One-per-user: a per-owner marker ConfigMap (name derived from the owner)
//     is created FIRST; a concurrent duplicate create fails AlreadyExists at
//     the API server, so two simultaneous requests from the same user can
//     never both proceed. (Phase D — replaces the owner-derived claim name.)
//   - One-user-per-member: the member is claimed with a plain Update carrying
//     the resourceVersion observed at list time — optimistic concurrency. Two
//     requests racing for the same member conflict; the loser retries with the
//     next free member.
//
// Failure handling: if no member can be claimed the marker is rolled back and
// ErrPoolEmpty is returned so the handler can queue the request. A crash
// between marker create and member claim leaves an orphaned marker; the pool
// manager garbage-collects markers with no matching owned claim.
func (s *SessionService) Assign(ctx context.Context, ownerRef string, req models.CreateSessionRequest) (*models.Session, error) {
	if ownerRef == "" {
		return nil, fmt.Errorf("empty ownerRef")
	}
	ttl := clampTTL(req.TTLMinutes)

	// Step 1 — atomic one-per-user reservation.
	if err := s.createOwnerMarker(ctx, ownerRef); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("create owner marker: %w", err)
	}

	// Step 2 — migration guard: users holding a legacy (pre-pool) session have
	// no marker, so the create above succeeded; refuse a second sandbox.
	existing, err := s.List(ctx, ownerRef)
	if err != nil {
		_ = s.deleteOwnerMarker(ctx, ownerRef)
		return nil, err
	}
	if len(existing) > 0 {
		_ = s.deleteOwnerMarker(ctx, ownerRef)
		return nil, ErrAlreadyExists
	}

	// Step 3 — claim a member with optimistic concurrency, oldest first (FIFO
	// keeps recycling churn low).
	members, err := s.listAssignableMembers(ctx)
	if err != nil {
		_ = s.deleteOwnerMarker(ctx, ownerRef)
		return nil, err
	}

	expiresAt := s.now().UTC().Add(time.Duration(ttl) * time.Minute).Format(time.RFC3339)
	for i := range members {
		m := members[i].DeepCopy()

		_ = unstructured.SetNestedField(m.Object, ownerRef, "spec", "ownerRef")
		_ = unstructured.SetNestedField(m.Object, ownerRef, "spec", "tenantRef")
		_ = unstructured.SetNestedField(m.Object, int64(ttl), "spec", "ttlMinutes")
		// TTL starts at assignment, not at (warm) creation: spec.expiresAt is
		// the authoritative expiry (Phase A prerequisite; read by cleanup.go
		// and surfaced to status.expiresAt by the composition).
		_ = unstructured.SetNestedField(m.Object, expiresAt, "spec", "expiresAt")

		labels := m.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[ownerLabel] = ownerHash(ownerRef)
		labels[poolLabel] = poolClaimed
		m.SetLabels(labels)

		annots := m.GetAnnotations()
		if annots == nil {
			annots = map[string]string{}
		}
		annots[ownerRefAnnot] = ownerRef
		m.SetAnnotations(annots)

		updated, err := s.resource().Update(ctx, m, metav1.UpdateOptions{})
		if err != nil {
			if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
				continue // raced with another assignment/recycle; try next member
			}
			_ = s.deleteOwnerMarker(ctx, ownerRef)
			return nil, fmt.Errorf("claim pool member %q: %w", m.GetName(), err)
		}

		// Record which member the owner holds (informational; best-effort).
		s.setOwnerMarkerMember(ctx, ownerRef, updated.GetName())
		sess := s.ToSession(updated)
		return &sess, nil
	}

	// No member could be claimed — release the reservation and let the
	// handler queue the request.
	_ = s.deleteOwnerMarker(ctx, ownerRef)
	return nil, ErrPoolEmpty
}

// listAssignableMembers returns pool members that are available, not deleting,
// Ready, and fresh — oldest first.
func (s *SessionService) listAssignableMembers(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := s.resource().List(ctx, metav1.ListOptions{
		LabelSelector: managedByLabel + "=" + managedByValue + "," + poolLabel + "=" + poolAvailable,
	})
	if err != nil {
		return nil, fmt.Errorf("list pool members: %w", err)
	}
	out := make([]unstructured.Unstructured, 0, len(list.Items))
	now := s.now()
	for i := range list.Items {
		m := list.Items[i]
		if m.GetDeletionTimestamp() != nil {
			continue
		}
		if ready, _, _ := unstructured.NestedBool(m.Object, "status", "workspaceReady"); !ready {
			continue
		}
		if s.maxWarmAge > 0 && now.Sub(m.GetCreationTimestamp().Time) > s.maxWarmAge {
			continue // stale — recycled by the pool manager, never handed out
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].GetCreationTimestamp().Time.Before(out[j].GetCreationTimestamp().Time)
	})
	return out, nil
}

// ---------------------------------------------------------------------------
// Warm provisioning (Phase B): called by the pool manager, never on the
// request path.
// ---------------------------------------------------------------------------

// CreateWarm provisions one unclaimed pool member through the existing
// composition (namespace, vcluster, shell pod, route). It carries no owner —
// tenantRef/ownerRef are empty until assignment — and the pool=available
// label marks it as a hand-out candidate once Ready.
func (s *SessionService) CreateWarm(ctx context.Context) (*models.Session, error) {
	name := warmNamePrefix + randSuffix()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": models.APIGroup + "/" + models.APIVersion,
			"kind":       models.Kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": s.namespace,
				"labels": map[string]interface{}{
					managedByLabel: managedByValue,
					poolLabel:      poolAvailable,
				},
			},
			"spec": map[string]interface{}{
				"tenantRef":      "",
				"ownerRef":       "",
				"ttlMinutes":     int64(models.DefaultTTLMinutes),
				"workspaceImage": s.defaultImage,
				"resources": map[string]interface{}{
					"cpu":    models.DefaultResources.CPU,
					"memory": models.DefaultResources.Memory,
				},
			},
		},
	}

	created, err := s.resource().Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create warm claim: %w", err)
	}
	sess := s.ToSession(created)
	return &sess, nil
}

// randSuffix returns a 10-char hex suffix for warm claim names.
func randSuffix() string {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		// Fall back to a time-derived suffix; collisions surface as
		// AlreadyExists on create and are retried by the pool manager.
		return fmt.Sprintf("%010x", time.Now().UnixNano()&0xffffffffff)
	}
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Per-owner markers (Phase D).
// ---------------------------------------------------------------------------

// markerName derives the deterministic marker name for an owner.
func markerName(ownerRef string) string {
	return markerNamePrefix + ownerHash(ownerRef)[:16]
}

// createOwnerMarker atomically reserves the one-sandbox-per-user slot.
func (s *SessionService) createOwnerMarker(ctx context.Context, ownerRef string) error {
	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      markerName(ownerRef),
				"namespace": s.namespace,
				"labels": map[string]interface{}{
					managedByLabel: managedByValue,
					markerLabel:    "true",
				},
			},
			"data": map[string]interface{}{
				markerKeyOwner:  ownerRef,
				markerKeyMember: "",
			},
		},
	}
	_, err := s.configmaps().Create(ctx, cm, metav1.CreateOptions{})
	return err
}

// setOwnerMarkerMember records which pool member the owner holds. Best-effort:
// the marker's existence (not its data) is what enforces the invariant.
func (s *SessionService) setOwnerMarkerMember(ctx context.Context, ownerRef, member string) {
	cm, err := s.configmaps().Get(ctx, markerName(ownerRef), metav1.GetOptions{})
	if err != nil {
		return
	}
	_ = unstructured.SetNestedField(cm.Object, member, "data", markerKeyMember)
	_, _ = s.configmaps().Update(ctx, cm, metav1.UpdateOptions{})
}

// deleteOwnerMarker releases an owner's slot. Missing markers (legacy
// sessions) are a no-op.
func (s *SessionService) deleteOwnerMarker(ctx context.Context, ownerRef string) error {
	err := s.configmaps().Delete(ctx, markerName(ownerRef), metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete owner marker: %w", err)
	}
	return nil
}

// listOwnerMarkers returns all per-owner markers (used by the pool manager's
// orphan GC).
func (s *SessionService) listOwnerMarkers(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := s.configmaps().List(ctx, metav1.ListOptions{
		LabelSelector: managedByLabel + "=" + managedByValue + "," + markerLabel + "=true",
	})
	if err != nil {
		return nil, fmt.Errorf("list owner markers: %w", err)
	}
	return list.Items, nil
}

// ---------------------------------------------------------------------------
// Legacy direct create (kept as the pool-disabled fallback; no longer used on
// the request path when the pool is enabled).
// ---------------------------------------------------------------------------

// Create mints a new claim owned by ownerRef with the deterministic per-owner
// name (one-per-user via AlreadyExists). Used only when the warm pool is
// disabled (e.g. local dev); the hot-pool request path is Assign.
func (s *SessionService) Create(ctx context.Context, ownerRef string, req models.CreateSessionRequest) (*models.Session, error) {
	ttl := clampTTL(req.TTLMinutes)

	image := req.WorkspaceImage
	if image == "" {
		image = s.defaultImage
	}

	name := sessionName(ownerRef)
	expiresAt := s.now().UTC().Add(time.Duration(ttl) * time.Minute).Format(time.RFC3339)

	spec := map[string]interface{}{
		"tenantRef":      ownerRef,
		"ownerRef":       ownerRef,
		"ttlMinutes":     int64(ttl),
		"expiresAt":      expiresAt,
		"workspaceImage": image,
		"resources": map[string]interface{}{
			"cpu":    models.DefaultResources.CPU,
			"memory": models.DefaultResources.Memory,
		},
	}
	if req.StarterLabRef != "" {
		spec["starterLabRef"] = req.StarterLabRef
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": models.APIGroup + "/" + models.APIVersion,
			"kind":       models.Kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": s.namespace,
				"labels": map[string]interface{}{
					managedByLabel: managedByValue,
					ownerLabel:     ownerHash(ownerRef),
				},
				"annotations": map[string]interface{}{
					ownerRefAnnot: ownerRef,
				},
			},
			"spec": spec,
		},
	}

	created, err := s.resource().Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("create claim: %w", err)
	}
	sess := s.ToSession(created)
	return &sess, nil
}

// ---------------------------------------------------------------------------
// Reads, deletes, authz (unchanged semantics; pool-aware guards added).
// ---------------------------------------------------------------------------

// List returns all sessions owned by ownerRef.
func (s *SessionService) List(ctx context.Context, ownerRef string) ([]models.Session, error) {
	if ownerRef == "" {
		return nil, nil // unclaimed pool members are ownerless; never listable
	}
	list, err := s.resource().List(ctx, metav1.ListOptions{
		LabelSelector: ownerLabel + "=" + ownerHash(ownerRef),
	})
	if err != nil {
		return nil, fmt.Errorf("list claims: %w", err)
	}
	out := make([]models.Session, 0, len(list.Items))
	for i := range list.Items {
		item := list.Items[i]
		// Defense in depth: a label collision must never expose another owner.
		if specOwner(&item) != ownerRef {
			continue
		}
		out = append(out, s.ToSession(&item))
	}
	return out, nil
}

// Get returns a single session by public id, but only if owned by ownerRef.
// Unknown or unowned ids both yield ErrNotFound (no existence leak).
func (s *SessionService) Get(ctx context.Context, id, ownerRef string) (*models.Session, error) {
	obj, err := s.getOwned(ctx, id, ownerRef)
	if err != nil {
		return nil, err
	}
	sess := s.ToSession(obj)
	return &sess, nil
}

// Delete removes a session by public id, only if owned by ownerRef. It also
// releases the owner's marker so the user can create a new sandbox.
func (s *SessionService) Delete(ctx context.Context, id, ownerRef string) error {
	obj, err := s.getOwned(ctx, id, ownerRef)
	if err != nil {
		return err
	}
	if err := s.resource().Delete(ctx, obj.GetName(), metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("delete claim: %w", err)
	}
	// Release the one-per-user slot (no-op for legacy sessions without one).
	_ = s.deleteOwnerMarker(ctx, ownerRef)
	return nil
}

// Authorize reports whether ownerRef owns the session identified by public id.
// It returns nil when the caller owns the claim, and ErrNotFound when the id is
// unknown, unowned, or malformed (ErrInvalidID). Callers that surface this to an
// untrusted client — notably the /authz ext-authz endpoint (G2) — MUST collapse
// unknown/unowned/malformed into a single denial so they cannot be distinguished
// (no existence leak).
func (s *SessionService) Authorize(ctx context.Context, id, ownerRef string) error {
	_, err := s.getOwned(ctx, id, ownerRef)
	return err
}

// getOwned fetches a claim by public id and enforces ownership. An empty
// ownerRef never matches anything: unclaimed pool members carry an empty
// spec.ownerRef and must be unreachable through owner-scoped paths.
func (s *SessionService) getOwned(ctx context.Context, id, ownerRef string) (*unstructured.Unstructured, error) {
	if ownerRef == "" {
		return nil, ErrNotFound
	}
	name, err := s.nameFromID(id)
	if err != nil {
		return nil, err
	}
	obj, err := s.resource().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get claim: %w", err)
	}
	if specOwner(obj) != ownerRef {
		return nil, ErrNotFound
	}
	return obj, nil
}

// Watch returns a watch scoped to a single claim by name (used by SSE).
func (s *SessionService) Watch(ctx context.Context, name string) (watch.Interface, error) {
	return s.resource().Watch(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
	})
}

// WatchManaged returns a watch over every claim this backend manages (used by
// the pool manager to react to pool/live changes without polling).
func (s *SessionService) WatchManaged(ctx context.Context) (watch.Interface, error) {
	return s.resource().Watch(ctx, metav1.ListOptions{
		LabelSelector: managedByLabel + "=" + managedByValue,
	})
}

// listManaged returns every claim this backend manages, across all owners. Used
// by the TTL controller (G3) and the pool manager (Phase B).
func (s *SessionService) listManaged(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := s.resource().List(ctx, metav1.ListOptions{
		LabelSelector: managedByLabel + "=" + managedByValue,
	})
	if err != nil {
		return nil, fmt.Errorf("list managed claims: %w", err)
	}
	return list.Items, nil
}

// deleteByName deletes a claim by its (non-public) name with background
// propagation, so a slow child finalizer never blocks the caller. An
// already-gone claim is treated as success.
func (s *SessionService) deleteByName(ctx context.Context, name string) error {
	policy := metav1.DeletePropagationBackground
	if err := s.resource().Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &policy}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete claim %q: %w", name, err)
	}
	return nil
}

// NameFromID exposes id->name resolution for handlers (e.g. SSE).
func (s *SessionService) NameFromID(id string) (string, error) {
	return s.nameFromID(id)
}

// ToSession converts a claim into the API representation.
func (s *SessionService) ToSession(obj *unstructured.Unstructured) models.Session {
	name := obj.GetName()
	ns := obj.GetNamespace()
	sess := models.Session{
		ID:        ns + "-" + name,
		Name:      name,
		Namespace: ns,
		CreatedAt: obj.GetCreationTimestamp().UTC().Format(time.RFC3339),
	}

	if spec, ok, _ := unstructured.NestedMap(obj.Object, "spec"); ok {
		sess.TenantRef, _ = spec["tenantRef"].(string)
		sess.OwnerRef, _ = spec["ownerRef"].(string)
		sess.WorkspaceImage, _ = spec["workspaceImage"].(string)
		sess.StarterLabRef, _ = spec["starterLabRef"].(string)
		sess.TTLMinutes = toInt(spec["ttlMinutes"])
		// spec.expiresAt is authoritative (set at assignment); status.expiresAt
		// below is the composition's echo and may lag a reconcile.
		sess.ExpiresAt, _ = spec["expiresAt"].(string)
		if r, ok := spec["resources"].(map[string]interface{}); ok {
			sess.Resources.CPU, _ = r["cpu"].(string)
			sess.Resources.Memory, _ = r["memory"].(string)
		}
	}

	if st, ok, _ := unstructured.NestedMap(obj.Object, "status"); ok {
		sess.Phase, _ = st["phase"].(string)
		sess.Message, _ = st["message"].(string)
		sess.WorkspaceReady, _ = st["workspaceReady"].(bool)
		sess.SessionNamespace, _ = st["sessionNamespace"].(string)
		if sess.ExpiresAt == "" {
			sess.ExpiresAt, _ = st["expiresAt"].(string)
		}
	}

	if sess.Phase == "" {
		sess.Phase = "Pending"
	}
	sess.URL = s.baseURL + "/s/" + sess.ID
	return sess
}

// nameFromID converts the public id "{namespace}-{name}" back to the claim name.
// The configured namespace is the only valid prefix.
func (s *SessionService) nameFromID(id string) (string, error) {
	prefix := s.namespace + "-"
	if !strings.HasPrefix(id, prefix) {
		return "", ErrInvalidID
	}
	name := strings.TrimPrefix(id, prefix)
	if name == "" || !strings.HasPrefix(name, "s-") {
		return "", ErrInvalidID
	}
	return name, nil
}

// sessionName derives the deterministic claim name for an owner, e.g.
// "s-1a2b3c4d5e6f7a8b". Used only by the legacy direct-create path; pool
// members have generated names and the one-per-user invariant lives in the
// per-owner marker instead.
func sessionName(ownerRef string) string {
	return "s-" + ownerHash(ownerRef)[:16]
}

// ownerHash produces a label-safe (DNS-1123) hash of an owner identifier,
// since owner ids (emails) are not valid label values.
func ownerHash(owner string) string {
	sum := sha256.Sum256([]byte(owner))
	return hex.EncodeToString(sum[:])[:32]
}

// specOwner reads spec.ownerRef from a claim.
func specOwner(obj *unstructured.Unstructured) string {
	v, _, _ := unstructured.NestedString(obj.Object, "spec", "ownerRef")
	return v
}

// poolState reads the pool label ("", "available", or "claimed").
func poolState(obj *unstructured.Unstructured) string {
	return obj.GetLabels()[poolLabel]
}

// toInt coerces the various numeric types unstructured may hold into an int.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}
