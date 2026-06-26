// Command server is the KubeSandbox backend control service (G1).
//
// It exposes a small HTTP API for creating, listing, reading and deleting
// KubeSandboxSession claims. Identity is taken from Envoy-forwarded X-User-*
// headers; claims are the source of truth (no application database).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api"
	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

func main() {
	cfg := config.Load()

	client, err := k8s.NewDynamicClient()
	if err != nil {
		log.Fatalf("kubernetes client: %v", err)
	}

	svc := k8s.NewSessionService(
		client,
		cfg.Namespace,
		cfg.PublicBaseURL,
		cfg.MaxSessionsPerUser,
		models.DefaultWorkspaceImage,
	)

	router := api.NewRouter(cfg, svc)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		// No write timeout: SSE responses are long-lived.
	}

	// Run the server.
	go func() {
		log.Printf("kubesandbox-backend listening on :%s (namespace=%s)", cfg.Port, cfg.Namespace)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
