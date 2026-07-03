// Command server is the KubeSandbox backend control service.
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
	"github.com/jeremy-misola/kubesandbox/backend/internal/telemetry"
)

func main() {
	cfg := config.Load()

	// OTel metrics (docs/reference/observability-architecture.md): OTLP push to
	// the node-local collector agent, env-driven. metrics is nil (no-op) when
	// OTEL_SDK_DISABLED=true or no endpoint is configured.
	metrics, telemetryShutdown, err := telemetry.Setup(context.Background())
	if err != nil {
		log.Fatalf("telemetry: %v", err)
	}
	if metrics != nil {
		log.Printf("telemetry: OTLP metrics enabled (endpoint=%s)", os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}

	client, err := k8s.NewDynamicClient()
	if err != nil {
		log.Fatalf("kubernetes client: %v", err)
	}

	svc := k8s.NewSessionService(
		client,
		cfg.Namespace,
		cfg.PublicBaseURL,
		models.DefaultWorkspaceImage,
	)
	svc.SetMetrics(metrics)

	queue := k8s.NewAssignQueue()
	queue.SetMetrics(metrics)
	router := api.NewRouter(cfg, svc, queue, metrics)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		// No write timeout: SSE responses are long-lived.
	}

	// Background TTL cleanup loop; cancelled on shutdown.
	bgCtx, bgCancel := context.WithCancel(context.Background())
	ttl := k8s.NewTTLController(svc, cfg.TTLCleanupInterval)
	ttl.SetMetrics(metrics)
	go ttl.Run(bgCtx)

	// Warm-pool manager: keeps N unclaimed sandboxes Ready so creates are a
	// metadata-only assignment. Strictly off the request path.
	if cfg.PoolEnabled {
		pool := k8s.NewPoolManager(svc, queue, k8s.PoolConfig{
			TargetWarm: cfg.PoolTargetWarm,
			MaxTotal:   cfg.PoolMaxTotal,
			MaxWarmAge: cfg.PoolMaxWarmAge,
			Resync:     cfg.PoolResync,
		})
		pool.SetMetrics(metrics)
		go pool.Run(bgCtx)
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
	bgCancel() // stop the TTL loop
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	// Flush buffered metrics on a FRESH deadline. srv.Shutdown blocks until
	// in-flight requests drain, and long-lived SSE streams have no write
	// timeout, so the 15s budget above can be fully consumed — reusing it here
	// would leave the OTLP exporter no time to flush over the network.
	flushCtx, flushCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer flushCancel()
	if err := telemetryShutdown(flushCtx); err != nil {
		log.Printf("telemetry shutdown: %v", err)
	}
}
