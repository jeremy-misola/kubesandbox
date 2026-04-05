package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/handlers"
	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
	"github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
)

func main() {
	// Configure logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Load configuration
	cfg := config.Load()
	slog.Info("Starting KubeSandbox backend", "port", cfg.Port, "namespace", cfg.Namespace)

	// Initialize Kubernetes client
	k8sClient, err := kubernetes.NewClient()
	if err != nil {
		slog.Error("Failed to create Kubernetes client", "error", err)
		os.Exit(1)
	}

	// Verify Kubernetes connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := k8sClient.Ping(ctx); err != nil {
		slog.Error("Failed to connect to Kubernetes", "error", err)
		os.Exit(1)
	}
	slog.Info("Connected to Kubernetes cluster")

	// Create session client
	sessionClient := kubernetes.NewSessionClient(k8sClient, cfg.Namespace)

	// Start TTL cleanup worker
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	defer cleanupCancel()
	sessionClient.StartTTLCleanup(cleanupCtx, time.Duration(cfg.TTLCleanupInterval)*time.Minute)
	slog.Info("Started TTL cleanup worker", "interval_minutes", cfg.TTLCleanupInterval)

	// Create handlers
	sessionHandler := handlers.NewSessionHandler(sessionClient)

	// Build router
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/v1/sessions", sessionHandler.ListSessions)
	mux.HandleFunc("POST /api/v1/sessions", sessionHandler.CreateSession)
	mux.HandleFunc("GET /api/v1/sessions/{name}", sessionHandler.GetSession)
	mux.HandleFunc("DELETE /api/v1/sessions/{name}", sessionHandler.DeleteSession)
	mux.HandleFunc("GET /api/v1/sessions/{name}/kubeconfig", sessionHandler.GetKubeconfig)
	mux.HandleFunc("GET /api/v1/sessions/events", sessionHandler.SessionEvents)
	mux.HandleFunc("GET /api/v1/user", handlers.GetCurrentUser)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Apply middleware
	handler := middleware.Logging(middleware.AllowAnyOrigin(middleware.Auth(cfg)(mux)))

	// Start server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("Server starting", "port", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
