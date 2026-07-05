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
	DefaultWorkspaceImage = "jurassicjey/ttyd-k8s:1.0.1"

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

// DefaultResources is the single uniform sandbox shape. Every sandbox is
// identical, which is what makes warm pool members fungible — any warm sandbox
// can be handed to any user.
var DefaultResources = Resources{CPU: "500m", Memory: "512Mi"}

// CreateSessionRequest is the JSON body of POST /api/sessions.
// workspaceImage is honored only on the legacy direct-create path; hot-pool
// assignment hands over a pre-provisioned, uniform sandbox.
//
// This struct rides the assignment queue verbatim, which is what lets a
// queued challenge request survive to admission — and it is the exact struct
// a future Redis-backed queue must round-trip in full (design §15).
type CreateSessionRequest struct {
	TTLMinutes     int    `json:"ttlMinutes,omitempty"`
	WorkspaceImage string `json:"workspaceImage,omitempty"`
	// ChallengeID selects a guided challenge to seed into the session's
	// vcluster after assignment. Validated against the content catalog
	// (unknown id → 400). starterLabRef is the deprecated alias.
	ChallengeID   string `json:"challengeId,omitempty"`
	StarterLabRef string `json:"starterLabRef,omitempty"`
}

// EffectiveChallengeID resolves challengeId with the deprecated starterLabRef
// alias.
func (r CreateSessionRequest) EffectiveChallengeID() string {
	if r.ChallengeID != "" {
		return r.ChallengeID
	}
	return r.StarterLabRef
}

// ChallengeRef is the session payload's challenge block. The frontend gates
// the terminal + instructions panel on SeedState == "seeded" (same pattern as
// the workspaceReady gate).
type ChallengeRef struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	// SeedState is pending | seeding | seeded | failed.
	SeedState string `json:"seedState"`
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

	// Challenge is present when the session was created for a guided
	// challenge. While seeding, Phase reports the synthetic "Seeding".
	Challenge *ChallengeRef `json:"challenge,omitempty"`
}

// GradeResult is the response of POST /api/sessions/{id}/challenge/grade.
// Steps are independent and ALL evaluated (no short-circuit — the user sees
// everything left to fix); Pass is the conjunction.
type GradeResult struct {
	ChallengeID string      `json:"challengeId"`
	Pass        bool        `json:"pass"`
	Steps       []GradeStep `json:"steps"`
	GradedAt    string      `json:"gradedAt"`
}

// GradeStep is one step's outcome. Message names the object and the observed
// vs expected value on failure — never evaluator internals.
type GradeStep struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Pass        bool   `json:"pass"`
	Message     string `json:"message,omitempty"`
}

// QueueStatus is returned when the warm pool is empty and the request has been
// queued. The client should follow /api/queue/events (SSE) for
// progress and the assigned session.
type QueueStatus struct {
	Status   string `json:"status"` // always "queued"
	Position int    `json:"position"`
	Message  string `json:"message,omitempty"`
}
