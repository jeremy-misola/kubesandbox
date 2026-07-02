package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// CreateWarm provisions one unclaimed pool member. It carries no owner
// (tenantRef/ownerRef empty until assignment); pool=available marks it as a
// hand-out candidate once Ready. Called by the pool manager, never on the
// request path.
func (s *SessionService) CreateWarm(ctx context.Context) (*models.Session, error) {
	obj := newClaimObject(s.namespace, warmNamePrefix+randSuffix(),
		map[string]string{
			managedByLabel: managedByValue,
			poolLabel:      poolAvailable,
		}, nil,
		map[string]interface{}{
			"tenantRef":      "",
			"ownerRef":       "",
			"ttlMinutes":     int64(models.DefaultTTLMinutes),
			"workspaceImage": s.defaultImage,
			"resources":      defaultResourcesMap(),
		})

	created, err := s.resource().Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create warm claim: %w", err)
	}
	sess := s.ToSession(created)
	return &sess, nil
}
