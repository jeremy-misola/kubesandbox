package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
	"github.com/jeremy-misola/kubesandbox/backend/pkg/apierror"
)

// contextKey is the type for context keys
type contextKey string

const (
	// UserEmailKey is the context key for user email
	UserEmailKey contextKey = "userEmail"
	// UserNameKey is the context key for user name
	UserNameKey contextKey = "userName"
	// UserGroupsKey is the context key for user groups
	UserGroupsKey contextKey = "userGroups"
)

// User represents the authenticated user
type User struct {
	Email  string
	Name   string
	Groups []string
}

// Auth extracts user information from headers injected by Envoy Gateway
func Auth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health check and other public endpoints
			if r.URL.Path == "/health" || r.URL.Path == "/ready" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			email := r.Header.Get(cfg.UserEmailHeader)
			name := r.Header.Get(cfg.UserNameHeader)
			groupsHeader := r.Header.Get(cfg.UserGroupsHeader)

			// For development, allow mock user via query param or header
			if email == "" {
				email = r.Header.Get("X-Mock-User-Email")
			}
			if name == "" {
				name = r.Header.Get("X-Mock-User-Name")
			}

			// If still no email, check for development mode
			if email == "" {
				// In development mode, create a mock user
				if devMode := r.Header.Get("X-Development-Mode"); devMode == "true" {
					email = "dev@example.com"
					name = "Developer"
					slog.Debug("Using development mode user")
				} else {
					apierror.WriteError(w, apierror.NewUnauthorized("missing user authentication headers"))
					return
				}
			}

			// Parse groups (comma-separated)
			var groups []string
			if groupsHeader != "" {
				groups = strings.Split(groupsHeader, ",")
				for i, g := range groups {
					groups[i] = strings.TrimSpace(g)
				}
			}

			// Add user info to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, UserEmailKey, email)
			ctx = context.WithValue(ctx, UserNameKey, name)
			ctx = context.WithValue(ctx, UserGroupsKey, groups)

			slog.Debug("Authenticated user", "email", email, "name", name, "groups", groups)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUser extracts the user from the request context
func GetUser(r *http.Request) *User {
	ctx := r.Context()
	email, _ := ctx.Value(UserEmailKey).(string)
	name, _ := ctx.Value(UserNameKey).(string)
	groups, _ := ctx.Value(UserGroupsKey).([]string)

	if email == "" {
		return nil
	}

	return &User{
		Email:  email,
		Name:   name,
		Groups: groups,
	}
}

// GetUserID returns a unique identifier for the user (email or name)
func GetUserID(r *http.Request) string {
	user := GetUser(r)
	if user == nil {
		return ""
	}
	if user.Email != "" {
		return user.Email
	}
	return user.Name
}
