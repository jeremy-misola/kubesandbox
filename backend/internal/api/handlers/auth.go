package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jeremy-misola/kubesandbox/backend/internal/auth"
	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
)

// AuthCallbackHandler handles the OIDC authorization code callback at
// /oauth2/callback (G2 Option B).
//
// Flow:
//  1. Authentik redirects here after the user logs in, with ?code=...&state=...
//  2. Verify the signed state token (proves it was us that started the flow,
//     contains the code_verifier and the original /s/{id}/... URL).
//  3. Exchange the code at Authentik's token endpoint (confidential client +
//     PKCE; server-to-server over TLS).
//  4. Parse the returned ID token (no JWKS validation needed — the token came
//     directly from Authentik over TLS, so the transport is the trust anchor).
//  5. Sign a session cookie with the user's identity (sub, email, name).
//  6. Redirect the browser back to the original /s/{id}/... URL.
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
	errParam := c.Query("error")

	// Authentik may return an error (e.g. access_denied).
	if errParam != "" {
		errDesc := c.Query("error_description")
		log.Printf("auth callback: provider error: %s: %s", errParam, errDesc)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   errParam,
			"message": errDesc,
		})
		return
	}

	if stateToken == "" || code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "bad_request",
			"message": "missing state or code parameter",
		})
		return
	}

	// --- Step 1: Verify state token ---
	stateClaims, err := auth.VerifyState(stateToken, h.cfg.SessionSecret)
	if err != nil {
		log.Printf("auth callback: invalid state token: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_state",
			"message": "state token is invalid or expired — please try again",
		})
		return
	}

	// --- Step 2: Exchange code for tokens ---
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
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "token_exchange_failed",
			"message": "could not exchange authorization code",
		})
		return
	}

	// --- Step 3: Parse ID token claims ---
	idClaims, err := auth.ParseIDTokenClaims(tokenResp.IDToken)
	if err != nil {
		log.Printf("auth callback: parse id_token failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "id_token_parse_failed",
			"message": "could not read identity from provider token",
		})
		return
	}

	// --- Step 4: Sign session cookie ---
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

	// --- Step 5: Set cookie + redirect ---
	// HttpOnly: JS cannot read it (XSS protection).
	// Secure: only sent over HTTPS.
	// SameSite=Lax: cookie follows top-level navigations (needed for the
	// post-login redirect from Authentik) but not cross-site AJAX.
	maxAgeSecs := int(h.cfg.SessionMaxAge.Seconds())
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		Domain:   h.cfg.SessionCookieDomain,
		MaxAge:   maxAgeSecs,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect back to the original URL the user was trying to reach.
	originalURL := stateClaims.OriginalURL
	if originalURL == "" {
		originalURL = h.cfg.PublicBaseURL + "/"
	}
	c.Redirect(http.StatusFound, originalURL)
}
