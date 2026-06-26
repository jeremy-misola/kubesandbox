package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

func TestExtractSessionID(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		wantID string
		wantOK bool
	}{
		{"bare session", "/s/playground-s-1a2b3c4d", "playground-s-1a2b3c4d", true},
		{"trailing slash", "/s/playground-s-1a2b3c4d/", "playground-s-1a2b3c4d", true},
		{"subpath token", "/s/playground-s-1a2b3c4d/token", "playground-s-1a2b3c4d", true},
		{"query string", "/s/playground-s-1a2b3c4d/ws?arg=1", "playground-s-1a2b3c4d", true},
		{"no leading slash", "s/playground-s-1a2b3c4d", "playground-s-1a2b3c4d", true},
		{"empty id", "/s/", "", false},
		{"no session segment", "/api/sessions", "", false},
		{"root", "/", "", false},
		{"empty", "", "", false},
		{"s only as last segment", "/something/s", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := extractSessionID(tc.path)
			if ok != tc.wantOK || id != tc.wantID {
				t.Fatalf("extractSessionID(%q) = (%q, %v), want (%q, %v)",
					tc.path, id, ok, tc.wantID, tc.wantOK)
			}
		})
	}
}

// newClaim builds an unstructured KubeSandboxSession owned by ownerRef.
func newClaim(namespace, name, ownerRef string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": models.APIGroup + "/" + models.APIVersion,
		"kind":       models.Kind,
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"ownerRef":  ownerRef,
			"tenantRef": ownerRef,
		},
	}}
	return u
}

func newAuthzService(objs ...runtime.Object) *k8s.SessionService {
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		models.GVR: models.Kind + "List",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)
	return k8s.NewSessionService(client, "playground", "https://kubesandbox.com", 3, models.DefaultWorkspaceImage)
}

// doAuthz runs the handler with the given identity subject and X-Forwarded-Uri.
func doAuthz(svc *k8s.SessionService, subject, uri string) int {
	gin.SetMode(gin.TestMode)
	cfg := config.Load()
	r := gin.New()
	h := NewAuthzHandler(svc)
	r.GET("/authz", middleware.IdentityMiddleware(cfg), h.Check)

	req := httptest.NewRequest(http.MethodGet, "/authz", nil)
	if subject != "" {
		req.Header.Set("X-User-Id", subject)
	}
	if uri != "" {
		req.Header.Set("X-Forwarded-Uri", uri)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestAuthzDecisionMatrix(t *testing.T) {
	owner := "alice@example.com"
	other := "bob@example.com"
	svc := newAuthzService(newClaim("playground", "s-1a2b3c4d", owner))

	cases := []struct {
		name    string
		subject string
		uri     string
		want    int
	}{
		{"owner allowed", owner, "/s/playground-s-1a2b3c4d", http.StatusOK},
		{"owner allowed subpath", owner, "/s/playground-s-1a2b3c4d/token", http.StatusOK},
		{"non-owner denied", other, "/s/playground-s-1a2b3c4d", http.StatusForbidden},
		{"unknown id denied", owner, "/s/playground-s-deadbeef", http.StatusForbidden},
		{"malformed id denied", owner, "/s/not-a-valid-id", http.StatusForbidden},
		{"no session path denied", owner, "/api/sessions", http.StatusForbidden},
		{"no identity 401", "", "/s/playground-s-1a2b3c4d", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := doAuthz(svc, tc.subject, tc.uri); got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
		})
	}
}
