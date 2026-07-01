package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jeremy-misola/kubesandbox/backend/internal/auth"
	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
)

// fakeIDToken builds a syntactically valid (but unsigned) JWT-shaped string
// carrying the given subject in its payload, matching what
// auth.ParseIDTokenClaims expects (three dot-separated base64url segments;
// signature is never checked by that function).
func fakeIDToken(t *testing.T, sub string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, err := json.Marshal(map[string]any{"sub": sub, "email": sub, "name": "Test User"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

// newCallbackTestServer starts a fake Authentik token endpoint returning a
// fixed subject's ID token, and a Config wired to use it.
func newCallbackTestServer(t *testing.T, sub string) (config.Config, func()) {
	t.Helper()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at",
			"id_token":     fakeIDToken(t, sub),
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	cfg := config.Config{
		PublicBaseURL:       "https://kubesandbox.com",
		SessionSecret:       testSecret,
		SessionCookieName:   "kubesandbox_session",
		SessionCookieDomain: "kubesandbox.com",
		SessionMaxAge:       8 * time.Hour,
		OIDCTokenEndpoint:   srv.URL,
		OIDCClientID:        "kubesandbox-backend",
		OIDCClientSecret:    "shh",
		OIDCRedirectURI:     "https://kubesandbox.com/oauth2/callback",
	}
	return cfg, func() {
		srv.Close()
		_ = called
	}
}

// doCallback runs the Callback handler with the given state/code query params
// and an optional nonce cookie, returning the recorded response.
func doCallback(cfg config.Config, state, code, nonceCookieVal string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAuthCallbackHandler(cfg)
	r.GET("/oauth2/callback", h.Callback)

	url := "/oauth2/callback?state=" + state + "&code=" + code
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if nonceCookieVal != "" {
		req.Header.Set("Cookie", oauthNonceCookieName+"="+nonceCookieVal)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCallbackRejectsMissingNonceCookie(t *testing.T) {
	cfg, cleanup := newCallbackTestServer(t, "alice-sub")
	defer cleanup()

	state, err := auth.SignState(auth.StateClaims{
		CodeVerifier: "verifier",
		OriginalURL:  "https://kubesandbox.com/s/playground-s-1a2b3c4d",
		Nonce:        "the-real-nonce",
		Exp:          time.Now().Add(5 * time.Minute).Unix(),
	}, cfg.SessionSecret)
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}

	// No nonce cookie sent at all.
	w := doCallback(cfg, state, "authcode", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (missing nonce cookie should be rejected)", w.Code, http.StatusBadRequest)
	}
}

func TestCallbackRejectsMismatchedNonceCookie(t *testing.T) {
	cfg, cleanup := newCallbackTestServer(t, "alice-sub")
	defer cleanup()

	state, err := auth.SignState(auth.StateClaims{
		CodeVerifier: "verifier",
		OriginalURL:  "https://kubesandbox.com/s/playground-s-1a2b3c4d",
		Nonce:        "the-real-nonce",
		Exp:          time.Now().Add(5 * time.Minute).Unix(),
	}, cfg.SessionSecret)
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}

	// This is the attack scenario: an attacker hands a victim a valid
	// state+code pair from the attacker's own login attempt. The victim's
	// browser has no cookie (or a cookie from its own, unrelated, in-flight
	// login), so the nonce in the cookie won't match the nonce baked into the
	// attacker's state token.
	w := doCallback(cfg, state, "authcode", "attacker-controlled-or-unrelated-nonce")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (mismatched nonce cookie should be rejected)", w.Code, http.StatusBadRequest)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err == nil {
		if body["error"] != "invalid_state" {
			t.Fatalf("error = %q, want %q", body["error"], "invalid_state")
		}
	}
}

func TestCallbackAcceptsMatchingNonceCookie(t *testing.T) {
	cfg, cleanup := newCallbackTestServer(t, "alice-sub")
	defer cleanup()

	state, err := auth.SignState(auth.StateClaims{
		CodeVerifier: "verifier",
		OriginalURL:  "https://kubesandbox.com/s/playground-s-1a2b3c4d",
		Nonce:        "the-real-nonce",
		Exp:          time.Now().Add(5 * time.Minute).Unix(),
	}, cfg.SessionSecret)
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}

	w := doCallback(cfg, state, "authcode", "the-real-nonce")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d (matching nonce should complete login)", w.Code, http.StatusFound)
	}
	if loc := w.Header().Get("Location"); loc != "https://kubesandbox.com/s/playground-s-1a2b3c4d" {
		t.Fatalf("Location = %q, want original URL", loc)
	}

	// A session cookie must have been set on success.
	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name == cfg.SessionCookieName {
			found = true
		}
	}
	if !found {
		t.Fatal("expected session cookie to be set on successful callback")
	}
}

func TestCallbackClearsNonceCookieOnEveryAttempt(t *testing.T) {
	cfg, cleanup := newCallbackTestServer(t, "alice-sub")
	defer cleanup()

	state, err := auth.SignState(auth.StateClaims{
		CodeVerifier: "verifier",
		OriginalURL:  "https://kubesandbox.com/s/playground-s-1a2b3c4d",
		Nonce:        "the-real-nonce",
		Exp:          time.Now().Add(5 * time.Minute).Unix(),
	}, cfg.SessionSecret)
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}

	// Even a failed (mismatched) attempt must clear the one-time nonce cookie
	// so it can't be reused.
	w := doCallback(cfg, state, "authcode", "wrong-nonce")
	cleared := false
	for _, c := range w.Result().Cookies() {
		if c.Name == oauthNonceCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("expected nonce cookie to be cleared (MaxAge < 0) even on a rejected callback")
	}
}
