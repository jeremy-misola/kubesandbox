// Package challenges implements the guided-challenges runtime: the Seeder
// (async state machine that applies a challenge bundle into an assigned
// session's vcluster, design §6) and the Grader (on-demand declarative check
// evaluation against live tenant state, design §7). Both consume the
// tenant-client factory via the TenantOps interface.
package challenges

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jeremy-misola/kubesandbox/backend/internal/content"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	"github.com/jeremy-misola/kubesandbox/backend/internal/telemetry"
)

// SessionOps is the seeder's claim-side surface, satisfied by
// *kubernetes.SessionService. Every mutation is a CAS Update or an idempotent
// delete — the properties the crash-safety story rests on.
type SessionOps interface {
	GetClaim(ctx context.Context, name string) (*unstructured.Unstructured, error)
	UpdateClaim(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	DeleteClaimKeepMarker(ctx context.Context, name string) error
	ReleaseOwnerMarker(ctx context.Context, ownerRef string) error
	ListSeedWork(ctx context.Context) ([]unstructured.Unstructured, error)
	ReassignForRecycle(ctx context.Context, ownerRef string, req models.CreateSessionRequest, expiresAt string, recycles int) (*models.Session, error)
}

// TenantOps is the tenant-vcluster surface shared by seeder and grader. The
// production implementation (NewTenantOps) wraps the tenant-client factory;
// tests fake it.
type TenantOps interface {
	// Apply server-side-applies the bundle's seed objects, in order, into the
	// session's vcluster. Idempotent (SSA, fieldManager kubesandbox-seeder,
	// force) — re-running a half-finished seed converges instead of erroring,
	// which is the entire retry story (§6.2).
	Apply(ctx context.Context, claimName string, bundle *content.Bundle) error
	// DeleteSeeded removes everything labeled kubesandbox.com/challenge=<id>
	// from the tenant (namespaces first) and waits for deletion — the reset
	// path (§7).
	DeleteSeeded(ctx context.Context, claimName, challengeID string) error
	// Grading reads.
	GetObject(ctx context.Context, claimName string, target content.TargetRef) (*unstructured.Unstructured, error)
	ListObjects(ctx context.Context, claimName string, target content.TargetRef) ([]unstructured.Unstructured, error)
	// CanI evaluates a SelfSubjectAccessReview while impersonating the given
	// tenant ServiceAccount (subjectCan/subjectCannot checks).
	CanI(ctx context.Context, claimName string, sa content.SubjectRef, access content.AccessRef) (bool, error)
}

// SeederConfig sizes the seeder. All values are env-configurable.
type SeederConfig struct {
	// Budget bounds one apply attempt (design §6.2: 10s, generous against the
	// measured 1-2s).
	Budget time.Duration
	// ResetBudget bounds the delete-and-wait phase of a reset re-seed
	// (namespace deletion takes seconds, not milliseconds).
	ResetBudget time.Duration
	// MaxAttempts is the retry-in-place ceiling per member (§6.3.1).
	MaxAttempts int
	// Backoff between in-place retries.
	Backoff time.Duration
	// Resync is the level-triggered reconcile interval (backstop behind the
	// assign-path notifier).
	Resync time.Duration
	// Workers is the number of concurrent seed workers. Concurrency is safe:
	// every state transition is CAS'd and the apply is idempotent.
	Workers int
}

// Seeder drives the per-claim seed state machine (§6.1):
//
//	pending → seeding → seeded | failed
//
// It receives claim names on a channel from the assign path (fast case) AND
// reconciles on startup + a slow resync by listing claimed members with a
// non-terminal seed state — the same level-triggered pattern as the pool
// manager, which is what makes a crash mid-seed a non-event: the interrupted
// claim is still pending/seeding, and SSA converges on re-apply.
type Seeder struct {
	ops    SessionOps
	store  content.Store
	tenant TenantOps
	cfg    SeederConfig

	work chan string

	mu       sync.Mutex
	inflight map[string]bool

	metrics *telemetry.Metrics
	now     func() time.Time
	// sleep is injectable so retry/backoff tests don't wall-clock wait.
	sleep func(ctx context.Context, d time.Duration)
}

// NewSeeder constructs a Seeder. Zero config fields get safe defaults
// (budget 10s, reset budget 60s, 3 attempts, 2s backoff, 30s resync, 2
// workers).
func NewSeeder(ops SessionOps, store content.Store, tenant TenantOps, cfg SeederConfig) *Seeder {
	if cfg.Budget <= 0 {
		cfg.Budget = 10 * time.Second
	}
	if cfg.ResetBudget <= 0 {
		cfg.ResetBudget = 60 * time.Second
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.Backoff <= 0 {
		cfg.Backoff = 2 * time.Second
	}
	if cfg.Resync <= 0 {
		cfg.Resync = 30 * time.Second
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	return &Seeder{
		ops:      ops,
		store:    store,
		tenant:   tenant,
		cfg:      cfg,
		work:     make(chan string, 64),
		inflight: map[string]bool{},
		now:      time.Now,
		sleep: func(ctx context.Context, d time.Duration) {
			select {
			case <-ctx.Done():
			case <-time.After(d):
			}
		},
	}
}

// SetMetrics injects the telemetry instrument set (nil is a valid no-op).
func (s *Seeder) SetMetrics(m *telemetry.Metrics) { s.metrics = m }

// Enqueue requests seeding of a claim (non-blocking; a full channel is fine —
// the resync reconcile is authoritative).
func (s *Seeder) Enqueue(claimName string) {
	select {
	case s.work <- claimName:
	default:
	}
}

// Run processes seed work until ctx is cancelled: an immediate startup
// reconcile (crash resume: claims still pending/seeding), the notifier
// channel, and the periodic resync.
func (s *Seeder) Run(ctx context.Context) {
	log.Printf("seeder: started (budget=%s attempts=%d workers=%d resync=%s)",
		s.cfg.Budget, s.cfg.MaxAttempts, s.cfg.Workers, s.cfg.Resync)

	var wg sync.WaitGroup
	for i := 0; i < s.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case name := <-s.work:
					s.processGuarded(ctx, name)
				}
			}
		}()
	}

	s.ReconcileOnce(ctx)
	ticker := time.NewTicker(s.cfg.Resync)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			log.Printf("seeder: stopped")
			return
		case <-ticker.C:
			s.ReconcileOnce(ctx)
		}
	}
}

