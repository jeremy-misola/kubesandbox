package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
)

// AuthzHandler serves the ext-authz (ForwardAuth) endpoint used by the per-session
// SecurityPolicy (G2, Option A). The gateway calls it for every request to a
// /s/{id} session route, after edge OIDC, to decide whether the authenticated
// caller owns that session. Ownership logic lives only here, so deleting a claim
// immediately revokes access (next request -> 403/404) with no per-session
// secrets to rotate.
//
// Contract with the gateway SecurityPolicy:
//   - Identity arrives the same way it does for /api: the X-User-* headers the
//     edge injects after authN (see IdentityMiddleware). Missing identity -> 401.
//   - The original request URI (/s/{id}/...) is conveyed either by a forwarded
//     header (X-Forwarded-Uri / X-Original-Uri / X-Envoy-Original-Path) or, when
//     Envoy forwards the original path directly to the auth service, as this
//     request's own path under the /authz mount.
//
// Decision:
//   - caller owns the session  -> 200 (allow)
//   - unknown / unowned / malformed id, or no /s/{id} in the path -> 403 (deny).
//     These are deliberately indistinguishable so existence is not leaked.
//   - backend/lookup error      -> 503 (fail closed).
type AuthzHandler struct {
	svc *k8s.SessionService
}

// NewAuthzHandler constructs an AuthzHandler.
func NewAuthzHandler(svc *k8s.SessionService) *AuthzHandler {
	return &AuthzHandler{svc: svc}
}

// forwardedPathHeaders are the headers an upstream proxy may use to convey the
// original request URI, checked in priority order.
var forwardedPathHeaders = []string{
	"X-Forwarded-Uri",
	"X-Original-Uri",
	"X-Envoy-Original-Path",
}

// Check handles GET /authz.
func (h *AuthzHandler) Check(c *gin.Context) {
	ident := middleware.GetIdentity(c)

	id, ok := extractSessionID(h.originalPath(c))
	if !ok {
		// No /s/{id} segment to authorize: deny by default rather than fail open.
		c.Status(http.StatusForbidden)
		return
	}

	switch err := h.svc.Authorize(c.Request.Context(), id, ident.Subject); {
	case err == nil:
		c.Status(http.StatusOK)
	case errors.Is(err, k8s.ErrNotFound), errors.Is(err, k8s.ErrInvalidID):
		// Unknown, unowned, and malformed ids are indistinguishable (no leak).
		c.Status(http.StatusForbidden)
	default:
		// Unexpected backend error: fail closed.
		c.Status(http.StatusServiceUnavailable)
	}
}

// originalPath determines the client's original request path. It prefers the
// forwarded-URI headers a gateway sets, then falls back to this request's own
// path with any /authz mount prefix stripped (Envoy's ext-authz HTTP service can
// forward the original path directly to the auth backend).
func (h *AuthzHandler) originalPath(c *gin.Context) string {
	for _, hdr := range forwardedPathHeaders {
		if v := strings.TrimSpace(c.GetHeader(hdr)); v != "" {
			return v
		}
	}
	return strings.TrimPrefix(c.Request.URL.Path, "/authz")
}

// extractSessionID pulls the session id from a path containing an "/s/{id}"
// segment, e.g. "/s/playground-s-1a2b3c4d/token" -> "playground-s-1a2b3c4d".
// It returns ok=false when no such segment is present.
func extractSessionID(path string) (string, bool) {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "s" && segs[i+1] != "" {
			return segs[i+1], true
		}
	}
	return "", false
}
