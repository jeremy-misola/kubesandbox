package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// SessionClient handles session CRUD operations
type SessionClient struct {
	client    *Client
	namespace string
}

// NewSessionClient creates a new session client
func NewSessionClient(client *Client, namespace string) *SessionClient {
	return &SessionClient{
		client:    client,
		namespace: namespace,
	}
}

// List returns all sessions for a given owner
func (s *SessionClient) List(ctx context.Context, ownerRef string) (*models.SessionListResponse, error) {
	gvr := GetGVR()

	labelSelector := labels.Set{
		"kubesandbox.com/owner": ownerRef,
	}.AsSelector().String()

	list, err := s.client.dynamicClient.Resource(gvr).Namespace(s.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	sessions := make([]models.SessionResponse, 0, len(list.Items))
	for _, item := range list.Items {
		session, err := s.unstructuredToSession(&item)
		if err != nil {
			slog.Warn("Failed to convert session", "error", err)
			continue
		}
		sessions = append(sessions, session.ToResponse())
	}

	return &models.SessionListResponse{
		Items: sessions,
		Total: len(sessions),
	}, nil
}

// Get returns a specific session by name
func (s *SessionClient) Get(ctx context.Context, name string) (*models.KubeSandboxSession, error) {
	gvr := GetGVR()

	obj, err := s.client.dynamicClient.Resource(gvr).Namespace(s.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return s.unstructuredToSession(obj)
}

// Create creates a new session
func (s *SessionClient) Create(ctx context.Context, req *models.CreateSessionRequest, ownerRef string) (*models.KubeSandboxSession, error) {
	gvr := GetGVR()

	// Set defaults
	ttlMinutes := req.TTLMinutes
	if ttlMinutes == 0 {
		ttlMinutes = 60
	}

	workspaceImage := req.WorkspaceImage
	if workspaceImage == "" {
		workspaceImage = "jurassicjey/ttyd-k8s:ttyd"
	}

	resources := req.Resources
	if resources.CPU == "" {
		resources.CPU = "500m"
	}
	if resources.Memory == "" {
		resources.Memory = "512Mi"
	}

	// Create the unstructured object
	session := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.kubesandbox.com/v1alpha1",
			"kind":       "KubeSandboxSession",
			"metadata": map[string]interface{}{
				"name":      req.Name,
				"namespace": s.namespace,
				"labels": map[string]interface{}{
					"kubesandbox.com/owner":  ownerRef,
					"kubesandbox.com/tenant": req.TenantRef,
				},
			},
			"spec": map[string]interface{}{
				"tenantRef":      req.TenantRef,
				"ownerRef":       ownerRef,
				"profile":        req.Profile,
				"ttlMinutes":     ttlMinutes,
				"workspaceImage": workspaceImage,
				"resources": map[string]interface{}{
					"cpu":    resources.CPU,
					"memory": resources.Memory,
				},
			},
		},
	}

	if req.StarterLabRef != "" {
		session.Object["spec"].(map[string]interface{})["starterLabRef"] = req.StarterLabRef
	}

	obj, err := s.client.dynamicClient.Resource(gvr).Namespace(s.namespace).Create(ctx, session, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return s.unstructuredToSession(obj)
}

// Delete removes a session
func (s *SessionClient) Delete(ctx context.Context, name string) error {
	gvr := GetGVR()

	err := s.client.dynamicClient.Resource(gvr).Namespace(s.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// Watch watches for changes to sessions (used for SSE)
func (s *SessionClient) Watch(ctx context.Context, ownerRef string) (<-chan models.KubeSandboxSession, error) {
	gvr := GetGVR()

	labelSelector := labels.Set{
		"kubesandbox.com/owner": ownerRef,
	}.AsSelector().String()

	watcher, err := s.client.dynamicClient.Resource(gvr).Namespace(s.namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to watch sessions: %w", err)
	}

	events := make(chan models.KubeSandboxSession, 100)

	go func() {
		defer close(events)
		defer watcher.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					return
				}

				obj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					continue
				}

				session, err := s.unstructuredToSession(obj)
				if err != nil {
					slog.Warn("Failed to convert watched session", "error", err)
					continue
				}

				select {
				case events <- *session:
				default:
					// Channel full, skip event
				}
			}
		}
	}()

	return events, nil
}