// ReconcileOnce enqueues every claim with outstanding seed work. Exported for
// tests and the startup path.
func (s *Seeder) ReconcileOnce(ctx context.Context) {
	claims, err := s.ops.ListSeedWork(ctx)
	if err != nil {
		log.Printf("seeder: reconcile list failed: %v", err)
		return
	}
	for i := range claims {
		s.Enqueue(claims[i].GetName())
	}
}

// processGuarded runs Process with per-claim single-flight (duplicate
// enqueues are common: notifier + resync).
func (s *Seeder) processGuarded(ctx context.Context, name string) {
	s.mu.Lock()
	if s.inflight[name] {
		s.mu.Unlock()
		return
	}
	s.inflight[name] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.inflight, name)
		s.mu.Unlock()
	}()
	s.Process(ctx, name)
}

// Process drives one claim to a terminal seed state. Exported for tests.
//
// Failure ladder (§6.3) — the user is never handed a half-seeded cluster:
//  1. retry in place (MaxAttempts, SSA converges over partial applies),
//  2. recycle the member and re-assign once (marker kept),
//  3. fail closed: seed-state=failed, claim deleted, marker released.
func (s *Seeder) Process(ctx context.Context, name string) {
	for {
		if ctx.Err() != nil {
			return
		}
		obj, err := s.ops.GetClaim(ctx, name)
		if err != nil {
			if errors.Is(err, k8s.ErrNotFound) {
				return // deleted underneath us (TTL, user, recycle) — nothing to do
			}
			log.Printf("seeder: get %s: %v", name, err)
			return
		}
		id := k8s.ChallengeID(obj)
		if id == "" {
			return
		}
		switch k8s.SeedState(obj) {
		case k8s.SeedStateSeeded, k8s.SeedStateFailed:
			return
		}

		bundle, ok := s.store.Get(id)
		if !ok {
			// Content vanished (or got quarantined) between create-validation
			// and seeding. No member will ever seed this — fail closed
			// without burning a recycle.
			log.Printf("seeder: %s references unknown/quarantined challenge %q — failing closed", name, id)
			if !s.failClosed(ctx, obj) {
				continue // fence lost a race; re-read and re-evaluate
			}
			return
		}

		if k8s.SeedAttempts(obj) >= s.cfg.MaxAttempts {
			if !s.escalate(ctx, obj, bundle) {
				continue // fence lost a race; re-read and re-evaluate
			}
			return
		}

		// CAS pending|seeding → seeding, attempts+1. The transition is the
		// lightweight lease (§6.1): with multiple replicas, the loser sees
		// the conflict and re-reads; even a double-apply is harmless (SSA).
		reset := k8s.SeedResetRequested(obj)
		next := obj.DeepCopy()
		annots := next.GetAnnotations()
		annots[k8s.SeedStateAnnot] = k8s.SeedStateSeeding
		annots[k8s.SeedAttemptsAnnot] = strconv.Itoa(k8s.SeedAttempts(obj) + 1)
		delete(annots, k8s.SeedResetAnnot)
		next.SetAnnotations(annots)
		if _, err := s.ops.UpdateClaim(ctx, next); err != nil {
			if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
				continue // re-read and re-evaluate
			}
			log.Printf("seeder: CAS %s -> seeding: %v", name, err)
			return
		}

		if err := s.seedOnce(ctx, name, bundle, reset); err != nil {
			log.Printf("seeder: seed %s (challenge %s) attempt failed: %v", name, id, err)
			s.metrics.RecordSeedAttempt(ctx, telemetry.SeedResultRetry)
			s.sleep(ctx, s.cfg.Backoff)
			continue // re-read: attempts are persisted; ladder advances
		}

		// CAS → seeded. On conflict re-read: if the claim is gone or was
		// re-marked (reset raced), the loop re-evaluates from truth.
		if s.casState(ctx, name, k8s.SeedStateSeeding, k8s.SeedStateSeeded) {
			log.Printf("seeder: %s seeded (challenge %s)", name, id)
			s.metrics.RecordSeedAttempt(ctx, telemetry.SeedResultSuccess)
		}
		return
	}
}

