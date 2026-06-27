package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/jeremy-misola/kubesandbox/backend/internal/auth"
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

const testSecret = "test-hmac-secret-32-bytes-padding"

// testCfg returns a minimal Config for authz tests with session auth wired up.
func testCfg() config.Config {
	return config.Config{
		PublicBaseURL:     "https://kubesandbox.com",
		SessionSecret:     testSecret,
		SessionCookieName: "kubesandbox_session",
		OIDCAuthEndpoint:  "https://auth.example.com/application/o/authorize/",
		OIDCClientID:      "kubesandbox-backend",
		OIDCRedirectURI:   "https://kubesandbox.com/oauth2/callback",
	}
}

// makeSessionCookie returns a valid signed session cookie value for subject.
func makeSessionCookie(subject string) string {
	tok, _ := auth.SignSession(auth.SessionClaims{
		Subject: subject,
		Email:   subject,
		Exp:     time.Now().Add(1 * time.Hour).Unix(),
	}, testSecret)
	return tok
}

// doAuthz runs the Check handler with the given session cookie value (empty = no
// cookie) and X-Forwarded-Uri, returning the HTTP status code.
func doAuthz(svc *k8s.SessionService, cookieVal, uri string) int {
	gin.SetMode(gin.TestMode)
	cfg := testCfg()
	r := gin.New()
	h := NewAuthzHandler(svc, cfg)
	r.GET("/authz", h.Check)

	req := httptest.NewRequest(http.MethodGet, "/authz", nil)
	if cookieVal != "" {
		req.Header.Set("Cookie", cfg.SessionCookieName+"="+cookieVal)
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

	ownerCookie := makeSessionCookie(owner)
	otherCookie := makeSessionCookie(other)

	cases := []struct {
		name      string
		cookieVal string // empty = no cookie
		uri       string
		want      int
	}{
		// Authenticated owner: allow (200).
		{"owner allowed", ownerCookie, "/s/playground-s-1a2b3c4d", http.StatusOK},
		{"owner allowed subpath", ownerCookie, "/s/playground-s-1a2b3c4d/token", http.StatusOK},
		// Authenticated but not owner: deny (403).
		{"non-owner denied", otherCookie, "/s/playground-s-1a2b3c4d", http.StatusForbidden},
		// Owner but unknown session id: deny (403, no existence leak).
		{"unknown id denied", ownerCookie, "/s/playground-s-deadbeef", http.StatusForbidden},
		// Owner but malformed id: deny (403).
		{"malformed id denied", ownerCookie, "/s/not-a-valid-id", http.StatusForbidden},
		// No /s/{id} in the path: deny (403).
		{"no session path denied", ownerCookie, "/api/sessions", http.StatusForbidden},
		// No cookie: redirect to Authentik (302).
		{"no cookie redirects to login", "", "/s/playground-s-1a2b3c4d", http.StatusFound},
		// Tampered cookie: redirect to login (302).
		{"tampered cookie redirects", ownerCookie + "TAMPERED", "/s/playground-s-1a2b3c4d", http.StatusFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := doAuthz(svc, tc.cookieVal, tc.uri); got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestAuthzLoginRedirectHasLocation verifies that a missing-cookie response
// includes a Location header pointing to the OIDC authorization endpoint.
func TestAuthzLoginRedirectHasLocation(t *testing.T) {
	svc := newAuthzService()
	gin.SetMode(gin.TestMode)
	cfg := testCfg()
	r := gin.New()
	r.GET("/authz", NewAuthzHandler(svc, cfg).Check)

	req := httptest.NewRequest(http.MethodGet, "/authz", nil)
	req.Header.Set("X-Forwarded-Uri", "/s/playground-s-1a2b3c4d")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header, got none")
	}
	if !containsString(loc, cfg.OIDCAuthEndpoint) {
		t.Fatalf("Location %q does not contain OIDC auth endpoint %q", loc, cfg.OIDCAuthEndpoint)
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
