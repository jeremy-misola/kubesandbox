// Package handlers implements the backend HTTP handlers.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health is a liveness/readiness probe handler. It is served at the root
// (/health, /healthz) so the kubelet can reach it directly on the pod, bypassing
// the gateway and identity middleware.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
