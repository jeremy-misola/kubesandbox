package kubernetes

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// Sentinel errors returned by the service and mapped to HTTP codes by handlers.
var (
	ErrNotFound      = errors.New("session not found")
	ErrQuotaExceeded = errors.New("session quota exceeded")
	ErrInvalidID     = errors.New("invalid session id")
)

const (
	ownerLabel     = "kubesandbox.com/owner"
	ownerRefAnnot  = "kubesandbox.com/owner-ref"
	managedByLabel = "app.kubernetes.io/managed-by"
	managedByValue = "kubesandbox-backend"
)

// SessionService performs CRUD on KubeSandboxSession claims for a single owner
// at a time. It is safe for concurrent use (the dynamic client is).
type SessionService struct {
	client       dynamic.Interface
	namespace    string
	baseURL      string
	maxPerUser   int
	defaultImage string
}

// NewSessionService constructs a SessionService.
func NewSessionService(client dynamic.Interface, namespace, baseURL string, maxPerUser int, defaultImage string) *SessionService {
	return &SessionService{
		client:       client,
		namespace:    namespace,
		baseURL:      strings.TrimRight(baseURL, "/"),
		maxPerUser:   maxPerUser,
		defaultImage: defaultImage,
	}
}

func (s *SessionService) resource() dynamic.ResourceInterface {
	return s.client.Resource(models.GVR).Namespace(s.namespace)
}

// Create mints a new claim owned by ownerRef. tenantRef is set equal to ownerRef
// (1 tenant = 1 user). It enforces the per-user concurrency cap.
func (s *SessionService) Create(ctx context.Context, ownerRef string, req models.CreateSessionRequest) (*models.Session, error) {
	if !req.Profile.Valid() {
		return nil, fmt.Errorf("invalid profile %q", req.Profile)
	}

	// Enforce the per-user concurrency cap.
	existing, err := s.List(ctx, ownerRef)
	if err != nil {
		return nil, fmt.Errorf("list existing sessions: %w", err)
	}
	if len(existing) >= s.maxPerUser {
		return nil, ErrQuotaExceeded
	}

	ttl := req.TTLMinutes
	if ttl == 0 {
		ttl = models.DefaultTTLMinutes
	}
	if ttl < models.MinTTLMinutes {
		ttl = models.MinTTLMinutes
	}
	if ttl > models.MaxTTLMinutes {
		ttl = models.MaxTTLMinutes
	}

	image := req.WorkspaceImage
	if image == "" {
		image = s.defaultImage
	}

	name, err := mintName()
	if err != nil {
		return nil, fmt.Errorf("mint session name: %w", err)
	}

	res := req.Profile.Resources()
	spec := map[string]interface{}{
		"tenantRef":      ownerRef,
		"ownerRef":       ownerRef,
		"profile":        string(req.Profile),
		"ttlMinutes":     int64(ttl),
		"workspaceImage": image,
		"resources": map[string]interface{}{
			"cpu":    res.CPU,
			"memory": res.Memory,
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
		return nil, fmt.Errorf("create claim: %w", err)
	}
	sess := s.ToSession(created)
	return &sess, nil
}

// List returns all sessions owned by ownerRef.
func (s *SessionService) List(ctx context.Context, ownerRef string) ([]models.Session, error) {
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
	sess := s.ToSession(obj)
	return &sess, nil
}

// Delete removes a session by public id, only if owned by ownerRef.
func (s *SessionService) Delete(ctx context.Context, id, ownerRef string) error {
	name, err := s.nameFromID(id)
	if err != nil {
		return err
	}
	obj, err := s.resource().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("get claim: %w", err)
	}
	if specOwner(obj) != ownerRef {
		return ErrNotFound
	}
	if err := s.resource().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("delete claim: %w", err)
	}
	return nil
}

// Authorize reports whether ownerRef owns the session identified by public id.
// It returns nil when the caller owns the claim, and ErrNotFound when the id is
// unknown, unowned, or malformed (ErrInvalidID). Callers that surface this to an
// untrusted client — notably the /authz ext-authz endpoint (G2) — MUST collapse
// unknown/unowned/malformed into a single denial so they cannot be distinguished
// (no existence leak).
func (s *SessionService) Authorize(ctx context.Context, id, ownerRef string) error {
	name, err := s.nameFromID(id)
	if err != nil {
		return err
	}
	obj, err := s.resource().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("get claim: %w", err)
	}
	if specOwner(obj) != ownerRef {
		return ErrNotFound
	}
	return nil
}

// Watch returns a watch scoped to a single claim by name (used by SSE).
func (s *SessionService) Watch(ctx context.Context, name string) (watch.Interface, error) {
	return s.resource().Watch(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
	})
}

// listManaged returns every claim this backend manages, across all owners. Used
// by the TTL controller (G3), which must see all sessions, not one owner's.
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
// propagation, so a slow child finalizer never blocks the caller. A
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
		sess.Profile, _ = spec["profile"].(string)
		sess.WorkspaceImage, _ = spec["workspaceImage"].(string)
		sess.StarterLabRef, _ = spec["starterLabRef"].(string)
		sess.TTLMinutes = toInt(spec["ttlMinutes"])
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
		sess.ExpiresAt, _ = st["expiresAt"].(string)
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

// mintName returns an opaque random claim name like "s-1a2b3c4d".
func mintName() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "s-" + hex.EncodeToString(b), nil
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
