package config

import (
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	// Server configuration
	Port string

	// Kubernetes configuration
	Namespace string // Namespace where sessions are created (default: playground)

	// Headers configuration (injected by Envoy Gateway)
	UserEmailHeader  string
	UserNameHeader   string
	UserGroupsHeader string

	// TTL cleanup interval in minutes
	TTLCleanupInterval int
}

// Load reads configuration from environment variables
func Load() *Config {
	return &Config{
		Port:               getEnv("PORT", "8080"),
		Namespace:          getEnv("NAMESPACE", "playground"),
		UserEmailHeader:    getEnv("USER_EMAIL_HEADER", "X-User-Email"),
		UserNameHeader:     getEnv("USER_NAME_HEADER", "X-User-Name"),
		UserGroupsHeader:   getEnv("USER_GROUPS_HEADER", "X-User-Groups"),
		TTLCleanupInterval: getEnvInt("TTL_CLEANUP_INTERVAL", 1),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