// seedOnce runs one attempt: optional reset deletion, then the bounded apply.
func (s *Seeder) seedOnce(ctx context.Context, name string, bundle *content.Bundle, reset bool) error {
	if reset {
		rctx, cancel := context.WithTimeout(ctx, s.cfg.ResetBudget)
		err := s.tenant.DeleteSeeded(rctx, name, bundle.ID)
		cancel()
		if err != nil {
			return fmt.Errorf("reset delete: %w", err)
		}
	}
	start := s.now()
	actx, cancel := context.WithTimeout(ctx, s.cfg.Budget)
	err := s.tenant.Apply(actx, name, bundle)
	cancel()
	if err != nil {
		return err
	}
	s.metrics.RecordSeedDuration(ctx, bundle.ID, s.now().Sub(start))
	return nil
}

// casState re-reads the claim and CAS-updates fromState → toState, retrying
// on conflict. Returns false when the claim vanished or left fromState.
func (s *Seeder) casState(ctx context.Context, name, fromState, toState string) bool {
	for {
		obj, err := s.ops.GetClaim(ctx, name)
		if err != nil {
			return false
		}
		if k8s.SeedState(obj) != fromState {
			return false
		}
		next := obj.DeepCopy()
		annots := next.GetAnnotations()
		annots[k8s.SeedStateAnnot] = toState
		next.SetAnnotations(annots)
		if _, err := s.ops.UpdateClaim(ctx, next); err != nil {
			if apierrors.IsConflict(err) {
				continue
			}
			return false
		}
		return true
	}
}

