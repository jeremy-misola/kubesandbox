package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/jeremy-misola/kubesandbox/backend/internal/content"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/telemetry"
)

// seedFieldManager is the SSA field manager for every seeded object (§6.2).
// A fixed manager + force:true means retries, crash resumes and replica races
// all converge on the same field ownership instead of conflicting.
const seedFieldManager = "kubesandbox-seeder"

// deleteFanout bounds how many resource kinds DeleteSeeded scans/deletes
// concurrently during a reset — enough to stop ~20 known kinds from
// serializing behind each other's network round-trip, not so much that a
// reset could hammer a tenant API server.
const deleteFanout = 6

var (
	namespaceGVR = schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	ssarGVR      = schema.GroupVersionResource{Group: "authorization.k8s.io", Version: "v1", Resource: "selfsubjectaccessreviews"}
)

// tenantOps is the production TenantOps over the tenant-client factory.
type tenantOps struct {
	factory *k8s.TenantClientFactory
	metrics *telemetry.Metrics
}

// NewTenantOps wraps the tenant-client factory in the TenantOps interface.
func NewTenantOps(factory *k8s.TenantClientFactory, metrics *telemetry.Metrics) TenantOps {
	return &tenantOps{factory: factory, metrics: metrics}
}

// client resolves the session's tenant client, counting failures.
func (t *tenantOps) client(ctx context.Context, claimName string) (*k8s.TenantClient, error) {
	c, err := t.factory.ForSession(ctx, claimName)
	if err != nil {
		t.metrics.RecordTenantClientError(ctx)
	}
	return c, err
}

// Apply implements TenantOps: ordered server-side apply of the bundle's seed
// objects (Namespaces → RBAC → everything else; the loader pre-sorted them).
func (t *tenantOps) Apply(ctx context.Context, claimName string, bundle *content.Bundle) error {
	c, err := t.client(ctx, claimName)
	if err != nil {
		return err
	}
	force := true
	for _, s := range bundle.Seed {
		obj := s.Object
		gvr, namespaced, err := content.GVRForKind(obj.GetAPIVersion(), obj.GetKind())
		if err != nil {
			return fmt.Errorf("seed %s doc %d: %w", s.File, s.Doc, err) // unreachable post-validation
		}
		data, err := json.Marshal(obj.Object)
		if err != nil {
			return fmt.Errorf("seed %s doc %d: marshal: %w", s.File, s.Doc, err)
		}
		ri := resourceFor(c.Dynamic, gvr, namespaced, obj.GetNamespace())
		if _, err := ri.Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
			FieldManager: seedFieldManager,
			Force:        &force,
		}); err != nil {
			t.metrics.RecordTenantClientError(ctx)
			return fmt.Errorf("apply %s %s/%s: %w", obj.GetKind(), obj.GetNamespace(), obj.GetName(), err)
		}
	}
	return nil
}

// DeleteSeeded implements TenantOps: removes everything labeled
// kubesandbox.com/challenge=<id> from the tenant, namespaces first (cascade
// does most of the work), then waits for the namespaces to be gone so a
// re-seed never races a terminating namespace (§7 reset).
//
// User-created clutter caveat (§7): objects the user created WITHOUT the
// label persist — acceptable, and arguably correct, for v1.
func (t *tenantOps) DeleteSeeded(ctx context.Context, claimName, challengeID string) error {
	c, err := t.client(ctx, claimName)
	if err != nil {
		return err
	}
	sel := content.ChallengeLabel + "=" + challengeID

	// 1) Labeled namespaces (cascade deletes their contents).
	nsList, err := c.Dynamic.Resource(namespaceGVR).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return fmt.Errorf("list challenge namespaces: %w", err)
	}
	for i := range nsList.Items {
		name := nsList.Items[i].GetName()
		if err := c.Dynamic.Resource(namespaceGVR).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete namespace %s: %w", name, err)
		}
	}

	// 2) Labeled leftovers elsewhere (objects seeded into pre-existing
	// namespaces, or labeled cluster-scoped objects). Deletes racing the
	// namespace cascade just 404 — ignored. Each kind is an independent
	// List+Delete round-trip to the tenant API, so they run concurrently
	// (bounded) instead of one at a time — ~20 known kinds run serially was
	// the slowest part of a reset for no reason.
	if err := t.deleteLabeledLeftovers(ctx, c, sel); err != nil {
		return err
	}

	// 3) Wait for the labeled namespaces to be fully gone.
	for {
		left, err := c.Dynamic.Resource(namespaceGVR).List(ctx, metav1.ListOptions{LabelSelector: sel})
		if err != nil {
			return fmt.Errorf("poll challenge namespaces: %w", err)
		}
		if len(left.Items) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %d challenge namespace(s) to delete", len(left.Items))
		case <-time.After(2 * time.Second):
		}
	}
}

