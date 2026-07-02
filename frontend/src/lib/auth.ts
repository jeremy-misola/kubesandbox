import {
  UserManager,
  WebStorageStateStore,
  type User,
} from "oidc-client-ts";

import { config } from "@/config";

// Authorization Code + PKCE against Authentik (public client, no secret).
//
// Token posture (revised from docs/06 §4.1/§5): the original design kept
// tokens in memory only and relied on iframe silent renew against the
// Authentik SSO session. That is structurally broken cross-site — the app
// (kubesandbox.com) and Authentik (auth.jeremymr.dev) are different sites, so
// the SameSite=Lax session cookie is never sent inside the hidden iframe and
// prompt=none always returns login_required (every page refresh logged the
// user out). Instead:
//
//   - The OIDC user (incl. refresh token) is stored in sessionStorage:
//     per-tab, cleared when the tab closes, never in localStorage.
//   - `offline_access` scope requests a refresh token, so renewal happens via
//     a direct fetch to the token endpoint — no iframe, no third-party
//     cookies involved.
export const userManager = new UserManager({
  authority: config.oidc.issuer,
  client_id: config.oidc.clientId,
  redirect_uri: config.oidc.redirectUri,
  response_type: "code",
  scope: config.oidc.scope,
  post_logout_redirect_uri: window.location.origin,
  automaticSilentRenew: true,
  // Per-tab persistence so a page refresh keeps the session (see above).
  userStore: new WebStorageStateStore({ store: window.sessionStorage }),
  // PKCE state (short-lived) may live in sessionStorage across the redirect.
  stateStore: new WebStorageStateStore({ store: window.sessionStorage }),
});

export async function login(targetPath?: string): Promise<void> {
  await userManager.signinRedirect({
    state: targetPath ? { returnTo: targetPath } : undefined,
  });
}

export async function completeLogin(): Promise<User> {
  return userManager.signinRedirectCallback();
}

export async function logout(): Promise<void> {
  await userManager.signoutRedirect();
}

/**
 * Returns a fresh access token, renewing silently if expired. Throws if the
 * user is not signed in (callers should redirect to login).
 */
export async function getAccessToken(): Promise<string> {
  let user = await userManager.getUser();
  // Only renew when a stored user exists (it carries the refresh token).
  // With no stored user, signinSilent() would fall back to the cross-site
  // iframe flow, which always fails — but only after several seconds.
  if (user?.expired) {
    try {
      user = await userManager.signinSilent();
    } catch {
      user = null;
    }
  }
  if (!user?.access_token) {
    throw new UnauthenticatedError();
  }
  return user.access_token;
}

/** The Authentik `sub` (hashed uid) — the identity key the backend uses. */
export function subject(user: User | null): string | undefined {
  return user?.profile?.sub;
}

export class UnauthenticatedError extends Error {
  constructor() {
    super("not authenticated");
    this.name = "UnauthenticatedError";
  }
}
