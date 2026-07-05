// Package api wires the HTTP routes for the backend.
package api

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/handlers"
	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
	"github.com/jeremy-misola/kubesandbox/backend/internal/content"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/telemetry"
)

// Routes:
//
//	GET  /health, /healthz  — unauthenticated kubelet probes at root.
//	/api/*                  — identity-guarded session control API.
//	GET  /authz, /authz/*   — ext-authz endpoint: reads the session cookie; no
//	                          valid cookie → redirect to Authentik; valid →
//	                          ownership check.
//	GET  /oauth2/callback   — OIDC callback: exchange code, set session cookie,
//	                          redirect to original URL. No auth.
//
// NewRouter builds the Gin engine. catalog and challengeHandler are nil when
// challenges are disabled: the /api/challenges surface is then absent and
// challengeId on create is rejected.
func NewRouter(cfg config.Config, svc *k8s.SessionService, queue *k8s.AssignQueue, catalog content.Store, challengeHandler *handlers.ChallengeHandler, metrics *telemetry.Metrics) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	// HTTP server metrics (http.server.request.duration, labelled by templated
	// http.route). Reads the global providers installed by telemetry.Setup;
	// when telemetry is disabled the globals are no-ops.
	r.Use(otelgin.Middleware("kubesandbox-backend"))

	r.GET("/health", handlers.Health)
	r.GET("/healthz", handlers.Health)

	sessions := handlers.NewSessionHandler(svc, queue, catalog, metrics)

	api := r.Group("/api")
	api.Use(middleware.IdentityMiddleware(cfg))
	{
		api.GET("/health", handlers.Health)
		api.POST("/sessions", sessions.Create)
		api.GET("/sessions", sessions.List)
		api.GET("/sessions/:id", sessions.Get)
		api.DELETE("/sessions/:id", sessions.Delete)
		api.GET("/sessions/:id/events", sessions.Events)
		api.GET("/queue", sessions.QueuePosition)
		api.GET("/queue/events", sessions.QueueEvents)

		// Guided challenges (design §8).
		if challengeHandler != nil {
			api.GET("/challenges", challengeHandler.List)
			api.GET("/challenges/:id", challengeHandler.Get)
			api.POST("/sessions/:id/challenge/grade", challengeHandler.Grade)
			api.POST("/sessions/:id/challenge/reset", challengeHandler.Reset)
		}
	}

	// Ext-authz endpoint. No IdentityMiddleware: identity comes from the session
	// cookie, not X-User-* headers.
	authz := handlers.NewAuthzHandler(svc, cfg)
	r.GET("/authz", authz.Check)
	r.GET("/authz/*rest", authz.Check)

	callback := handlers.NewAuthCallbackHandler(cfg)
	r.GET("/oauth2/callback", callback.Callback)

	return r
}
