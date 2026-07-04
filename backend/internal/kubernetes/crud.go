package kubernetes

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// List returns all sessions owned by ownerRef.
func (s *SessionService) List(ctx context.Context, ownerRef string) ([]models.Session, error) {
	if ownerRef == "" {
		return nil, nil // unclaimed pool members are ownerless; never listable
	}
	list, err := s.resource().List(ctx, metav1.ListOptions{
		LabelSelector: ownerLabel + "=" + ownerHash(ownerRef),
	})
	if err != nil {
		return nil, fmt.Errorf("list claims: %w", err)
	}
	out := make([]models.Session, 0, len(list.Items))
	for i := range list.Items {
		item := list.Items[i]
		// Defense in depth: a label collision must never expose another owner.
		if specOwner(&item) != ownerRef {
			continue
		}
		out = append(out, s.ToSession(&item))
	}
	return out, nil
}

// Get returns a single session by public id, only if owned by ownerRef.
// Unknown and unowned ids both yield ErrNotFound (no existence leak).
func (s *SessionService) Get(ctx context.Context, id, ownerRef string) (*models.Session, error) {
	obj, err := s.getOwned(ctx, id, ownerRef)
	if err != nil {
		return nil, err
	}
	sess := s.ToSession(obj)
	return &sess, nil
}

// Delete removes a session by public id, only if owned by ownerRef, and
// releases the owner's marker.
func (s *SessionService) Delete(ctx context.Context, id, ownerRef string) error {
	obj, err := s.getOwned(ctx, id, ownerRef)
	if err != nil {
		return err
	}
	if err := s.resource().Delete(ctx, obj.GetName(), metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("delete claim: %w", err)
	}
	_ = s.deleteOwnerMarker(ctx, ownerRef)
	return nil
}

// Authorize reports whether ownerRef owns the session identified by id. It
// returns nil when the caller owns the claim, and ErrNotFound/ErrInvalidID for
// unknown, unowned, or malformed ids. Callers exposing this to untrusted
// clients (the /authz endpoint) MUST collapse those into a single denial.
func (s *SessionService) Authorize(ctx context.Context, id, ownerRef string) error {
	_, err := s.getOwned(ctx, id, ownerRef)
	return err
}

// getOwned fetches a claim by public id and enforces ownership. An empty
// ownerRef never matches: unclaimed pool members carry an empty spec.ownerRef.
func (s *SessionService) getOwned(ctx context.Context, id, ownerRef string) (*unstructured.Unstructured, error) {
	if ownerRef == "" {
		return nil, ErrNotFound
	}
	name, err := s.nameFromID(id)
	if err != nil {
		return nil, err
	}
	obj, err := s.resource().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get claim: %w", err)
	}
	if specOwner(obj) != ownerRef {
		return nil, ErrNotFound
	}
	return obj, nil
}

// Watch returns a watch scoped to a single claim by name (used by SSE).
func (s *SessionService) Watch(ctx context.Context, name string) (watch.Interface, error) {
	return s.resource().Watch(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
	})
}

// WatchManaged returns a watch over every claim this backend manages (used by
// the pool manager to react without polling).
func (s *SessionService) WatchManaged(ctx context.Context) (watch.Interface, error) {
	return s.resource().Watch(ctx, metav1.ListOptions{
		LabelSelector: managedByLabel + "=" + managedByValue,
	})
}

// listManaged returns every claim this backend manages, across all owners.
func (s *SessionService) listManaged(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := s.resource().List(ctx, metav1.ListOptions{
		LabelSelector: managedByLabel + "=" + managedByValue,
	})
	if err != nil {
		return nil, fmt.Errorf("list managed claims: %w", err)
	}
	return list.Items, nil
}

// deleteByName deletes a claim by name with background propagation, so a slow
// child finalizer never blocks the caller. An already-gone claim is success.
func (s *SessionService) deleteByName(ctx context.Context, name string) error {
	policy := metav1.DeletePropagationBackground
	if err := s.resource().Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &policy}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete claim %q: %w", name, err)
	}
	return nil
}
