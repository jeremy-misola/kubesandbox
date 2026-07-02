package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// SessionHandler serves the /api/sessions endpoints.
type SessionHandler struct {
	svc         *k8s.SessionService
	queue       *k8s.AssignQueue
	poolEnabled bool
}

// NewSessionHandler constructs a SessionHandler. queue may be nil when the
// warm pool is disabled (legacy direct-create mode).
func NewSessionHandler(svc *k8s.SessionService, queue *k8s.AssignQueue, poolEnabled bool) *SessionHandler {
	return &SessionHandler{svc: svc, queue: queue, poolEnabled: poolEnabled}
}

// Create handles POST /api/sessions.
//
// Hot-pool path (default): assign an already-running warm sandbox — a
// metadata change, sub-second. If the pool is empty the request is QUEUED
// (202) with progress on GET /api/queue/events; it is never put on a
// synchronous cold build (docs/08 §2.2).
func (h *SessionHandler) Create(c *gin.Context) {
	ident := middleware.GetIdentity(c)

	var req models.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "request body is not valid JSON",
		})
		return
	}

	if !h.poolEnabled {
		h.createDirect(c, ident.Subject, req)
		return
	}

	sess, err := h.svc.Assign(c.Request.Context(), ident.Subject, req)
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, sess)
	case errors.Is(err, k8s.ErrAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "session_exists",
			"message": "you already have a sandbox (or one is still being cleaned up); delete it before creating a new one",
		})
	case errors.Is(err, k8s.ErrPoolEmpty):
		pos := h.queue.Enqueue(ident.Subject, req)
		c.JSON(http.StatusAccepted, models.QueueStatus{
			Status:   "queued",
			Position: pos,
			Message:  "all sandboxes are in use; you're in line — follow /api/queue/events for progress",
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "create_failed",
			"message": "could not create session",
		})
	}
}

// createDirect is the legacy pool-disabled path (cold build on request).
func (h *SessionHandler) createDirect(c *gin.Context, owner string, req models.CreateSessionRequest) {
	sess, err := h.svc.Create(c.Request.Context(), owner, req)
	if err != nil {
		switch {
		case errors.Is(err, k8s.ErrAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{
				"error":   "session_exists",
				"message": "you already have a sandbox (or one is still being cleaned up); delete it before creating a new one",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "create_failed",
				"message": "could not create session",
			})
		}
		return
	}
	c.JSON(http.StatusCreated, sess)
}

// QueuePosition handles GET /api/queue — JSON poll of the caller's queue state.
func (h *SessionHandler) QueuePosition(c *gin.Context) {
	ident := middleware.GetIdentity(c)
	if h.queue == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_queued", "message": "queueing is disabled"})
		return
	}
	if pos, ok := h.queue.Position(ident.Subject); ok {
		c.JSON(http.StatusOK, models.QueueStatus{Status: "queued", Position: pos})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not_queued", "message": "you are not in the queue"})
}

// List handles GET /api/sessions.
func (h *SessionHandler) List(c *gin.Context) {
	ident := middleware.GetIdentity(c)
	sessions, err := h.svc.List(c.Request.Context(), ident.Subject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "list_failed",
			"message": "could not list sessions",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// Get handles GET /api/sessions/:id.
func (h *SessionHandler) Get(c *gin.Context) {
	ident := middleware.GetIdentity(c)
	sess, err := h.svc.Get(c.Request.Context(), c.Param("id"), ident.Subject)
	if err != nil {
		respondLookupError(c, err)
		return
	}
	c.JSON(http.StatusOK, sess)
}

// Delete handles DELETE /api/sessions/:id.
func (h *SessionHandler) Delete(c *gin.Context) {
	ident := middleware.GetIdentity(c)
	if err := h.svc.Delete(c.Request.Context(), c.Param("id"), ident.Subject); err != nil {
		respondLookupError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// respondLookupError maps service lookup errors to HTTP responses. Unknown,
// unowned, and malformed ids all return 404 to avoid leaking existence.
func respondLookupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, k8s.ErrNotFound), errors.Is(err, k8s.ErrInvalidID):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": "session not found",
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "unexpected error",
		})
	}
}
