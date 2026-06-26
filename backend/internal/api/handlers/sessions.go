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
	svc *k8s.SessionService
}

// NewSessionHandler constructs a SessionHandler.
func NewSessionHandler(svc *k8s.SessionService) *SessionHandler {
	return &SessionHandler{svc: svc}
}

// Create handles POST /api/sessions.
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
	if !req.Profile.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_profile",
			"message": "profile must be one of: starter, standard, advanced",
		})
		return
	}

	sess, err := h.svc.Create(c.Request.Context(), ident.Subject, req)
	if err != nil {
		switch {
		case errors.Is(err, k8s.ErrQuotaExceeded):
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "quota_exceeded",
				"message": "you have reached the maximum number of concurrent sessions",
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
