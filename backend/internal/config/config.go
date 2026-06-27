// Package config loads runtime configuration from environment variables.
//
// All values are injected by the kubesandbox-backend Helm chart (see
// kubesandbox-charts/kubesandbox-backend/values.yaml -> .Values.config). Every
// option has a safe default so the binary also runs locally with no env set.
package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved backend configuration.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port string
	// Namespace is where KubeSandboxSession claims are created and read.
	Namespace string
	// PublicBaseURL is the externally reachable origin used to build session
	// URLs, e.g. https://kubesandbox.com -> https://kubesandbox.com/s/{id}.
	PublicBaseURL string

	// Identity headers injected by Envoy Gateway after edge OIDC/JWT (G1/G4).
	// Still used by /api; not used by /authz in G2 Option B (cookie-based).
	UserEmailHeader  string
	UserNameHeader   string
	UserGroupsHeader string
	UserIDHeader     string

	// TTLCleanupInterval is reserved for the G3 TTL loop.
	TTLCleanupInterval time.Duration
	// MaxSessionsPerUser caps concurrent sessions per owner.
	MaxSessionsPerUser int

	// --- G2 Option B: backend-owned session auth ---

	// OIDCIssuer is the Authentik provider issuer URL
	// (e.g. https://auth.jeremymr.dev/application/o/kubesandbox-backend/).
	OIDCIssuer string
	// OIDCClientID is the OAuth2 client_id registered in Authentik.
	OIDCClientID string
	// OIDCClientSecret is the OAuth2 client_secret (injected from a K8s Secret).
	OIDCClientSecret string
	// OIDCRedirectURI is the callback URL registered in Authentik and sent with
	// every authorization request (e.g. https://kubesandbox.com/oauth2/callback).
	OIDCRedirectURI string
	// OIDCAuthEndpoint is the Authentik authorization endpoint. Defaults to
	// OIDCIssuer + "authorize/" if not set explicitly.
	OIDCAuthEndpoint string
	// OIDCTokenEndpoint is the token endpoint used for the code exchange. Defaults
	// to OIDCIssuer + "token/" if not set explicitly.
	OIDCTokenEndpoint string

	// SessionSecret is the HMAC-SHA256 key used to sign and verify session
	// cookies and PKCE state tokens. Must be set when sessionAuth.enabled.
	SessionSecret string
	// SessionCookieName is the name of the browser session cookie.
	// Default: "kubesandbox_session".
	SessionCookieName string
	// SessionCookieDomain is the cookie Domain attribute. Defaults to the host
	// component of PublicBaseURL.
	SessionCookieDomain string
	// SessionMaxAge is how long the session cookie is valid. Default: 8 hours.
	SessionMaxAge time.Duration
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// hostOf extracts the hostname from a URL string, returning "" on error.
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.Split(u.Host, ":")[0] // strip port if present
}

// Load reads configuration from the environment, applying defaults.
func Load() Config {
	publicBaseURL := getenv("PUBLIC_BASE_URL", "https://kubesandbox.com")
	oidcIssuer := getenv("OIDC_ISSUER", "")
	sessionCookieDomain := getenv("SESSION_COOKIE_DOMAIN", hostOf(publicBaseURL))

	// Derive OIDC endpoints from the issuer if not overridden.
	oidcAuthEndpoint := getenv("OIDC_AUTH_ENDPOINT", "")
	if oidcAuthEndpoint == "" && oidcIssuer != "" {
		oidcAuthEndpoint = strings.TrimRight(oidcIssuer, "/") + "/authorize/"
	}
	oidcTokenEndpoint := getenv("OIDC_TOKEN_ENDPOINT", "")
	if oidcTokenEndpoint == "" && oidcIssuer != "" {
		oidcTokenEndpoint = strings.TrimRight(oidcIssuer, "/") + "/token/"
	}

	sessionMaxAgeSecs := getenvInt("SESSION_MAX_AGE_SECONDS", 8*3600)

	return Config{
		Port:             getenv("PORT", "8080"),
		Namespace:        getenv("NAMESPACE", "playground"),
		PublicBaseURL:    publicBaseURL,
		UserEmailHeader:  getenv("USER_EMAIL_HEADER", "X-User-Email"),
		UserNameHeader:   getenv("USER_NAME_HEADER", "X-User-Name"),
		UserGroupsHeader: getenv("USER_GROUPS_HEADER", "X-User-Groups"),
		UserIDHeader:     getenv("USER_ID_HEADER", "X-User-Id"),

		TTLCleanupInterval: time.Duration(getenvInt("TTL_CLEANUP_INTERVAL", 1)) * time.Minute,
		MaxSessionsPerUser: getenvInt("MAX_SESSIONS_PER_USER", 3),

		OIDCIssuer:        oidcIssuer,
		OIDCClientID:      getenv("OIDC_CLIENT_ID", ""),
		OIDCClientSecret:  getenv("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURI:   getenv("OIDC_REDIRECT_URI", ""),
		OIDCAuthEndpoint:  oidcAuthEndpoint,
		OIDCTokenEndpoint: oidcTokenEndpoint,

		SessionSecret:       getenv("SESSION_SECRET", ""),
		SessionCookieName:   getenv("SESSION_COOKIE_NAME", "kubesandbox_session"),
		SessionCookieDomain: sessionCookieDomain,
		SessionMaxAge:       time.Duration(sessionMaxAgeSecs) * time.Second,
	}
}
