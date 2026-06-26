// Package middleware contains Gin middleware for the backend API.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
)

// contextKey is the gin context key under which the resolved identity is stored.
const contextKey = "kubesandbox.identity"

// Identity is the authenticated caller, derived from Envoy-forwarded headers
// (G1). Subject is used as both ownerRef and tenantRef (1 tenant = 1 user).
type Identity struct {
	Subject string
	Email   string
	Name    string
	Groups  []string
}

// IdentityMiddleware reads the X-User-* headers Envoy injects after edge OIDC
// and attaches an Identity to the request. Requests without an identity are
// rejected with 401.
//
// SECURITY: this trusts injected headers, so the gateway->backend path MUST be
// locked down (NetworkPolicy / mTLS) so callers cannot spoof X-User-*.
func IdentityMiddleware(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := strings.TrimSpace(c.GetHeader(cfg.UserEmailHeader))
		userID := strings.TrimSpace(c.GetHeader(cfg.UserIDHeader))

		subject := userID
		if subject == "" {
			subject = email
		}
		if subject == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "missing identity headers from gateway",
			})
			return
		}

		var groups []string
		if g := c.GetHeader(cfg.UserGroupsHeader); g != "" {
			for _, part := range strings.Split(g, ",") {
				if t := strings.TrimSpace(part); t != "" {
					groups = append(groups, t)
				}
			}
		}

		c.Set(contextKey, Identity{
			Subject: subject,
			Email:   email,
			Name:    strings.TrimSpace(c.GetHeader(cfg.UserNameHeader)),
			Groups:  groups,
		})
		c.Next()
	}
}

// GetIdentity returns the Identity attached by IdentityMiddleware.
func GetIdentity(c *gin.Context) Identity {
	v, _ := c.Get(contextKey)
	id, _ := v.(Identity)
	return id
}
