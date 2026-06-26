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

// Profile controls the default resource shape of a session.
type Profile string

const (
	ProfileStarter  Profile = "starter"
	ProfileStandard Profile = "standard"
	ProfileAdvanced Profile = "advanced"
)

// Resources is the CPU/memory request+limit applied to the shell pod.
type Resources struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// profileResources maps each profile to its resource shape (per the plan).
var profileResources = map[Profile]Resources{
	ProfileStarter:  {CPU: "250m", Memory: "256Mi"},
	ProfileStandard: {CPU: "500m", Memory: "512Mi"},
	ProfileAdvanced: {CPU: "1", Memory: "1Gi"},
}

// Valid reports whether p is a known profile.
func (p Profile) Valid() bool {
	_, ok := profileResources[p]
	return ok
}

// Resources returns the resource shape for the profile (zero value if unknown).
func (p Profile) Resources() Resources {
	return profileResources[p]
}

// CreateSessionRequest is the JSON body of POST /api/sessions.
type CreateSessionRequest struct {
	Profile        Profile `json:"profile"`
	TTLMinutes     int     `json:"ttlMinutes,omitempty"`
	WorkspaceImage string  `json:"workspaceImage,omitempty"`
	StarterLabRef  string  `json:"starterLabRef,omitempty"`
}

// Session is the API representation of a KubeSandboxSession returned to clients.
type Session struct {
	// ID is the public, opaque identifier and routing key: "{namespace}-{name}".
	ID string `json:"id"`
	// Name is the underlying opaque claim name, e.g. "s-1a2b3c4d".
	Name string `json:"name"`
	// Namespace is the claim namespace (the backend's configured namespace).
	Namespace string `json:"namespace"`

	TenantRef      string    `json:"tenantRef"`
	OwnerRef       string    `json:"ownerRef"`
	Profile        string    `json:"profile"`
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
