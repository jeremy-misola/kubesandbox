package kubernetes

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// markerName derives the deterministic marker name for an owner.
func markerName(ownerRef string) string {
	return markerNamePrefix + ownerHash(ownerRef)[:16]
}

// createOwnerMarker atomically reserves the one-sandbox-per-user slot.
func (s *SessionService) createOwnerMarker(ctx context.Context, ownerRef string) error {
	cm := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      markerName(ownerRef),
			"namespace": s.namespace,
			"labels": map[string]interface{}{
				managedByLabel: managedByValue,
				markerLabel:    "true",
			},
		},
		"data": map[string]interface{}{
			markerKeyOwner:  ownerRef,
			markerKeyMember: "",
		},
	}}
	_, err := s.configmaps().Create(ctx, cm, metav1.CreateOptions{})
	return err
}

// setOwnerMarkerMember records which member the owner holds (best-effort; the
// marker's existence, not its data, enforces the invariant).
func (s *SessionService) setOwnerMarkerMember(ctx context.Context, ownerRef, member string) {
	cm, err := s.configmaps().Get(ctx, markerName(ownerRef), metav1.GetOptions{})
	if err != nil {
		return
	}
	_ = unstructured.SetNestedField(cm.Object, member, "data", markerKeyMember)
	_, _ = s.configmaps().Update(ctx, cm, metav1.UpdateOptions{})
}

// deleteOwnerMarker releases an owner's slot. Missing markers are a no-op.
func (s *SessionService) deleteOwnerMarker(ctx context.Context, ownerRef string) error {
	err := s.configmaps().Delete(ctx, markerName(ownerRef), metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete owner marker: %w", err)
	}
	return nil
}

// listOwnerMarkers returns all per-owner markers (used by the pool manager's
// orphan GC).
func (s *SessionService) listOwnerMarkers(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := s.configmaps().List(ctx, metav1.ListOptions{
		LabelSelector: managedByLabel + "=" + managedByValue + "," + markerLabel + "=true",
	})
	if err != nil {
		return nil, fmt.Errorf("list owner markers: %w", err)
	}
	return list.Items, nil
}
