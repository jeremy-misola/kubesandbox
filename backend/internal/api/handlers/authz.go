package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jeremy-misola/kubesandbox/backend/internal/auth"
	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
)

// AuthzHandler serves the ext-authz (ForwardAuth) endpoint used by the
// per-session SecurityPolicy (G2 Option B).
//
// The gateway calls /authz for every request to a /s/{id} session route. The
// handler drives the full session auth flow:
//
//  1. Read the session cookie from the forwarded Cookie header.
//  2. No cookie / invalid / expired → generate PKCE, sign a state token, and
//     return 302 to Authentik. Envoy forwards the redirect to the browser; the
//     browser logs in and returns to /oauth2/callback.
//  3. Valid cookie → check that the caller (cookie.sub) owns the session (id).
//     Owner → 200 (allow). Non-owner / unknown → 403. Backend error → 503
//     (fail closed).
//
// This approach is stateless (no server-side session store): the PKCE
// code_verifier and original URL travel in the signed state parameter that
// Authentik reflects back on the callback.
type AuthzHandler struct {
	svc *k8s.SessionService
	cfg config.Config
}

// NewAuthzHandler constructs an AuthzHandler.
func NewAuthzHandler(svc *k8s.SessionService, cfg config.Config) *AuthzHandler {
	return &AuthzHandler{svc: svc, cfg: cfg}
}

// forwardedPathHeaders are the headers an upstream proxy may use to convey the
// original request URI, checked in priority order.
var forwardedPathHeaders = []string{
	"X-Forwarded-Uri",
	"X-Original-Uri",
	"X-Envoy-Original-Path",
}

// Check handles GET /authz and GET /authz/*.
func (h *AuthzHandler) Check(c *gin.Context) {
	// Resolve the original path the browser was requesting (/s/{id}/...).
	origPath := h.originalPath(c)

	// --- Step 1: Read and validate the session cookie ---
	cookieVal, err := c.Cookie(h.cfg.SessionCookieName)
	if err != nil || cookieVal == "" {
		h.redirectToLogin(c, origPath)
		return
	}

	claims, err := auth.VerifySession(cookieVal, h.cfg.SessionSecret)
	if err != nil {
		// Cookie invalid or expired — send the user through login again.
		h.redirectToLogin(c, origPath)
		return
	}

	// --- Step 2: Ownership check ---
	id, ok := extractSessionID(origPath)
	if !ok {
		// No /s/{id} in the path; deny by default.
		c.Status(http.StatusForbidden)
		return
	}

	switch err := h.svc.Authorize(c.Request.Context(), id, claims.Subject); {
	case err == nil:
		c.Status(http.StatusOK)
	case errors.Is(err, k8s.ErrNotFound), errors.Is(err, k8s.ErrInvalidID):
		// Unknown, unowned, and malformed ids are indistinguishable (no leak).
		c.Status(http.StatusForbidden)
	default:
		// Unexpected backend error: fail closed.
		c.Status(http.StatusServiceUnavailable)
	}
}

// redirectToLogin generates a PKCE challenge and returns 302 → Authentik.
// Envoy forwards the 302 to the browser, which follows the redirect to log in.
func (h *AuthzHandler) redirectToLogin(c *gin.Context, origPath string) {
	// Generate PKCE verifier + S256 challenge.
	codeVerifier, err := auth.GenerateCodeVerifier()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	challenge := auth.CodeChallenge(codeVerifier)

	// The original URL is stored in the signed state so no server-side storage
	// is needed. After login, /oauth2/callback redirects back here.
	originalURL := h.cfg.PublicBaseURL + origPath
	stateToken, err := auth.SignState(auth.StateClaims{
		CodeVerifier: codeVerifier,
		OriginalURL:  originalURL,
		Exp:          time.Now().Add(5 * time.Minute).Unix(),
	}, h.cfg.SessionSecret)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// Build the Authentik authorization URL.
	authURL, err := url.Parse(h.cfg.OIDCAuthEndpoint)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", h.cfg.OIDCClientID)
	q.Set("redirect_uri", h.cfg.OIDCRedirectURI)
	q.Set("scope", "openid email profile")
	q.Set("state", stateToken)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	authURL.RawQuery = q.Encode()

	// Return 302 — Envoy ext-authz forwards non-2xx responses to the client,
	// so the browser will follow this redirect to the Authentik login page.
	c.Header("Location", authURL.String())
	c.Status(http.StatusFound)
}

// originalPath determines the client's original request path. It prefers the
// forwarded-URI headers a gateway sets, then falls back to this request's own
// path with any /authz mount prefix stripped.
func (h *AuthzHandler) originalPath(c *gin.Context) string {
	for _, hdr := range forwardedPathHeaders {
		if v := strings.TrimSpace(c.GetHeader(hdr)); v != "" {
			return v
		}
	}
	return strings.TrimPrefix(c.Request.URL.Path, "/authz")
}

// extractSessionID pulls the session id from a path containing an "/s/{id}"
// segment, e.g. "/s/playground-s-1a2b3c4d/token" -> "playground-s-1a2b3c4d".
// It returns ok=false when no such segment is present.
func extractSessionID(path string) (string, bool) {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "s" && segs[i+1] != "" {
			return segs[i+1], true
		}
	}
	return "", false
}

