package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
	"github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	"github.com/jeremy-misola/kubesandbox/backend/pkg/apierror"
)

// SessionHandler handles session API requests
type SessionHandler struct {
	sessionClient *kubernetes.SessionClient
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(sessionClient *kubernetes.SessionClient) *SessionHandler {
	return &SessionHandler{
		sessionClient: sessionClient,
	}
}

// ListSessions returns all sessions for the authenticated user
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		apierror.WriteError(w, apierror.ErrUnauthorized)
		return
	}

	sessions, err := h.sessionClient.List(r.Context(), userID)
	if err != nil {
		apierror.WriteError(w, apierror.NewInternal(err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, sessions)
}

// GetSession returns a specific session
func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	name := getPathParam(r, "name")
	if name == "" {
		apierror.WriteError(w, apierror.NewBadRequest("session name is required"))
		return
	}

	session, err := h.sessionClient.Get(r.Context(), name)
	if err != nil {
		apierror.WriteError(w, apierror.NewNotFound("session", name))
		return
	}

	// Verify ownership
	userID := middleware.GetUserID(r)
	if session.Spec.OwnerRef != userID {
		apierror.WriteError(w, apierror.ErrForbidden)
		return
	}

	writeJSON(w, http.StatusOK, session.ToResponse())
}

// CreateSession creates a new session
func (h *SessionHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req models.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.WriteError(w, apierror.NewBadRequest("invalid request body"))
		return
	}

	// Validate request
	if req.Name == "" {
		apierror.WriteError(w, apierror.NewBadRequest("name is required"))
		return
	}
	if req.TenantRef == "" {
		apierror.WriteError(w, apierror.NewBadRequest("tenantRef is required"))
		return
	}
	if req.Profile == "" {
		apierror.WriteError(w, apierror.NewBadRequest("profile is required"))
		return
	}

	userID := middleware.GetUserID(r)
	if userID == "" {
		apierror.WriteError(w, apierror.ErrUnauthorized)
		return
	}

	session, err := h.sessionClient.Create(r.Context(), &req, userID)
	if err != nil {
		apierror.WriteError(w, apierror.NewInternal(err.Error()))
		return
	}

	writeJSON(w, http.StatusCreated, session.ToResponse())
}

// DeleteSession deletes a session
func (h *SessionHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	name := getPathParam(r, "name")
	if name == "" {
		apierror.WriteError(w, apierror.NewBadRequest("session name is required"))
		return
	}

	// Verify ownership first
	session, err := h.sessionClient.Get(r.Context(), name)
	if err != nil {
		apierror.WriteError(w, apierror.NewNotFound("session", name))
		return
	}

	userID := middleware.GetUserID(r)
	if session.Spec.OwnerRef != userID {
		apierror.WriteError(w, apierror.ErrForbidden)
		return
	}

	if err := h.sessionClient.Delete(r.Context(), name); err != nil {
		apierror.WriteError(w, apierror.NewInternal(err.Error()))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetKubeconfig returns the vcluster kubeconfig for a session
func (h *SessionHandler) GetKubeconfig(w http.ResponseWriter, r *http.Request) {
	name := getPathParam(r, "name")
	if name == "" {
		apierror.WriteError(w, apierror.NewBadRequest("session name is required"))
		return
	}

	// Verify ownership
	session, err := h.sessionClient.Get(r.Context(), name)
	if err != nil {
		apierror.WriteError(w, apierror.NewNotFound("session", name))
		return
	}

	userID := middleware.GetUserID(r)
	if session.Spec.OwnerRef != userID {
		apierror.WriteError(w, apierror.ErrForbidden)
		return
	}

	kubeconfig, err := h.sessionClient.GetKubeconfig(r.Context(), name)
	if err != nil {
		apierror.WriteError(w, apierror.NewInternal(err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, kubeconfig)
}

// SessionEvents handles SSE for session status updates
func (h *SessionHandler) SessionEvents(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		apierror.WriteError(w, apierror.ErrUnauthorized)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Watch for session changes
	events, err := h.sessionClient.Watch(r.Context(), userID)
	if err != nil {
		apierror.WriteError(w, apierror.NewInternal(err.Error()))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		apierror.WriteError(w, apierror.NewInternal("streaming not supported"))
		return
	}

	// Send initial connection message
	fmt.Fprintf(w, "event: connected\ndata: {\"message\":\"Connected to session events\"}\n\n")
	flusher.Flush()

	// Stream events
	for {
		select {
		case <-r.Context().Done():
			return
		case session, ok := <-events:
			if !ok {
				return
			}

			data, err := json.Marshal(session.ToResponse())
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "event: session\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// getPathParam extracts a path parameter from the request
func getPathParam(r *http.Request, name string) string {
	return r.PathValue(name)
}
