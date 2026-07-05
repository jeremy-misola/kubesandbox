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
	"github.com/jeremy-misola/kubesandbox/backend/internal/api/handlers"
	"github.com/jeremy-misola/kubesandbox/backend/internal/challenges"
	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
	"github.com/jeremy-misola/kubesandbox/backend/internal/content"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	"github.com/jeremy-misola/kubesandbox/backend/internal/redisclient"
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
		cfg.WorkspaceImage,
	)
	svc.SetMetrics(metrics)

	// Background loops; cancelled on shutdown.
	bgCtx, bgCancel := context.WithCancel(context.Background())

	// Assign queue: Redis-backed shared state when REDIS_ADDR is set (required
	// for >1 replica), else the in-memory single-replica queue
	// (docs/redis-queue-horizontal-scaling.md).
	var queue k8s.Queue
	if cfg.RedisAddr != "" {
		rc := redisclient.New(redisclient.Options{
			Addr:        cfg.RedisAddr,
			Password:    cfg.RedisPassword,
			DB:          cfg.RedisDB,
			KeyPrefix:   cfg.RedisKeyPrefix,
			DialTimeout: cfg.RedisDialTimeout,
		})
		if err := rc.Healthy(bgCtx); err != nil {
			// Non-fatal: the queue degrades to reject-new-enqueues until Redis
			// is reachable; direct assignment and pool healing are unaffected.
			log.Printf("queue: redis %s not reachable yet: %v", cfg.RedisAddr, err)
		}
		rq := k8s.NewRedisQueue(rc, k8s.RedisQueueConfig{MaxWait: cfg.QueueMaxWait})
		rq.SetOwnedSessionLookup(func(ctx context.Context, owner string) *models.Session {
			sessions, err := svc.List(ctx, owner)
			if err != nil || len(sessions) == 0 {
				return nil
			}
			return &sessions[0]
		})
		go rq.Run(bgCtx)
		queue = rq
		log.Printf("queue: redis-backed (addr=%s prefix=%s maxWait=%s)",
			cfg.RedisAddr, cfg.RedisKeyPrefix, cfg.QueueMaxWait)
	} else {
		queue = k8s.NewAssignQueue()
		log.Printf("queue: in-memory (single-replica mode; set REDIS_ADDR to scale out)")
	}
	queue.SetMetrics(metrics)

	// Guided challenges (docs/history/challenges-backend-architecture.md):
	// content store (ConfigMaps via GitOps), tenant-client factory, async
	// seeder and on-demand grader. All strictly off the request path.
	var (
		catalog          content.Store
		challengeHandler *handlers.ChallengeHandler
	)
	if cfg.ChallengesEnabled {
		store := content.NewConfigMapStore(client, cfg.Namespace, cfg.ChallengeContentResync)
		store.SetMetrics(metrics)
		// Synchronous initial fill so create-with-challengeId validates
		// correctly from the very first request after startup.
		if err := store.RebuildOnce(bgCtx); err != nil {
			log.Printf("challenges: initial catalog build failed (will retry via watch/resync): %v", err)
		}
		go store.Run(bgCtx)
		catalog = store

		factory := k8s.NewTenantClientFactory(client, cfg.Namespace, 32)
		tenantOps := challenges.NewTenantOps(factory, metrics)

		seeder := challenges.NewSeeder(svc, store, tenantOps, challenges.SeederConfig{
			Budget:      cfg.ChallengeSeedTimeout,
			ResetBudget: cfg.ChallengeResetTimeout,
			MaxAttempts: cfg.ChallengeSeedMaxAttempts,
			Backoff:     cfg.ChallengeSeedBackoff,
			Resync:      cfg.ChallengeSeedResync,
			Workers:     cfg.ChallengeSeedWorkers,
		})
		seeder.SetMetrics(metrics)
		go seeder.Run(bgCtx)

		grader := challenges.NewGrader(store, tenantOps)
		grader.SetMetrics(metrics)

		// Session-service hooks: fast-path seed notification, challenge
		// titles on session payloads, tenant-cache invalidation on deletes.
		svc.SetSeedNotifier(seeder.Enqueue)
		svc.SetChallengeTitleLookup(func(id string) (string, bool) {
			if b, ok := store.Get(id); ok {
				return b.Title, true
			}
			return "", false
		})
		svc.SetClaimDeletedHook(factory.Invalidate)

		challengeHandler = handlers.NewChallengeHandler(store, svc, grader, seeder, cfg.ChallengeGradeMinInterval)
		log.Printf("challenges: enabled (namespace=%s seedTimeout=%s)", cfg.Namespace, cfg.ChallengeSeedTimeout)
	}

	router := api.NewRouter(cfg, svc, queue, catalog, challengeHandler, metrics)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		// No write timeout: SSE responses are long-lived.
	}

	ttl := k8s.NewTTLController(svc, cfg.TTLCleanupInterval)
	ttl.SetMetrics(metrics)
	go ttl.Run(bgCtx)

	// Warm-pool manager: keeps N unclaimed sandboxes Ready so creates are a
	// metadata-only assignment. Strictly off the request path.
	pool := k8s.NewPoolManager(svc, queue, k8s.PoolConfig{
		TargetWarm: cfg.PoolTargetWarm,
		MaxTotal:   cfg.PoolMaxTotal,
		MaxWarmAge: cfg.PoolMaxWarmAge,
		Resync:     cfg.PoolResync,
	})
	pool.SetMetrics(metrics)
	if cfg.LeaderElection {
		// Exactly one replica reconciles; standbys serve HTTP and campaign.
		clientset, err := k8s.NewClientset()
		if err != nil {
			log.Fatalf("kubernetes clientset (leader election): %v", err)
		}
		go k8s.RunWithLeaderElection(bgCtx, clientset, cfg.Namespace, cfg.PodName, pool.Run)
	} else {
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
	bgCancel() // stop the background loops (TTL, pool, content store, seeder)
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
