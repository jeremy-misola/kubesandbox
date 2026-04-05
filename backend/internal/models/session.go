package models

import (
	"time"
)

// Profile enum values
const (
	ProfileStarter  = "starter"
	ProfileStandard = "standard"
	ProfileAdvanced = "advanced"
)

// Phase values
const (
	PhaseProvisioning = "Provisioning"
	PhaseReady        = "Ready"
	PhaseError        = "Error"
	PhaseUnknown      = "Unknown"
)

// SessionSpec defines the desired state of KubeSandboxSession
type SessionSpec struct {
	TenantRef      string           `json:"tenantRef"`
	OwnerRef       string           `json:"ownerRef"`
	Profile        string           `json:"profile"`
	TTLMinutes     int              `json:"ttlMinutes,omitempty"`
	WorkspaceImage string           `json:"workspaceImage,omitempty"`
	StarterLabRef  string           `json:"starterLabRef,omitempty"`
	Resources      SessionResources `json:"resources,omitempty"`
}

// SessionResources defines resource limits for the session
type SessionResources struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// SessionStatus defines the observed state of KubeSandboxSession
type SessionStatus struct {
	Phase            string `json:"phase,omitempty"`
	Message          string `json:"message,omitempty"`
	ExpiresAt        string `json:"expiresAt,omitempty"`
	SessionNamespace string `json:"sessionNamespace,omitempty"`
	VclusterRelease  string `json:"vclusterRelease,omitempty"`
	WorkspacePod     string `json:"workspacePod,omitempty"`
	WorkspaceReady   bool   `json:"workspaceReady,omitempty"`
}

// KubeSandboxSession is the Schema for the kubesandboxsessions API
type KubeSandboxSession struct {
	// APIVersion defines the versioned schema of this representation
	APIVersion string `json:"apiVersion,omitempty"`
	// Kind is a string value representing the REST resource
	Kind string `json:"kind,omitempty"`

	// Metadata
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp time.Time         `json:"creationTimestamp,omitempty"`

	Spec   SessionSpec   `json:"spec,omitempty"`
	Status SessionStatus `json:"status,omitempty"`
}

// KubeSandboxSessionList contains a list of KubeSandboxSession
type KubeSandboxSessionList struct {
	APIVersion string               `json:"apiVersion,omitempty"`
	Kind       string               `json:"kind,omitempty"`
	Items      []KubeSandboxSession `json:"items"`
}

// CreateSessionRequest is the request body for creating a session
type CreateSessionRequest struct {
	Name           string           `json:"name" validate:"required,dns1035"`
	TenantRef      string           `json:"tenantRef" validate:"required"`
	Profile        string           `json:"profile" validate:"required,oneof=starter standard advanced"`
	TTLMinutes     int              `json:"ttlMinutes,omitempty" validate:"omitempty,min=15,max=1440"`
	WorkspaceImage string           `json:"workspaceImage,omitempty" validate:"omitempty,max=255"`
	StarterLabRef  string           `json:"starterLabRef,omitempty" validate:"omitempty,max=128"`
	Resources      SessionResources `json:"resources,omitempty"`
}

// SessionResponse is the response for session operations
type SessionResponse struct {
	Name             string           `json:"name"`
	Namespace        string           `json:"namespace"`
	Phase            string           `json:"phase"`
	Message          string           `json:"message,omitempty"`
	ExpiresAt        string           `json:"expiresAt,omitempty"`
	SessionNamespace string           `json:"sessionNamespace,omitempty"`
	VclusterRelease  string           `json:"vclusterRelease,omitempty"`
	WorkspacePod     string           `json:"workspacePod,omitempty"`
	WorkspaceReady   bool             `json:"workspaceReady"`
	TenantRef        string           `json:"tenantRef"`
	OwnerRef         string           `json:"ownerRef"`
	Profile          string           `json:"profile"`
	TTLMinutes       int              `json:"ttlMinutes"`
	WorkspaceImage   string           `json:"workspaceImage"`
	Resources        SessionResources `json:"resources"`
	CreatedAt        string           `json:"createdAt"`
}

// SessionListResponse is the response for listing sessions
type SessionListResponse struct {
	Items []SessionResponse `json:"items"`
	Total int               `json:"total"`
}

// KubeconfigResponse is the response for kubeconfig download
type KubeconfigResponse struct {
	Kubeconfig string `json:"kubeconfig"`
	Server     string `json:"server"`
}

// ToResponse converts KubeSandboxSession to SessionResponse
func (s *KubeSandboxSession) ToResponse() SessionResponse {
	return SessionResponse{
		Name:             s.Name,
		Namespace:        s.Namespace,
		Phase:            s.Status.Phase,
		Message:          s.Status.Message,
		ExpiresAt:        s.Status.ExpiresAt,
		SessionNamespace: s.Status.SessionNamespace,
		VclusterRelease:  s.Status.VclusterRelease,
		WorkspacePod:     s.Status.WorkspacePod,
		WorkspaceReady:   s.Status.WorkspaceReady,
		TenantRef:        s.Spec.TenantRef,
		OwnerRef:         s.Spec.OwnerRef,
		Profile:          s.Spec.Profile,
		TTLMinutes:       s.Spec.TTLMinutes,
		WorkspaceImage:   s.Spec.WorkspaceImage,
		Resources:        s.Spec.Resources,
		CreatedAt:        s.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}
}
