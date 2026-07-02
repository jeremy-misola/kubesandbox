// Runtime configuration.
//
// Vite inlines import.meta.env.VITE_* at BUILD time, but the Helm chart injects
// env into the RUNNING container. To make one image promotable across
// environments, docker-entrypoint.sh writes /config.js at container start:
//
//   window.__ENV = { VITE_API_BASE: "/api", VITE_OIDC_ISSUER: "...", ... }
//
// Resolution order per key: window.__ENV -> import.meta.env -> default.

function fromEnv(key: keyof ImportMetaEnv, fallback: string): string {
  const runtime = typeof window !== "undefined" ? window.__ENV?.[key] : undefined;
  const build = import.meta.env[key];
  const val = (runtime ?? build ?? "").toString().trim();
  return val.length > 0 ? val : fallback;
}

export const config = {
  /** Base path for the control API (gateway routes this to the backend). */
  apiBase: fromEnv("VITE_API_BASE", "/api"),
  /** Origin used to build the terminal hand-off URL: `${publicBaseUrl}/s/{id}`. */
  publicBaseUrl: fromEnv("VITE_PUBLIC_BASE_URL", window.location.origin),
  oidc: {
    issuer: fromEnv(
      "VITE_OIDC_ISSUER",
      "https://auth.jeremymr.dev/application/o/kubesandbox-frontend/",
    ),
    clientId: fromEnv("VITE_OIDC_CLIENT_ID", "kubesandbox-frontend"),
    redirectUri: fromEnv(
      "VITE_OIDC_REDIRECT_URI",
      `${window.location.origin}/auth/callback`,
    ),
    // Authentik's OIDC endpoints are shared, not per-provider — but
    // oidc-client-ts discovers them from the issuer's
    // /.well-known/openid-configuration, which Authentik serves correctly.
    // `offline_access` requests a refresh token so renewal (and page
    // refresh) works via the token endpoint instead of iframe silent renew,
    // which fails cross-site (see lib/auth.ts). Requires the offline_access
    // scope mapping to be enabled on the Authentik provider.
    scope: "openid email profile offline_access",
  },
} as const;

/** Build the browser terminal URL for a session id. */
export function terminalUrl(id: string): string {
  return `${config.publicBaseUrl}/s/${id}`;
}
