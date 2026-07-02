// Package models defines the API data transfer objects for KubeSandbox sessions
// and the mapping to the platform.kubesandbox.com/v1alpha1 KubeSandboxSession
// claim. The claim is the source of truth; these types are the JSON shapes the
// backend exchanges with the frontend.
package models

import "k8s.io/apimachinery/pkg/runtime/schema"

// CRD coordinates for the KubeSandboxSession claim.
const (
	APIGroup   = "platform.kubesandbox.com"
	APIVersion = "v1alpha1"
	Kind       = "KubeSandboxSession"
	Plural     = "kubesandboxsessions"

	// DefaultWorkspaceImage matches the XRD default and the composition.
	DefaultWorkspaceImage = "jurassicjey/ttyd-k8s:ttyd"

	// TTL bounds mirror the XRD schema (minimum 15, maximum 1440, default 60).
	DefaultTTLMinutes = 60
	MinTTLMinutes     = 15
	MaxTTLMinutes     = 1440
)

// GVR is the GroupVersionResource used by the dynamic client.
var GVR = schema.GroupVersionResource{
	Group:    APIGroup,
	Version:  APIVersion,
	Resource: Plural,
}

// Resources is the CPU/memory request+limit applied to the shell pod.
type Resources struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// DefaultResources is the single uniform sandbox shape. The platform was
// collapsed to ONE sandbox type (sign-off 2026-07-02, docs/08 §2.2): the
// starter/standard/advanced profiles are gone, which is what makes warm pool
// members globally fungible — any warm sandbox can be handed to any user.
var DefaultResources = Resources{CPU: "500m", Memory: "512Mi"}

// CreateSessionRequest is the JSON body of POST /api/sessions.
//
// Note: there is no profile — every sandbox is identical (single-type
// platform). workspaceImage/starterLabRef are honored only on the legacy
// direct-create path; hot-pool assignment hands over a pre-provisioned,
// uniform sandbox.
type CreateSessionRequest struct {
	TTLMinutes     int    `json:"ttlMinutes,omitempty"`
	WorkspaceImage string `json:"workspaceImage,omitempty"`
	StarterLabRef  string `json:"starterLabRef,omitempty"`
}

// Session is the API representation of a KubeSandboxSession returned to clients.
type Session struct {
	// ID is the public, opaque identifier and routing key: "{namespace}-{name}".
	ID string `json:"id"`
	// Name is the underlying opaque claim name, e.g. "s-1a2b3c4d" or "s-pool-…".
	Name string `json:"name"`
	// Namespace is the claim namespace (the backend's configured namespace).
	Namespace string `json:"namespace"`

	TenantRef      string    `json:"tenantRef"`
	OwnerRef       string    `json:"ownerRef"`
	TTLMinutes     int       `json:"ttlMinutes"`
	WorkspaceImage string    `json:"workspaceImage"`
	StarterLabRef  string    `json:"starterLabRef,omitempty"`
	Resources      Resources `json:"resources"`

	// Status fields surfaced from the claim's .status by Crossplane.
	Phase            string `json:"phase"`
	Message          string `json:"message,omitempty"`
	WorkspaceReady   bool   `json:"workspaceReady"`
	SessionNamespace string `json:"sessionNamespace,omitempty"`
	ExpiresAt        string `json:"expiresAt,omitempty"`

	// URL is the browser terminal URL: "{PublicBaseURL}/s/{id}".
	URL       string `json:"url,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// QueueStatus is returned when the warm pool is empty and the request has been
// queued (Phase E). The client should follow /api/queue/events (SSE) for
// progress and the assigned session.
type QueueStatus struct {
	Status   string `json:"status"` // always "queued"
	Position int    `json:"position"`
	Message  string `json:"message,omitempty"`
}
