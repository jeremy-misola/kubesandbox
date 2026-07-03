package kubernetes

import (
	"context"
	"fmt"
	"sort"
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
func (s *SessionService) Assign(ctx context.Context, ownerRef string, req models.CreateSessionRequest) (*models.Session, error) {
	if ownerRef == "" {
		return nil, fmt.Errorf("empty ownerRef")
	}
	ttl := clampTTL(req.TTLMinutes)

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

	members, err := s.listAssignableMembers(ctx)
	if err != nil {
		_ = s.deleteOwnerMarker(ctx, ownerRef)
		s.metrics.RecordAssignAttempt(ctx, telemetry.ResultError)
		return nil, err
	}

	// TTL starts at assignment, not warm creation: spec.expiresAt is the
	// authoritative expiry read by cleanup.go.
	expiresAt := s.now().UTC().Add(time.Duration(ttl) * time.Minute).Format(time.RFC3339)
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
		m.SetAnnotations(annots)

		updated, err := s.resource().Update(ctx, m, metav1.UpdateOptions{})
		if err != nil {
			if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
				// Raced with another assignment/recycle; try the next member.
				s.metrics.RecordAssignAttempt(ctx, telemetry.ResultConflictRetry)
				continue
			}
			_ = s.deleteOwnerMarker(ctx, ownerRef)
			s.metrics.RecordAssignAttempt(ctx, telemetry.ResultError)
			return nil, fmt.Errorf("claim pool member %q: %w", m.GetName(), err)
		}

		s.setOwnerMarkerMember(ctx, ownerRef, updated.GetName())
		s.metrics.RecordAssignAttempt(ctx, telemetry.ResultSuccess)
		sess := s.ToSession(updated)
		return &sess, nil
	}

	_ = s.deleteOwnerMarker(ctx, ownerRef)
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