// GetKubeconfig retrieves the vcluster kubeconfig for a session
func (s *SessionClient) GetKubeconfig(ctx context.Context, sessionName string) (*models.KubeconfigResponse, error) {
	// First get the session to find the namespace
	session, err := s.Get(ctx, sessionName)
	if err != nil {
		return nil, err
	}

	sessionNamespace := session.Status.SessionNamespace
	if sessionNamespace == "" {
		return nil, fmt.Errorf("session namespace not yet created")
	}

	// vcluster creates a secret with the kubeconfig
	// Secret name format: vc-{claim-namespace}-{claim-name}-vcluster
	secretName := fmt.Sprintf("vc-%s-%s-vcluster", s.namespace, sessionName)

	secret, err := s.client.clientset.CoreV1().Secrets(sessionNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig secret: %w", err)
	}

	// vcluster stores the kubeconfig in different keys depending on version
	var kubeconfigData []byte
	if val, ok := secret.Data["config"]; ok {
		kubeconfigData = val
	} else if val, ok := secret.Data["kubeconfig"]; ok {
		kubeconfigData = val
	} else {
		// Check for base64 encoded config
		if val, ok := secret.Data["value"]; ok {
			decoded, err := base64.StdEncoding.DecodeString(string(val))
			if err != nil {
				return nil, fmt.Errorf("failed to decode kubeconfig: %w", err)
			}
			kubeconfigData = decoded
		} else {
			return nil, fmt.Errorf("kubeconfig not found in secret")
		}
	}

	server := ""
	if rawConfig, ok := secret.Data["rawConfig"]; ok {
		server = string(rawConfig)
	}

	return &models.KubeconfigResponse{
		Kubeconfig: string(kubeconfigData),
		Server:     server,
	}, nil
}

// unstructuredToSession converts unstructured.Unstructured to KubeSandboxSession
func (s *SessionClient) unstructuredToSession(obj *unstructured.Unstructured) (*models.KubeSandboxSession, error) {
	session := &models.KubeSandboxSession{}

	// Extract metadata
	metadata, ok := obj.Object["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid metadata")
	}

	session.Name, _ = metadata["name"].(string)
	session.Namespace, _ = metadata["namespace"].(string)

	if creationTimestamp, ok := metadata["creationTimestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, creationTimestamp); err == nil {
			session.CreationTimestamp = t
		}
	}

	// Extract spec
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if ok {
		session.Spec.TenantRef, _ = spec["tenantRef"].(string)
		session.Spec.OwnerRef, _ = spec["ownerRef"].(string)
		session.Spec.Profile, _ = spec["profile"].(string)
		session.Spec.TTLMinutes, _ = spec["ttlMinutes"].(int)
		session.Spec.WorkspaceImage, _ = spec["workspaceImage"].(string)
		session.Spec.StarterLabRef, _ = spec["starterLabRef"].(string)

		if resources, ok := spec["resources"].(map[string]interface{}); ok {
			session.Spec.Resources.CPU, _ = resources["cpu"].(string)
			session.Spec.Resources.Memory, _ = resources["memory"].(string)
		}
	}

	// Extract status
	status, ok := obj.Object["status"].(map[string]interface{})
	if ok {
		session.Status.Phase, _ = status["phase"].(string)
		session.Status.Message, _ = status["message"].(string)
		session.Status.ExpiresAt, _ = status["expiresAt"].(string)
		session.Status.SessionNamespace, _ = status["sessionNamespace"].(string)
		session.Status.VclusterRelease, _ = status["vclusterRelease"].(string)
		session.Status.WorkspacePod, _ = status["workspacePod"].(string)
		session.Status.WorkspaceReady, _ = status["workspaceReady"].(bool)
	}

	return session, nil
}

// CleanupExpiredSessions removes sessions that have exceeded their TTL
func (s *SessionClient) CleanupExpiredSessions(ctx context.Context) error {
	gvr := GetGVR()

	list, err := s.client.dynamicClient.Resource(gvr).Namespace(s.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list sessions for cleanup: %w", err)
	}

	now := time.Now()
	for _, item := range list.Items {
		session, err := s.unstructuredToSession(&item)
		if err != nil {
			slog.Warn("Failed to convert session during cleanup", "error", err)
			continue
		}

		// Check if session has expired based on TTL
		if session.Status.ExpiresAt != "" {
			expiresAt, err := time.Parse(time.RFC3339, session.Status.ExpiresAt)
			if err != nil {
				slog.Warn("Failed to parse expiresAt", "error", err, "session", session.Name)
				continue
			}

			if now.After(expiresAt) {
				slog.Info("Deleting expired session", "session", session.Name, "expiredAt", expiresAt)
				if err := s.Delete(ctx, session.Name); err != nil {
					slog.Error("Failed to delete expired session", "error", err, "session", session.Name)
				}
			}
		} else {
			// Calculate expiry from TTL and creation time
			ttlMinutes := session.Spec.TTLMinutes
			if ttlMinutes == 0 {
				ttlMinutes = 60
			}

			expiresAt := session.CreationTimestamp.Add(time.Duration(ttlMinutes) * time.Minute)
			if now.After(expiresAt) {
				slog.Info("Deleting expired session (TTL based)", "session", session.Name, "ttl", ttlMinutes)
				if err := s.Delete(ctx, session.Name); err != nil {
					slog.Error("Failed to delete expired session", "error", err, "session", session.Name)
				}
			}
		}
	}

	return nil
}

// StartTTLCleanup starts a background goroutine to cleanup expired sessions
func (s *SessionClient) StartTTLCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.CleanupExpiredSessions(ctx); err != nil {
					slog.Error("Failed to cleanup expired sessions", "error", err)
				}
			}
		}
	}()
}

// Ensure corev1 is used for linters
var _ = corev1.Pod{}
