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
// Route summary:
//
//	GET  /health, /healthz         — unauthenticated kubelet probes (at root,
//	                                  bypassing the gateway).
//	/api/*                         — JWT-guarded session control API (G1/G4).
//	GET  /authz, /authz/*          — ext-authz ForwardAuth endpoint (G2 Option B):
//	                                  reads the session cookie; no valid cookie →
//	                                  redirect to Authentik; valid → ownership check.
//	GET  /oauth2/callback          — OIDC callback: exchange code, set session
//	                                  cookie, redirect to original URL. No auth.
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

	// Ext-authz (ForwardAuth) endpoint for the per-session SecurityPolicy (G2
	// Option B). No IdentityMiddleware: identity comes from the session cookie,
	// not from X-User-* headers. The handler either redirects to Authentik (no
	// valid cookie) or checks ownership and returns 200/403/503.
	authz := handlers.NewAuthzHandler(svc, cfg)
	r.GET("/authz", authz.Check)
	r.GET("/authz/*rest", authz.Check)

	// OIDC callback: receives ?code=...&state=... from Authentik after login.
	// Sets the session cookie and redirects back to the original /s/{id}/... URL.
	// No authentication required — this is the login completion endpoint.
	callback := handlers.NewAuthCallbackHandler(cfg)
	r.GET("/oauth2/callback", callback.Callback)

	return r
}
