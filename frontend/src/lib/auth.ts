import {
  UserManager,
  WebStorageStateStore,
  InMemoryWebStorage,
  type User,
} from "oidc-client-ts";

import { config } from "@/config";

// Authorization Code + PKCE against Authentik (public client, no secret).
//
// Token posture (see docs/06 §4.1/§5): the access token is kept in memory via
// InMemoryWebStorage, NOT localStorage — a page reload re-runs silent renew
// against the Authentik SSO session rather than persisting tokens in the browser.
export const userManager = new UserManager({
  authority: config.oidc.issuer,
  client_id: config.oidc.clientId,
  redirect_uri: config.oidc.redirectUri,
  response_type: "code",
  scope: config.oidc.scope,
  post_logout_redirect_uri: window.location.origin,
  automaticSilentRenew: true,
  // Keep tokens out of persistent storage.
  userStore: new WebStorageStateStore({ store: new InMemoryWebStorage() }),
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

export async function getUser(): Promise<User | null> {
  return userManager.getUser();
}

/**
 * Returns a fresh access token, renewing silently if expired. Throws if the
 * user is not signed in (callers should redirect to login).
 */
export async function getAccessToken(): Promise<string> {
  let user = await userManager.getUser();
  if (!user || user.expired) {
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
