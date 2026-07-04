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

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// Config is the fully-resolved backend configuration.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port string
	// Namespace is where KubeSandboxSession claims are created and read.
	Namespace string
	// WorkspaceImage is the ttyd shell image stamped onto every session claim.
	// Injected as WORKSPACE_IMAGE by the Helm chart (.Values.workspaceImage) so
	// the tag lives in one place; falls back to models.DefaultWorkspaceImage.
	WorkspaceImage string
	// PublicBaseURL is the externally reachable origin used to build session
	// URLs, e.g. https://kubesandbox.com -> https://kubesandbox.com/s/{id}.
	PublicBaseURL string

	// Identity headers injected by Envoy Gateway after edge OIDC. Used by /api;
	// /authz uses the session cookie instead.
	UserEmailHeader  string
	UserNameHeader   string
	UserGroupsHeader string
	UserIDHeader     string

	// TTLCleanupInterval is the interval of the server-side TTL cleanup loop.
	TTLCleanupInterval time.Duration

	// --- Hot warm-pool ---

	// PoolEnabled turns the hot pool + assignment path on. When false the
	// backend falls back to legacy direct creates (cold build on request).
	PoolEnabled bool
	// PoolTargetWarm is how many hot, unclaimed sandboxes to keep Ready.
	PoolTargetWarm int
	// PoolMaxTotal is the concurrent-session ceiling (warm + live).
	PoolMaxTotal int
	// PoolMaxWarmAge is how old an unclaimed member may get before it is
	// recycled instead of handed out.
	PoolMaxWarmAge time.Duration
	// PoolResync is the pool manager's periodic reconcile interval.
	PoolResync time.Duration

	// --- Backend-owned session auth ---

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

// getenvBool parses a boolean env var, falling back to def when unset or
// unparseable.
func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// hostOf extracts the hostname (without port) from a URL string, returning ""
// on error.
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
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
		WorkspaceImage:   getenv("WORKSPACE_IMAGE", models.DefaultWorkspaceImage),
		PublicBaseURL:    publicBaseURL,
		UserEmailHeader:  getenv("USER_EMAIL_HEADER", "X-User-Email"),
		UserNameHeader:   getenv("USER_NAME_HEADER", "X-User-Name"),
		UserGroupsHeader: getenv("USER_GROUPS_HEADER", "X-User-Groups"),
		UserIDHeader:     getenv("USER_ID_HEADER", "X-User-Id"),

		TTLCleanupInterval: time.Duration(getenvInt("TTL_CLEANUP_INTERVAL", 1)) * time.Minute,

		PoolEnabled:    getenvBool("POOL_ENABLED", true),
		PoolTargetWarm: getenvInt("POOL_TARGET_WARM", 2),
		PoolMaxTotal:   getenvInt("POOL_MAX_TOTAL", 60),
		PoolMaxWarmAge: time.Duration(getenvInt("POOL_MAX_WARM_AGE_HOURS", 24)) * time.Hour,
		PoolResync:     time.Duration(getenvInt("POOL_RESYNC_SECONDS", 30)) * time.Second,

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
