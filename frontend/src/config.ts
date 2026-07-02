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

/**
 * xterm.js theme matching the app palette (see index.css :root — Ink /
 * Carbon / Seafoam / Signal / Mist). ttyd merges URL query params into its
 * client options, so passing `theme` + font settings restyles the embedded
 * terminal without touching the ttyd deployment.
 */
const ttydTheme = {
  background: "#080d0c", // Ink — matches --background
  foreground: "#e6f0ec", // Mist
  cursor: "#25d0a0", //     Seafoam — matches --primary
  cursorAccent: "#080d0c",
  selectionBackground: "#25d0a04d",
  black: "#111917",
  red: "#e14747",
  green: "#25d0a0",
  yellow: "#f4b434",
  blue: "#4aa8e8",
  magenta: "#b48ae8",
  cyan: "#21d5ed",
  white: "#e6f0ec",
  brightBlack: "#3d4a46",
  brightRed: "#ef6b6b",
  brightGreen: "#4fe0b6",
  brightYellow: "#f7c65e",
  brightBlue: "#6fbcf0",
  brightMagenta: "#c7a5f0",
  brightCyan: "#55e0f2",
  brightWhite: "#f4faf8",
} as const;

/** Terminal URL with ttyd client options that match the site's theme. */
export function embeddedTerminalUrl(id: string): string {
  const params = new URLSearchParams({
    fontSize: "14",
    fontFamily: "JetBrains Mono, SFMono-Regular, Menlo, monospace",
    theme: JSON.stringify(ttydTheme),
  });
  return `${terminalUrl(id)}?${params.toString()}`;
}

/**
 * The terminal can only be embedded when it's served from the SPA's own
 * origin — cross-origin deployments can't be auth-probed (CORS) and the
 * Authentik redirect can't be framed, so we fall back to the new-tab handoff.
 */
export function isTerminalEmbeddable(): boolean {
  try {
    return new URL(config.publicBaseUrl).origin === window.location.origin;
  } catch {
    return false;
  }
}
