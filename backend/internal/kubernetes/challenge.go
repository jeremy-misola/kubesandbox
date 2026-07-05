package kubernetes

import (
	"context"
	"fmt"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// Challenge state on the claim (design §6.1). Annotations are host-claim,
// backend-owned, survive restarts, and already flow through the SSE claim
// watch — no new store, no XRD change. spec.starterLabRef (existing, unused,
// maxLength 128) carries the challenge id in spec.
const (
	// ChallengeIDAnnot records which bundle the session was created for. Set
	// atomically with ownership in the same CAS Update that claims the member.
	ChallengeIDAnnot = "kubesandbox.com/challenge-id"
	// SeedStateAnnot is the seeder state machine: pending → seeding → seeded |
	// failed. Every transition is a CAS Update (resourceVersion-guarded).
	SeedStateAnnot = "kubesandbox.com/seed-state"
	// SeedAttemptsAnnot counts apply attempts (persisted so retries survive a
	// backend crash).
	SeedAttemptsAnnot = "kubesandbox.com/seed-attempts"
	// SeedRecyclesAnnot counts recycle-and-reassign passes; the seeder
	// recycles at most once (§6.3.2) before failing closed.
	SeedRecyclesAnnot = "kubesandbox.com/seed-recycles"
	// SeedResetAnnot marks a reset request: the seeder deletes seeded state
	// from the tenant before re-applying, then clears the flag in the
	// pending→seeding CAS. Durable intent — a crash mid-reset re-runs it.
	SeedResetAnnot = "kubesandbox.com/seed-reset"
)

// Seed states.
const (
	SeedStatePending = "pending"
	SeedStateSeeding = "seeding"
	SeedStateSeeded  = "seeded"
	SeedStateFailed  = "failed"
)

// ChallengeID returns the claim's challenge annotation ("" = not a challenge
// session).
func ChallengeID(obj *unstructured.Unstructured) string {
	return obj.GetAnnotations()[ChallengeIDAnnot]
}

// SeedState returns the claim's seed-state annotation.
func SeedState(obj *unstructured.Unstructured) string {
	return obj.GetAnnotations()[SeedStateAnnot]
}

// SeedAttempts returns the persisted attempt counter.
func SeedAttempts(obj *unstructured.Unstructured) int {
	n, _ := strconv.Atoi(obj.GetAnnotations()[SeedAttemptsAnnot])
	return n
}

// SeedRecycles returns the persisted recycle counter.
func SeedRecycles(obj *unstructured.Unstructured) int {
	n, _ := strconv.Atoi(obj.GetAnnotations()[SeedRecyclesAnnot])
	return n
}

// SeedResetRequested reports whether a reset is pending on the claim.
func SeedResetRequested(obj *unstructured.Unstructured) bool {
	return obj.GetAnnotations()[SeedResetAnnot] == "true"
}

// ClaimOwner returns spec.ownerRef ("" for unclaimed warm members).
func ClaimOwner(obj *unstructured.Unstructured) string { return specOwner(obj) }

// ClaimExpiresAt returns spec.expiresAt (RFC3339, "" if unset).
func ClaimExpiresAt(obj *unstructured.Unstructured) string {
	v, _, _ := unstructured.NestedString(obj.Object, "spec", "expiresAt")
	return v
}

// ClaimTTLMinutes returns spec.ttlMinutes (0 if unset).
func ClaimTTLMinutes(obj *unstructured.Unstructured) int {
	n, _, _ := unstructured.NestedInt64(obj.Object, "spec", "ttlMinutes")
	return int(n)
}

// GetClaim fetches a managed claim by name (no ownership filter — seeder use;
// callers exposing data to users must go through GetOwnedClaim).
func (s *SessionService) GetClaim(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	obj, err := s.resource().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get claim %q: %w", name, err)
	}
	return obj, nil
}

// GetOwnedClaim fetches a claim by public session id, enforcing ownership
// exactly like Get/Delete (unknown, unowned and malformed ids are all
// ErrNotFound — no existence leak).
func (s *SessionService) GetOwnedClaim(ctx context.Context, id, ownerRef string) (*unstructured.Unstructured, error) {
	return s.getOwned(ctx, id, ownerRef)
}

// UpdateClaim performs a CAS update: obj must carry the resourceVersion the
// caller observed; the API server rejects stale writes with a Conflict, which
// is returned unwrapped so callers can re-read and re-evaluate.
func (s *SessionService) UpdateClaim(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return s.resource().Update(ctx, obj, metav1.UpdateOptions{})
}

// DeleteClaimKeepMarker deletes a claim WITHOUT releasing the owner's marker —
// the seeder's recycle path (§6.3.2): the owner keeps their reservation while
// the replacement member is claimed.
func (s *SessionService) DeleteClaimKeepMarker(ctx context.Context, name string) error {
	return s.deleteByName(ctx, name)
}

// ReleaseOwnerMarker releases an owner's one-per-user reservation (seeder
// fail-closed path §6.3.3).
func (s *SessionService) ReleaseOwnerMarker(ctx context.Context, ownerRef string) error {
	return s.deleteOwnerMarker(ctx, ownerRef)
}

// ListSeedWork returns claimed challenge sessions whose seed state is not
// terminal (pending or seeding, including reset requests) — the seeder's
// startup/resync reconcile source (§6.1: level-triggered, like the pool
// manager, which is what makes a crash mid-seed a non-event).
func (s *SessionService) ListSeedWork(ctx context.Context) ([]unstructured.Unstructured, error) {
	claims, err := s.listManaged(ctx)
	if err != nil {
		return nil, err
	}
	var out []unstructured.Unstructured
	for i := range claims {
		c := claims[i]
		if c.GetDeletionTimestamp() != nil || specOwner(&c) == "" || ChallengeID(&c) == "" {
			continue
		}
		switch SeedState(&c) {
		case SeedStatePending, SeedStateSeeding:
			out = append(out, c)
		}
	}
	return out, nil
}

// ReassignForRecycle claims the next warm member for an owner whose marker is
// ALREADY held (the recycle path — Assign would fail AlreadyExists). The new
// claim inherits the original expiry so a recycle never extends the session's
// clock, and records the incremented recycle counter.
func (s *SessionService) ReassignForRecycle(ctx context.Context, ownerRef string, req models.CreateSessionRequest, expiresAt string, recycles int) (*models.Session, error) {
	if ownerRef == "" {
		return nil, fmt.Errorf("empty ownerRef")
	}
	return s.claimReadyMember(ctx, ownerRef, req, expiresAt, recycles)
}
