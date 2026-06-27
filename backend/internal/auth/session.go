// Package auth provides session cookie signing/verification and OIDC PKCE
// utilities for the backend-owned session auth flow (G2 Option B).
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

// SignSession encodes claims as base64url(JSON) and appends an HMAC-SHA256
// signature, producing a compact token: "<payload>.<sig>".
func SignSession(claims SessionClaims, secret string) (string, error) {
	payload, err := marshalB64(claims)
	if err != nil {
		return "", err
	}
	sig := signHMAC(payload, secret)
	return payload + "." + sig, nil
}

// VerifySession validates the HMAC, checks expiry, and returns the claims.
func VerifySession(token, secret string) (*SessionClaims, error) {
	payload, sig, ok := splitToken(token)
	if !ok {
		return nil, ErrInvalidToken
	}
	if !hmac.Equal([]byte(signHMAC(payload, secret)), []byte(sig)) {
		return nil, ErrInvalidToken
	}
	var claims SessionClaims
	if err := unmarshalB64(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, ErrExpiredToken
	}
	return &claims, nil
}

// StateClaims is the payload stored in the OIDC state parameter (signed JWT-like
// token passed to Authentik and returned on the /oauth2/callback). Stateless:
// the code_verifier and original URL travel in the signed state so no server-side
// storage is required across replicas.
type StateClaims struct {
	CodeVerifier string `json:"cv"`
	OriginalURL  string `json:"url"`
	Exp          int64  `json:"exp"` // Unix timestamp (short-lived, ~5 min)
}

// SignState produces a signed state token for the PKCE authorization request.
func SignState(claims StateClaims, secret string) (string, error) {
	payload, err := marshalB64(claims)
	if err != nil {
		return "", err
	}
	sig := signHMAC(payload, secret)
	return payload + "." + sig, nil
}

// VerifyState validates the HMAC and checks expiry, returning the state claims.
func VerifyState(token, secret string) (*StateClaims, error) {
	payload, sig, ok := splitToken(token)
	if !ok {
		return nil, ErrInvalidToken
	}
	if !hmac.Equal([]byte(signHMAC(payload, secret)), []byte(sig)) {
		return nil, ErrInvalidToken
	}
	var claims StateClaims
	if err := unmarshalB64(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
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
