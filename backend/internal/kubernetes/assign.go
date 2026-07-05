package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	"github.com/jeremy-misola/kubesandbox/backend/internal/telemetry"
)

// Assign hands an available, Ready, fresh pool member to ownerRef.
//
// One-per-user: a per-owner marker ConfigMap is created FIRST, so a concurrent
// duplicate create fails AlreadyExists at the API server. One-user-per-member:
// the member is claimed with an Update carrying the resourceVersion observed at
// list time (optimistic concurrency); the loser of a race retries the next
// member. If no member can be claimed the marker is rolled back and
// ErrPoolEmpty is returned so the handler can queue the request.
//
// Challenges (design §6.1): when req names a challenge, the SAME CAS Update
// that claims the member also stamps spec.starterLabRef and the
// challenge-id/seed-state=pending annotations — atomic with ownership, so
// there is no window where a challenge session exists without its seed intent
// recorded. Assign never seeds; it stays a sub-second metadata change, and the
// async seeder picks the claim up from the notifier/reconcile.
func (s *SessionService) Assign(ctx context.Context, ownerRef string, req models.CreateSessionRequest) (*models.Session, error) {
	if ownerRef == "" {
		return nil, fmt.Errorf("empty ownerRef")
	}

	if err := s.createOwnerMarker(ctx, ownerRef); err != nil {
		if apierrors.IsAlreadyExists(err) {
			s.metrics.RecordAssignAttempt(ctx, telemetry.ResultAlreadyExists)
			return nil, ErrAlreadyExists
		}
		s.metrics.RecordAssignAttempt(ctx, telemetry.ResultError)
		return nil, fmt.Errorf("create owner marker: %w", err)
	}

	// Migration guard: legacy (pre-pool) sessions have no marker, so the create
	// above succeeded; refuse a second sandbox.
	existing, err := s.List(ctx, ownerRef)
	if err != nil {
		_ = s.deleteOwnerMarker(ctx, ownerRef)
		s.metrics.RecordAssignAttempt(ctx, telemetry.ResultError)
		return nil, err
	}
	if len(existing) > 0 {
		_ = s.deleteOwnerMarker(ctx, ownerRef)
		s.metrics.RecordAssignAttempt(ctx, telemetry.ResultAlreadyExists)
		return nil, ErrAlreadyExists
	}

	sess, err := s.claimReadyMember(ctx, ownerRef, req, "", 0)
	if err != nil {
		_ = s.deleteOwnerMarker(ctx, ownerRef)
		return nil, err
	}
	return sess, nil
}

// claimReadyMember claims the oldest Ready member for ownerRef via CAS. The
// caller owns marker lifecycle. expiresAtOverride ("" = now + ttl) preserves
// the original expiry on the seeder's recycle path; recycles > 0 is stamped
// so the seeder can enforce its recycle-at-most-once rule.
func (s *SessionService) claimReadyMember(ctx context.Context, ownerRef string, req models.CreateSessionRequest, expiresAtOverride string, recycles int) (*models.Session, error) {
	ttl := clampTTL(req.TTLMinutes)

	members, err := s.listAssignableMembers(ctx)
	if err != nil {
		s.metrics.RecordAssignAttempt(ctx, telemetry.ResultError)
		return nil, err
	}

	// TTL starts at assignment, not warm creation: spec.expiresAt is the
	// authoritative expiry read by cleanup.go.
	expiresAt := expiresAtOverride
	if expiresAt == "" {
		expiresAt = s.now().UTC().Add(time.Duration(ttl) * time.Minute).Format(time.RFC3339)
	}
	challengeID := req.EffectiveChallengeID()

	for i := range members {
		m := members[i].DeepCopy()

		_ = unstructured.SetNestedField(m.Object, ownerRef, "spec", "ownerRef")
		_ = unstructured.SetNestedField(m.Object, ownerRef, "spec", "tenantRef")
		_ = unstructured.SetNestedField(m.Object, int64(ttl), "spec", "ttlMinutes")
		_ = unstructured.SetNestedField(m.Object, expiresAt, "spec", "expiresAt")

		labels := m.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[ownerLabel] = ownerHash(ownerRef)
		labels[poolLabel] = poolClaimed
		m.SetLabels(labels)

		annots := m.GetAnnotations()
		if annots == nil {
			annots = map[string]string{}
		}
		annots[ownerRefAnnot] = ownerRef
		if challengeID != "" {
			// Atomic with ownership: challenge selection + seed intent land in
			// the same CAS write that claims the member (§6.1). The existing,
			// previously-unused starterLabRef field carries the id in spec —
			// no XRD migration.
			_ = unstructured.SetNestedField(m.Object, challengeID, "spec", "starterLabRef")
			annots[ChallengeIDAnnot] = challengeID
			annots[SeedStateAnnot] = SeedStatePending
			annots[SeedAttemptsAnnot] = "0"
			if recycles > 0 {
				annots[SeedRecyclesAnnot] = strconv.Itoa(recycles)
			}
		}
		m.SetAnnotations(annots)

		updated, err := s.resource().Update(ctx, m, metav1.UpdateOptions{})
		if err != nil {
			if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
				// Raced with another assignment/recycle; try the next member.
				s.metrics.RecordAssignAttempt(ctx, telemetry.ResultConflictRetry)
				continue
			}
			s.metrics.RecordAssignAttempt(ctx, telemetry.ResultError)
			return nil, fmt.Errorf("claim pool member %q: %w", m.GetName(), err)
		}

		s.setOwnerMarkerMember(ctx, ownerRef, updated.GetName())
		s.metrics.RecordAssignAttempt(ctx, telemetry.ResultSuccess)
		if challengeID != "" {
			s.notifySeed(updated.GetName())
		}
		sess := s.ToSession(updated)
		return &sess, nil
	}

	s.metrics.RecordAssignAttempt(ctx, telemetry.ResultPoolEmpty)
	return nil, ErrPoolEmpty
}

// listAssignableMembers returns available, not-deleting, Ready, fresh pool
// members, oldest first.
func (s *SessionService) listAssignableMembers(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := s.resource().List(ctx, metav1.ListOptions{
		LabelSelector: managedByLabel + "=" + managedByValue + "," + poolLabel + "=" + poolAvailable,
	})
	if err != nil {
		return nil, fmt.Errorf("list pool members: %w", err)
	}
	out := make([]unstructured.Unstructured, 0, len(list.Items))
	now := s.now()
	for i := range list.Items {
		m := list.Items[i]
		if m.GetDeletionTimestamp() != nil {
			continue
		}
		if ready, _, _ := unstructured.NestedBool(m.Object, "status", "workspaceReady"); !ready {
			continue
		}
		if s.maxWarmAge > 0 && now.Sub(m.GetCreationTimestamp().Time) > s.maxWarmAge {
			continue // stale — recycled by the pool manager, never handed out
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].GetCreationTimestamp().Time.Before(out[j].GetCreationTimestamp().Time)
	})
	return out, nil
}
