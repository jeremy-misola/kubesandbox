import {
  createContext,
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import type { User } from "oidc-client-ts";

import { login as doLogin, logout as doLogout, subject, userManager } from "@/lib/auth";
import { config } from "@/config";

export interface AuthState {
  user: User | null;
  loading: boolean;
  isAuthenticated: boolean;
  /** Authentik `sub` (hashed uid) — the backend identity key. */
  sub?: string;
  login: (targetPath?: string) => Promise<void>;
  logout: () => Promise<void>;
}

// eslint-disable-next-line react-refresh/only-export-components
export const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;

    // Register the event listeners FIRST, unconditionally. CallbackPage
    // completes the explicit sign-in via completeLogin(), and the only way
    // AuthProvider learns about that user is the `userLoaded` event — so the
    // listeners must be in place even on the callback route. (Previously they
    // were registered after the early return below, so a successful login
    // never reached React state and ProtectedRoute bounced back to "/".)
    const onLoaded = (u: User) => setUser(u);
    const onUnloaded = () => setUser(null);
    userManager.events.addUserLoaded(onLoaded);
    userManager.events.addUserUnloaded(onUnloaded);
    userManager.events.addAccessTokenExpired(onUnloaded);

    const cleanup = () => {
      active = false;
      userManager.events.removeUserLoaded(onLoaded);
      userManager.events.removeUserUnloaded(onUnloaded);
      userManager.events.removeAccessTokenExpired(onUnloaded);
    };

    const isSilentRenewIframe = window.self !== window.top;
    const isOidcCallbackRoute =
      window.location.pathname === new URL(config.oidc.redirectUri).pathname;

    // Two cases where AuthProvider must NOT run its own getUser()/signinSilent()
    // dance:
    //  1. This document is the hidden iframe spawned by signinSilent() (see
    //     lib/auth.ts) — it's only here to run CallbackPage's relay-to-parent
    //     logic, not to render the app.
    //  2. This is a top-level load of the OIDC redirect route (/auth/callback),
    //     i.e. we just came back from Authentik after an explicit sign-in.
    //     CallbackPage owns completing that flow via completeLogin(). Running a
    //     second, redundant signinSilent() here races against it: it spins up
    //     its own iframe round trip to Authentik and, if it resolves after (or
    //     fails independently from) the real login, its `catch` branch would
    //     overwrite the just-established session with `null`. AuthProvider
    //     picks up the real result via the `userLoaded` listener above.
    if (isSilentRenewIframe || isOidcCallbackRoute) {
      setLoading(false);
      return cleanup;
    }

    // The OIDC user persists in sessionStorage (see lib/auth.ts), so a page
    // reload normally finds it via getUser(). If it's missing or the access
    // token has expired, signinSilent() renews via the refresh token — a
    // direct token-endpoint fetch, which works cross-site (iframe renew
    // does not; that's what broke refresh before).
    (async () => {
      let u = await userManager.getUser();
      if (!u || u.expired) {
        try {
          u = await userManager.signinSilent();
        } catch {
          u = null;
        }
      }
      if (active) setUser(u);
    })().finally(() => active && setLoading(false));

    return cleanup;
  }, []);

  const login = useCallback((targetPath?: string) => doLogin(targetPath), []);
  const logout = useCallback(() => doLogout(), []);

  const value = useMemo<AuthState>(
    () => ({
      user,
      loading,
      isAuthenticated: !!user && !user.expired,
      sub: subject(user),
      login,
      logout,
    }),
    [user, loading, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
