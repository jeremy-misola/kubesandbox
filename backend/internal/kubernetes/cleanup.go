package kubernetes

import (
	"context"
	"log"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// TTLController enforces session TTLs server-side (G3). Sessions are ephemeral;
// this deletes any managed claim past its expiry regardless of client behaviour.
//
// SAFE CLEANUP (per the 2026-06-24 post-mortem): it deletes only the top-level
// KubeSandboxSession CLAIM and lets Crossplane cascade teardown via owner
// references, using a BACKGROUND delete so a slow/stuck finalizer never blocks
// the loop. It never blocks on child teardown; the sweep CronJob is the backstop
// for namespaces orphaned by a wedged delete.
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
	}
	return deleted, nil
}

// claimExpiry computes when a claim expires. It prefers the controller-published
// status.expiresAt, falling back to creationTimestamp + spec.ttlMinutes so TTL is
// enforced even though nothing currently populates status.expiresAt.
func claimExpiry(obj *unstructured.Unstructured) (time.Time, bool) {
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
