package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
)

// respondError writes a uniform {error, message} JSON body with the given
// status.
func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": code, "message": message})
}

// respondLookupError maps service lookup errors to HTTP responses. Unknown,
// unowned, and malformed ids all return 404 to avoid leaking existence.
func respondLookupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, k8s.ErrNotFound), errors.Is(err, k8s.ErrInvalidID):
		respondError(c, http.StatusNotFound, "not_found", "session not found")
	default:
		respondError(c, http.StatusInternalServerError, "internal_error", "unexpected error")
	}
}