// escalate handles an exhausted member: recycle-and-reassign once (§6.3.2),
// then fail closed (§6.3.3). Returns false when its view of the claim proved
// stale (the caller must re-read and re-evaluate).
//
// DESTRUCTIVE ACTIONS ARE CAS-FENCED: claim deletion itself is not
// rv-guarded, so before deleting anything the escalation writes an
// rv-guarded annotation update. A racing seeder that just CAS'd the claim
// (e.g. to seeded — its apply succeeded while our view aged) makes the fence
// conflict, and we back off instead of deleting a healthy session. This is
// the same "every transition is a CAS" rule (§6.1) applied to the failure
// ladder; the -race concurrency test locks it in.
func (s *Seeder) escalate(ctx context.Context, obj *unstructured.Unstructured, bundle *content.Bundle) bool {
	name := obj.GetName()
	owner := k8s.ClaimOwner(obj)

	if k8s.SeedRecycles(obj) > 0 {
		return s.failClosed(ctx, obj)
	}

	// Fence for the recycle path: record the recycle on the old claim first.
	// (If we crash between this fence and the delete, the resync re-reads a
	// claim with recycles=1 and takes the fail-closed branch — conservative
	// but safe: the user is never left with a half-seeded cluster.)
	next := obj.DeepCopy()
	annots := next.GetAnnotations()
	annots[k8s.SeedRecyclesAnnot] = "1"
	next.SetAnnotations(annots)
	if _, err := s.ops.UpdateClaim(ctx, next); err != nil {
		if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
			return false // stale view — someone else transitioned the claim
		}
		log.Printf("seeder: recycle fence for %s: %v", name, err)
		return true // give up this pass; resync retries the ladder
	}

	log.Printf("seeder: recycling %s after %d failed attempts (challenge %s)", name, k8s.SeedAttempts(obj), bundle.ID)
	s.metrics.RecordSeedAttempt(ctx, telemetry.SeedResultRecycled)

	// Delete the bad member (cascades the vcluster) but KEEP the owner
	// marker: the user's reservation survives the swap.
	if err := s.ops.DeleteClaimKeepMarker(ctx, name); err != nil {
		log.Printf("seeder: recycle delete %s: %v", name, err)
		return true // resync will retry the whole ladder
	}

	req := models.CreateSessionRequest{
		TTLMinutes:  k8s.ClaimTTLMinutes(obj),
		ChallengeID: bundle.ID,
	}
	sess, err := s.ops.ReassignForRecycle(ctx, owner, req, k8s.ClaimExpiresAt(obj), 1)
	if err != nil {
		// No replacement member (pool empty) or claim failure: fail closed —
		// release the marker so the user can retry, exactly as if the
		// original create had failed.
		log.Printf("seeder: re-assign after recycle failed for %s: %v", name, err)
		s.metrics.RecordSeedAttempt(ctx, telemetry.SeedResultFailed)
		if err := s.ops.ReleaseOwnerMarker(ctx, owner); err != nil {
			log.Printf("seeder: release marker for %s: %v", owner, err)
		}
		return true
	}
	log.Printf("seeder: re-assigned %s -> %s (challenge %s)", name, sess.Name, bundle.ID)
	s.Enqueue(sess.Name)
	return true
}

// failClosed is the terminal failure (§6.3.3): mark failed (SSE surfaces it),
// delete the claim, release the marker. The user can retry; they are never
// handed a cluster in an unknown state. The failed-marker write doubles as
// the CAS fence: a conflict means the view was stale, and the caller must
// re-read instead of deleting (returns false).
func (s *Seeder) failClosed(ctx context.Context, obj *unstructured.Unstructured) bool {
	name := obj.GetName()
	owner := k8s.ClaimOwner(obj)

	next := obj.DeepCopy()
	annots := next.GetAnnotations()
	annots[k8s.SeedStateAnnot] = k8s.SeedStateFailed
	next.SetAnnotations(annots)
	if _, err := s.ops.UpdateClaim(ctx, next); err != nil {
		if apierrors.IsConflict(err) {
			return false // stale view — re-evaluate before destroying anything
		}
		if !apierrors.IsNotFound(err) {
			log.Printf("seeder: mark failed %s: %v", name, err)
			return true // resync retries
		}
		// Already deleted underneath us: still release the marker below.
	}

	log.Printf("seeder: failing closed on %s (challenge %s)", name, k8s.ChallengeID(obj))
	s.metrics.RecordSeedAttempt(ctx, telemetry.SeedResultFailed)

	if err := s.ops.DeleteClaimKeepMarker(ctx, name); err != nil {
		log.Printf("seeder: fail-closed delete %s: %v", name, err)
	}
	if owner != "" {
		if err := s.ops.ReleaseOwnerMarker(ctx, owner); err != nil {
			log.Printf("seeder: release marker for %s: %v", owner, err)
		}
	}
	return true
}
