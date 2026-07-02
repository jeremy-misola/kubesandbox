package kubernetes

import (
	"context"
	"log"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// TTLController enforces session TTLs server-side. Sessions are ephemeral; this
// deletes any managed claim past its expiry regardless of client behaviour.
//
// It deletes only the top-level KubeSandboxSession claim and lets Crossplane
// cascade teardown via owner references, using a background delete so a stuck
// finalizer never blocks the loop. The sweep CronJob is the backstop for
// namespaces orphaned by a wedged delete.
type TTLController struct {
	svc      *SessionService
	interval time.Duration
	now      func() time.Time
}

// NewTTLController constructs a TTLController. A non-positive interval defaults
// to one minute.
func NewTTLController(svc *SessionService, interval time.Duration) *TTLController {
	if interval <= 0 {
		interval = time.Minute
	}
	return &TTLController{svc: svc, interval: interval, now: time.Now}
}

// Run reconciles once per interval until ctx is cancelled. It runs an immediate
// pass on start so a restart reaps anything that expired while down.
func (t *TTLController) Run(ctx context.Context) {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	log.Printf("ttl: cleanup loop started (interval=%s)", t.interval)

	for {
		if n, err := t.reconcileOnce(ctx); err != nil {
			log.Printf("ttl: reconcile error: %v", err)
		} else if n > 0 {
			log.Printf("ttl: deleted %d expired session(s)", n)
		}

		select {
		case <-ctx.Done():
			log.Printf("ttl: cleanup loop stopped")
			return
		case <-ticker.C:
		}
	}
}

// reconcileOnce deletes every managed claim whose expiry is in the past.
func (t *TTLController) reconcileOnce(ctx context.Context) (int, error) {
	claims, err := t.svc.listManaged(ctx)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for i := range claims {
		c := claims[i]
		// Already being torn down: don't re-issue deletes.
		if c.GetDeletionTimestamp() != nil {
			continue
		}
		// Unclaimed warm members have no lifecycle yet — their TTL starts at
		// assignment, and the pool manager owns them. Reaping here would drain
		// the pool.
		if poolState(&c) == poolAvailable {
			continue
		}
		exp, ok := claimExpiry(&c)
		if !ok || t.now().Before(exp) {
			continue
		}
		if err := t.svc.deleteByName(ctx, c.GetName()); err != nil {
			// Log and continue; the next tick retries. A single stuck claim must
			// not stall reaping of the others.
			log.Printf("ttl: delete %s failed: %v", c.GetName(), err)
			continue
		}
		deleted++
		// Release the owner's one-per-user marker so they can create again.
		if owner := specOwner(&c); owner != "" {
			if err := t.svc.deleteOwnerMarker(ctx, owner); err != nil {
				log.Printf("ttl: marker cleanup for %s failed: %v", c.GetName(), err)
			}
		}
	}
	return deleted, nil
}

// claimExpiry computes when a claim expires, in preference order:
//
//  1. spec.expiresAt — set by the backend at ASSIGNMENT (hand-over), which is
//     when the lifecycle clock starts for pool members. Authoritative.
//  2. status.expiresAt — the composition's echo of (1); may lag a reconcile.
//  3. creationTimestamp + spec.ttlMinutes — legacy fallback for claims created
//     before spec.expiresAt existed.
func claimExpiry(obj *unstructured.Unstructured) (time.Time, bool) {
	if v, ok, _ := unstructured.NestedString(obj.Object, "spec", "expiresAt"); ok && v != "" {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			return ts.UTC(), true
		}
	}
	if v, ok, _ := unstructured.NestedString(obj.Object, "status", "expiresAt"); ok && v != "" {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			return ts.UTC(), true
		}
	}

	created := obj.GetCreationTimestamp().Time
	if created.IsZero() {
		return time.Time{}, false
	}
	ttl, ok, _ := unstructured.NestedInt64(obj.Object, "spec", "ttlMinutes")
	if !ok || ttl <= 0 {
		ttl = int64(models.DefaultTTLMinutes)
	}
	return created.Add(time.Duration(ttl) * time.Minute).UTC(), true
}
