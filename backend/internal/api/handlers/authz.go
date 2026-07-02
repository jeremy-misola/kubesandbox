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

// AuthzHandler serves the ext-authz (ForwardAuth) endpoint the gateway calls
// for every request to a /s/{id} session route:
//
//  1. Read the session cookie. No/invalid/expired cookie → 302 to Authentik
//     (with PKCE + a signed state carrying the original URL).
//  2. Valid cookie → check the caller owns the session. Owner → 200; non-owner
//     or unknown → 403; backend error → 503 (fail closed).
//
// The flow is stateless: the PKCE code_verifier and original URL travel in the
// signed state that Authentik reflects back on the callback.
type AuthzHandler struct {
	svc *k8s.SessionService
	cfg config.Config
}

// NewAuthzHandler constructs an AuthzHandler.
func NewAuthzHandler(svc *k8s.SessionService, cfg config.Config) *AuthzHandler {
	return &AuthzHandler{svc: svc, cfg: cfg}
}

// forwardedPathHeaders convey the original request URI, in priority order.
var forwardedPathHeaders = []string{
	"X-Forwarded-Uri",
	"X-Original-Uri",
	"X-Envoy-Original-Path",
}

// oauthNonceCookieName holds the per-login-attempt nonce that binds the signed
// OIDC state to the browser that started the flow (CSRF protection). Set here,
// verified and cleared by the /oauth2/callback handler.
const oauthNonceCookieName = "kubesandbox_oauth_nonce"

// oauthNonceMaxAge matches the state token's expiry: the nonce cookie should
// not outlive the login attempt.
const oauthNonceMaxAge = 5 * time.Minute

// Check handles GET /authz and GET /authz/*.
func (h *AuthzHandler) Check(c *gin.Context) {
	origPath := h.originalPath(c)

	cookieVal, err := c.Cookie(h.cfg.SessionCookieName)
	if err != nil || cookieVal == "" {
		h.redirectToLogin(c, origPath)
		return
	}

	claims, err := auth.VerifySession(cookieVal, h.cfg.SessionSecret)
	if err != nil {
		h.redirectToLogin(c, origPath)
		return
	}

	id, ok := extractSessionID(origPath)
	if !ok {
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
		c.Status(http.StatusServiceUnavailable) // fail closed
	}
}

// redirectToLogin generates a PKCE challenge and returns 302 → Authentik. Envoy
// forwards non-2xx ext-authz responses to the browser, which follows it.
func (h *AuthzHandler) redirectToLogin(c *gin.Context, origPath string) {
	codeVerifier, err := auth.GenerateCodeVerifier()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	challenge := auth.CodeChallenge(codeVerifier)

	// The nonce binds this login attempt to the current browser: it travels in
	// the signed state and in a cookie set below. /oauth2/callback requires
	// both copies to match, stopping an attacker from replaying their own
	// state+code pair into a victim's browser (OAuth login CSRF).
	nonce, err := auth.GenerateNonce()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	stateToken, err := auth.SignState(auth.StateClaims{
		CodeVerifier: codeVerifier,
		OriginalURL:  h.cfg.PublicBaseURL + origPath,
		Nonce:        nonce,
		Exp:          time.Now().Add(5 * time.Minute).Unix(),
	}, h.cfg.SessionSecret)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// SameSite=Lax (not Strict): the browser must still send this cookie on
	// Authentik's cross-site top-level navigation back to /oauth2/callback.
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthNonceCookieName,
		Value:    nonce,
		Path:     "/",
		Domain:   h.cfg.SessionCookieDomain,
		MaxAge:   int(oauthNonceMaxAge.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

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

	c.Header("Location", authURL.String())
	c.Status(http.StatusFound)
}

// originalPath determines the client's original request path, preferring the
// forwarded-URI headers a gateway sets, then falling back to this request's
// path with any /authz mount prefix stripped.
func (h *AuthzHandler) originalPath(c *gin.Context) string {
	for _, hdr := range forwardedPathHeaders {
		if v := strings.TrimSpace(c.GetHeader(hdr)); v != "" {
			return v
		}
	}
	return strings.TrimPrefix(c.Request.URL.Path, "/authz")
}

// extractSessionID pulls the id from a path containing an "/s/{id}" segment,
// e.g. "/s/playground-s-1a2b3c4d/token" -> "playground-s-1a2b3c4d".
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
