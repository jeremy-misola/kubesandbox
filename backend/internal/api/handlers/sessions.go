package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	"github.com/jeremy-misola/kubesandbox/backend/internal/telemetry"
)

const msgSessionExists = "you already have a sandbox (or one is still being cleaned up); delete it before creating a new one"

// SessionHandler serves the /api/sessions endpoints.
type SessionHandler struct {
	svc   *k8s.SessionService
	queue *k8s.AssignQueue
	// metrics is the injected instrument set; nil is a valid no-op.
	metrics *telemetry.Metrics
}

// NewSessionHandler constructs a SessionHandler. metrics may be nil when
// telemetry is disabled.
func NewSessionHandler(svc *k8s.SessionService, queue *k8s.AssignQueue, metrics *telemetry.Metrics) *SessionHandler {
	return &SessionHandler{svc: svc, queue: queue, metrics: metrics}
}

// Create handles POST /api/sessions. It assigns an already-warm sandbox (a
// metadata change); an empty pool queues the request (202) rather than running
// a synchronous cold build.
func (h *SessionHandler) Create(c *gin.Context) {
	ident := middleware.GetIdentity(c)

	var req models.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "request body is not valid JSON")
		return
	}

	sess, err := h.svc.Assign(c.Request.Context(), ident.Subject, req)
	switch {
	case err == nil:
		// source=request distinguishes direct hand-outs from queue admissions
		// (recorded by the pool manager); both paths share Assign.
		h.metrics.RecordClaimed(c.Request.Context(), telemetry.SourceRequest)
		c.JSON(http.StatusCreated, sess)
	case errors.Is(err, k8s.ErrAlreadyExists):
		respondError(c, http.StatusConflict, "session_exists", msgSessionExists)
	case errors.Is(err, k8s.ErrPoolEmpty):
		pos := h.queue.Enqueue(ident.Subject, req)
		c.JSON(http.StatusAccepted, models.QueueStatus{
			Status:   "queued",
			Position: pos,
			Message:  "all sandboxes are in use; you're in line — follow /api/queue/events for progress",
		})
	default:
		respondError(c, http.StatusInternalServerError, "create_failed", "could not create session")
	}
}

// QueuePosition handles GET /api/queue — a JSON poll of the caller's queue state.
func (h *SessionHandler) QueuePosition(c *gin.Context) {
	ident := middleware.GetIdentity(c)
	if pos, ok := h.queue.Position(ident.Subject); ok {
		c.JSON(http.StatusOK, models.QueueStatus{Status: "queued", Position: pos})
		return
	}
	respondError(c, http.StatusNotFound, "not_queued", "you are not in the queue")
}

// List handles GET /api/sessions.
func (h *SessionHandler) List(c *gin.Context) {
	ident := middleware.GetIdentity(c)
	sessions, err := h.svc.List(c.Request.Context(), ident.Subject)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed", "could not list sessions")
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
