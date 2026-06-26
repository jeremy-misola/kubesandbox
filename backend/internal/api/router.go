// Package api wires the HTTP routes for the backend.
package api

import (
	"github.com/gin-gonic/gin"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/handlers"
	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
)

// NewRouter builds the Gin engine.
//
// Routing note: the Envoy HTTPRoute forwards the "/api" prefix unchanged (no URL
// rewrite filter), so the API lives under /api. Health probes are served at the
// root because the kubelet hits the pod directly, bypassing the gateway.
func NewRouter(cfg config.Config, svc *k8s.SessionService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Unauthenticated probes (reached directly by the kubelet).
	r.GET("/health", handlers.Health)
	r.GET("/healthz", handlers.Health)

	sessions := handlers.NewSessionHandler(svc)

	api := r.Group("/api")
	api.Use(middleware.IdentityMiddleware(cfg))
	{
		api.GET("/health", handlers.Health)
		api.POST("/sessions", sessions.Create)
		api.GET("/sessions", sessions.List)
		api.GET("/sessions/:id", sessions.Get)
		api.DELETE("/sessions/:id", sessions.Delete)
		api.GET("/sessions/:id/events", sessions.Events)
	}

	// Ext-authz (ForwardAuth) endpoint for the per-session SecurityPolicy (G2).
	// Mounted outside /api: the session route's SecurityPolicy points its ext-authz
	// backendRef here. IdentityMiddleware enforces the same X-User-* identity
	// contract (missing identity -> 401). Both /authz and /authz/<original-path>
	// are accepted so the policy can either forward the URI via header or send the
	// original path directly.
	authz := handlers.NewAuthzHandler(svc)
	r.GET("/authz", middleware.IdentityMiddleware(cfg), authz.Check)
	r.GET("/authz/*rest", middleware.IdentityMiddleware(cfg), authz.Check)

	return r
}
