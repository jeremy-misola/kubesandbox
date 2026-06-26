// Package config loads runtime configuration from environment variables.
//
// All values are injected by the kubesandbox-backend Helm chart (see
// kubesandbox-charts/kubesandbox-backend/values.yaml -> .Values.config). Every
// option has a safe default so the binary also runs locally with no env set.
package config

import (
	"os"
	"strconv"
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

	// Identity headers injected by Envoy Gateway after edge OIDC (G1).
	UserEmailHeader  string
	UserNameHeader   string
	UserGroupsHeader string
	UserIDHeader     string

	// TTLCleanupInterval is reserved for the G3 TTL loop (not run in G1).
	TTLCleanupInterval time.Duration
	// MaxSessionsPerUser caps concurrent sessions per owner.
	MaxSessionsPerUser int
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

// Load reads configuration from the environment, applying defaults.
func Load() Config {
	c := Config{
		Port:             getenv("PORT", "8080"),
		Namespace:        getenv("NAMESPACE", "playground"),
		PublicBaseURL:    getenv("PUBLIC_BASE_URL", "https://kubesandbox.com"),
		UserEmailHeader:  getenv("USER_EMAIL_HEADER", "X-User-Email"),
		UserNameHeader:   getenv("USER_NAME_HEADER", "X-User-Name"),
		UserGroupsHeader: getenv("USER_GROUPS_HEADER", "X-User-Groups"),
		UserIDHeader:     getenv("USER_ID_HEADER", "X-User-Id"),

		TTLCleanupInterval: time.Duration(getenvInt("TTL_CLEANUP_INTERVAL", 1)) * time.Minute,
		MaxSessionsPerUser: getenvInt("MAX_SESSIONS_PER_USER", 3),
	}
	return c
}
