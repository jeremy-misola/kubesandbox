// Package auth provides session cookie signing/verification and OIDC PKCE
// utilities for the backend-owned session auth flow.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ErrInvalidToken is returned when a signed token cannot be verified.
var ErrInvalidToken = errors.New("invalid or tampered token")

// ErrExpiredToken is returned when a token's exp claim is in the past.
var ErrExpiredToken = errors.New("token has expired")

// SessionClaims is the payload stored in the browser session cookie.
type SessionClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Exp     int64  `json:"exp"` // Unix timestamp
}

func (c SessionClaims) expiry() int64 { return c.Exp }

// SignSession encodes claims as base64url(JSON) and appends an HMAC-SHA256
// signature, producing a compact token: "<payload>.<sig>".
func SignSession(claims SessionClaims, secret string) (string, error) {
	return signToken(claims, secret)
}

// VerifySession validates the HMAC, checks expiry, and returns the claims.
func VerifySession(token, secret string) (*SessionClaims, error) {
	return verifyToken[SessionClaims](token, secret)
}

// StateClaims is the payload stored in the OIDC state parameter (signed JWT-like
// token passed to Authentik and returned on the /oauth2/callback). Stateless:
// the code_verifier and original URL travel in the signed state so no server-side
// storage is required across replicas.
//
// Nonce binds this state to the browser that initiated the login: it is also
// set as a short-lived cookie when the redirect to Authentik is issued, and
// the callback must see the same value in both places before trusting the
// state. Without this, the signed state is self-contained and portable — an
// attacker who completes their own login can hand their valid code+state pair
// to a victim (e.g. via a crafted link to /oauth2/callback) and get the
// victim's browser silently logged in as the attacker (OAuth "login CSRF").
// The nonce cookie can only have been set by the victim's own prior request
// to /authz, so a mismatch means the callback wasn't triggered by the same
// browser that started the flow.
type StateClaims struct {
	CodeVerifier string `json:"cv"`
	OriginalURL  string `json:"url"`
	Nonce        string `json:"n"`
	Exp          int64  `json:"exp"` // Unix timestamp (short-lived, ~5 min)
}

func (c StateClaims) expiry() int64 { return c.Exp }

// SignState produces a signed state token for the PKCE authorization request.
func SignState(claims StateClaims, secret string) (string, error) {
	return signToken(claims, secret)
}

// VerifyState validates the HMAC and checks expiry, returning the state claims.
func VerifyState(token, secret string) (*StateClaims, error) {
	return verifyToken[StateClaims](token, secret)
}

// --- signing core ---

// expirable is satisfied by any claims type carrying an exp timestamp.
type expirable interface{ expiry() int64 }

// signToken encodes claims as base64url(JSON) and appends an HMAC-SHA256
// signature, producing a compact token: "<payload>.<sig>".
func signToken[T any](claims T, secret string) (string, error) {
	payload, err := marshalB64(claims)
	if err != nil {
		return "", err
	}
	return payload + "." + signHMAC(payload, secret), nil
}

// verifyToken validates the HMAC, checks expiry, and returns the decoded claims.
func verifyToken[T expirable](token, secret string) (*T, error) {
	payload, sig, ok := splitToken(token)
	if !ok {
		return nil, ErrInvalidToken
	}
	if !hmac.Equal([]byte(signHMAC(payload, secret)), []byte(sig)) {
		return nil, ErrInvalidToken
	}
	var claims T
	if err := unmarshalB64(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if exp := claims.expiry(); exp > 0 && time.Now().Unix() > exp {
		return nil, ErrExpiredToken
	}
	return &claims, nil
}

// --- helpers ---

func signHMAC(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func marshalB64(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func unmarshalB64(s string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func splitToken(token string) (payload, sig string, ok bool) {
	i := strings.LastIndexByte(token, '.')
	if i < 0 {
		return "", "", false
	}
	return token[:i], token[i+1:], true
}
