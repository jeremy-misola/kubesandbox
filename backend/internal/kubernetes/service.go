package kubernetes

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	"github.com/jeremy-misola/kubesandbox/backend/internal/telemetry"
)

// Sentinel errors returned by the service and mapped to HTTP codes by handlers.
var (
	ErrNotFound      = errors.New("session not found")
	ErrAlreadyExists = errors.New("session already exists for this user")
	ErrInvalidID     = errors.New("invalid session id")
	// ErrPoolEmpty means no warm pool member is Ready for assignment; handlers
	// queue the request rather than fall back to a synchronous cold build.
	ErrPoolEmpty = errors.New("no warm sandbox available")
)

const (
	ownerLabel     = "kubesandbox.com/owner"
	ownerRefAnnot  = "kubesandbox.com/owner-ref"
	managedByLabel = "app.kubernetes.io/managed-by"
	managedByValue = "kubesandbox-backend"

	// poolLabel marks hot-pool members: "available" = warm and unclaimed,
	// "claimed" = assigned. Members without the label are legacy sessions.
	poolLabel     = "kubesandbox.com/pool"
	poolAvailable = "available"
	poolClaimed   = "claimed"

	// warmNamePrefix prefixes generated pool-member names (owner is unknown at
	// warm time, so the name can't be owner-derived).
	warmNamePrefix = "s-pool-"

	// markerNamePrefix prefixes per-owner marker ConfigMaps. The name is
	// owner-derived, so a duplicate create fails AlreadyExists at the API
	// server — this is the atomic one-sandbox-per-user guarantee.
	markerNamePrefix = "sbxowner-"
	markerLabel      = "kubesandbox.com/owner-marker"

	markerKeyOwner  = "owner"
	markerKeyMember = "member"
)

// configMapGVR is used for per-owner marker objects.
var configMapGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

// SessionService performs CRUD on KubeSandboxSession claims for a single owner,
// plus warm-pool operations (provisioning, atomic assignment, per-owner
// markers). It is safe for concurrent use.
type SessionService struct {
	client       dynamic.Interface
	namespace    string
	baseURL      string
	defaultImage string

	// maxWarmAge: an available member older than this is never handed out; the
	// pool manager recycles it. Zero disables the check.
	maxWarmAge time.Duration

	// metrics is the injected instrument set; nil (tests, telemetry disabled)
	// is a valid no-op.
	metrics *telemetry.Metrics

	now func() time.Time // injectable for tests
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

// SetMetrics injects the telemetry instrument set (nil is a valid no-op).
func (s *SessionService) SetMetrics(m *telemetry.Metrics) { s.metrics = m }

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

// defaultResourcesMap is the uniform sandbox resource shape as an unstructured
// spec fragment.
func defaultResourcesMap() map[string]interface{} {
	return map[string]interface{}{
		"cpu":    models.DefaultResources.CPU,
		"memory": models.DefaultResources.Memory,
	}
}

// newClaimObject assembles a KubeSandboxSession claim. annotations may be nil.
func newClaimObject(namespace, name string, labels, annotations map[string]string, spec map[string]interface{}) *unstructured.Unstructured {
	meta := map[string]interface{}{
		"name":      name,
		"namespace": namespace,
		"labels":    toStringMap(labels),
	}
	if len(annotations) > 0 {
		meta["annotations"] = toStringMap(annotations)
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": models.APIGroup + "/" + models.APIVersion,
		"kind":       models.Kind,
		"metadata":   meta,
		"spec":       spec,
	}}
}

func toStringMap(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
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
		// is the composition's echo and may lag a reconcile.
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

// NameFromID exposes id->name resolution for handlers (e.g. SSE).
func (s *SessionService) NameFromID(id string) (string, error) {
	return s.nameFromID(id)
}

// nameFromID converts the public id "{namespace}-{name}" back to the claim
// name. The configured namespace is the only valid prefix.
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

// sessionName derives the deterministic claim name for an owner. Used only by
// the legacy direct-create path; pool members have generated names.
func sessionName(ownerRef string) string {
	return "s-" + ownerHash(ownerRef)[:16]
}

// randSuffix returns a 10-char hex suffix for warm claim names.
func randSuffix() string {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%010x", time.Now().UnixNano()&0xffffffffff)
	}
	return hex.EncodeToString(b)
}

// ownerHash produces a DNS-1123-safe hash of an owner id (emails are not valid
// label values).
func ownerHash(owner string) string {
	sum := sha256.Sum256([]byte(owner))
	return hex.EncodeToString(sum[:])[:32]
}

func specOwner(obj *unstructured.Unstructured) string {
	v, _, _ := unstructured.NestedString(obj.Object, "spec", "ownerRef")
	return v
}

func poolState(obj *unstructured.Unstructured) string {
	return obj.GetLabels()[poolLabel]
}

// toInt coerces the numeric types unstructured may hold into an int.
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