// deleteLabeledLeftovers deletes every labeled object across the known
// resource kinds other than Namespace (handled by the caller). Kinds are
// independent of each other, so they're scanned/deleted with a small bounded
// worker pool rather than one at a time; the first real error (after every
// worker has finished) is returned. A context cancellation/timeout still
// applies per-request via ctx, same as the sequential version.
func (t *tenantOps) deleteLabeledLeftovers(ctx context.Context, c *k8s.TenantClient, sel string) error {
	kinds := content.KnownGVRs()
	sem := make(chan struct{}, deleteFanout)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for _, kk := range kinds {
		if kk.GVR == namespaceGVR {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := deleteLabeledKind(ctx, c, kk, sel); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return firstErr
}

// deleteLabeledKind lists and deletes every object of one kind carrying sel.
// An absent API group (IsNotFound on the list) is fine — not every tenant
// vcluster has every known kind installed.
func deleteLabeledKind(ctx context.Context, c *k8s.TenantClient, kk content.KnownGVR, sel string) error {
	ri := c.Dynamic.Resource(kk.GVR)
	var list *unstructured.UnstructuredList
	var err error
	if kk.Namespaced {
		list, err = ri.Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{LabelSelector: sel})
	} else {
		list, err = ri.List(ctx, metav1.ListOptions{LabelSelector: sel})
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil // API group absent in this tenant — fine
		}
		return fmt.Errorf("list %s: %w", kk.GVR.Resource, err)
	}
	for i := range list.Items {
		o := &list.Items[i]
		di := resourceFor(c.Dynamic, kk.GVR, kk.Namespaced, o.GetNamespace())
		if err := di.Delete(ctx, o.GetName(), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			return fmt.Errorf("delete %s %s/%s: %w", o.GetKind(), o.GetNamespace(), o.GetName(), err)
		}
	}
	return nil
}

// GetObject implements TenantOps (grading read).
func (t *tenantOps) GetObject(ctx context.Context, claimName string, target content.TargetRef) (*unstructured.Unstructured, error) {
	c, err := t.client(ctx, claimName)
	if err != nil {
		return nil, err
	}
	gvr, namespaced, err := content.GVRForKind(target.APIVersion, target.Kind)
	if err != nil {
		return nil, err
	}
	return resourceFor(c.Dynamic, gvr, namespaced, target.Namespace).Get(ctx, target.Name, metav1.GetOptions{})
}

// ListObjects implements TenantOps (grading read).
func (t *tenantOps) ListObjects(ctx context.Context, claimName string, target content.TargetRef) ([]unstructured.Unstructured, error) {
	c, err := t.client(ctx, claimName)
	if err != nil {
		return nil, err
	}
	gvr, namespaced, err := content.GVRForKind(target.APIVersion, target.Kind)
	if err != nil {
		return nil, err
	}
	list, err := resourceFor(c.Dynamic, gvr, namespaced, target.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: target.LabelSelector,
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// CanI implements TenantOps: a SelfSubjectAccessReview issued while
// impersonating the tenant ServiceAccount, so RBAC challenges grade the
// EFFECT of the user's fix, not the object shape (§4). The exported vcluster
// admin identity may impersonate; the impersonated call is a review — the
// grader stays read-only by construction.
func (t *tenantOps) CanI(ctx context.Context, claimName string, sa content.SubjectRef, access content.AccessRef) (bool, error) {
	c, err := t.client(ctx, claimName)
	if err != nil {
		return false, err
	}
	imp, err := c.ImpersonateServiceAccount(sa.Namespace, sa.Name)
	if err != nil {
		t.metrics.RecordTenantClientError(ctx)
		return false, err
	}
	ssar := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "authorization.k8s.io/v1",
		"kind":       "SelfSubjectAccessReview",
		"spec": map[string]interface{}{
			"resourceAttributes": map[string]interface{}{
				"namespace": access.Namespace,
				"verb":      access.Verb,
				"group":     access.APIGroup,
				"resource":  access.Resource,
				"name":      access.Name,
			},
		},
	}}
	res, err := imp.Resource(ssarGVR).Create(ctx, ssar, metav1.CreateOptions{})
	if err != nil {
		t.metrics.RecordTenantClientError(ctx)
		return false, fmt.Errorf("selfsubjectaccessreview as %s/%s: %w", sa.Namespace, sa.Name, err)
	}
	allowed, _, _ := unstructured.NestedBool(res.Object, "status", "allowed")
	return allowed, nil
}

// resourceFor scopes a dynamic resource client to a namespace when the kind
// is namespaced.
func resourceFor(dyn dynamic.Interface, gvr schema.GroupVersionResource, namespaced bool, namespace string) dynamic.ResourceInterface {
	if namespaced {
		return dyn.Resource(gvr).Namespace(namespace)
	}
	return dyn.Resource(gvr)
}

var _ TenantOps = (*tenantOps)(nil)
