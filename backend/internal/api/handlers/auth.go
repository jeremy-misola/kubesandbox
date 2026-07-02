package handlers

import (
	"crypto/subtle"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jeremy-misola/kubesandbox/backend/internal/auth"
	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
)

// AuthCallbackHandler handles the OIDC authorization code callback at
// /oauth2/callback: verify the signed state, verify the nonce cookie (CSRF
// binding), exchange the code for tokens, and set a signed session cookie
// before redirecting back to the original /s/{id} URL.
type AuthCallbackHandler struct {
	cfg config.Config
}

// NewAuthCallbackHandler constructs an AuthCallbackHandler.
func NewAuthCallbackHandler(cfg config.Config) *AuthCallbackHandler {
	return &AuthCallbackHandler{cfg: cfg}
}

// Callback handles GET /oauth2/callback.
func (h *AuthCallbackHandler) Callback(c *gin.Context) {
	stateToken := c.Query("state")
	code := c.Query("code")

	if errParam := c.Query("error"); errParam != "" {
		errDesc := c.Query("error_description")
		log.Printf("auth callback: provider error: %s: %s", errParam, errDesc)
		respondError(c, http.StatusBadRequest, errParam, errDesc)
		return
	}
	if stateToken == "" || code == "" {
		respondError(c, http.StatusBadRequest, "bad_request", "missing state or code parameter")
		return
	}

	stateClaims, err := auth.VerifyState(stateToken, h.cfg.SessionSecret)
	if err != nil {
		log.Printf("auth callback: invalid state token: %v", err)
		respondError(c, http.StatusBadRequest, "invalid_state", "state token is invalid or expired — please try again")
		return
	}

	// Verify the nonce cookie matches the state. The nonce cookie can only be
	// present if this browser is the one /authz redirected, so a missing or
	// mismatched nonce means the callback wasn't triggered by the browser that
	// started the login. Cleared unconditionally so it can't be replayed.
	clearNonceCookie(c, h.cfg.SessionCookieDomain)
	nonceCookie, err := c.Cookie(oauthNonceCookieName)
	if err != nil || nonceCookie == "" ||
		subtle.ConstantTimeCompare([]byte(nonceCookie), []byte(stateClaims.Nonce)) != 1 {
		log.Printf("auth callback: nonce mismatch or missing (cookie present: %v)", err == nil)
		respondError(c, http.StatusBadRequest, "invalid_state", "login session could not be verified — please try again")
		return
	}

	tokenResp, err := auth.ExchangeCode(
		c.Request.Context(),
		h.cfg.OIDCTokenEndpoint,
		h.cfg.OIDCClientID,
		h.cfg.OIDCClientSecret,
		code,
		stateClaims.CodeVerifier,
		h.cfg.OIDCRedirectURI,
	)
	if err != nil {
		log.Printf("auth callback: code exchange failed: %v", err)
		respondError(c, http.StatusBadGateway, "token_exchange_failed", "could not exchange authorization code")
		return
	}

	idClaims, err := auth.ParseIDTokenClaims(tokenResp.IDToken)
	if err != nil {
		log.Printf("auth callback: parse id_token failed: %v", err)
		respondError(c, http.StatusBadGateway, "id_token_parse_failed", "could not read identity from provider token")
		return
	}

	sessionToken, err := auth.SignSession(auth.SessionClaims{
		Subject: idClaims.Sub,
		Email:   idClaims.Email,
		Name:    idClaims.Name,
		Exp:     time.Now().Add(h.cfg.SessionMaxAge).Unix(),
	}, h.cfg.SessionSecret)
	if err != nil {
		log.Printf("auth callback: sign session: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// HttpOnly (JS can't read it), Secure (HTTPS only), SameSite=Lax (follows
	// the post-login top-level redirect, not cross-site AJAX).
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		Domain:   h.cfg.SessionCookieDomain,
		MaxAge:   int(h.cfg.SessionMaxAge.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	originalURL := stateClaims.OriginalURL
	if originalURL == "" {
		originalURL = h.cfg.PublicBaseURL + "/"
	}
	c.Redirect(http.StatusFound, originalURL)
}

// clearNonceCookie deletes the one-time login nonce cookie so it can't be
// reused for a retried or replayed callback.
func clearNonceCookie(c *gin.Context, domain string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthNonceCookieName,
		Value:    "",
		Path:     "/",
		Domain:   domain,
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
