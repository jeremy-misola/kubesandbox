import {
  createContext,
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import type { User } from "oidc-client-ts";

import {
  login as doLogin,
  logout as doLogout,
  signup as doSignup,
  subject,
  userManager,
} from "@/lib/auth";
import { config } from "@/config";

export interface AuthState {
  user: User | null;
  loading: boolean;
  isAuthenticated: boolean;
  /** Authentik `sub` (hashed uid) — the backend identity key. */
  sub?: string;
  login: (targetPath?: string) => Promise<void>;
  logout: () => Promise<void>;
  signup: () => void;
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

    // On the OIDC redirect route (/auth/callback, top-level return from
    // Authentik after an explicit sign-in) AuthProvider must NOT run its own
    // getUser()/signinSilent() below: CallbackPage owns completing that flow
    // via completeLogin(), and a concurrent signinSilent() could race it and
    // overwrite the just-established session with `null`. AuthProvider picks
    // up the real result via the `userLoaded` listener above.
    const isOidcCallbackRoute =
      window.location.pathname === new URL(config.oidc.redirectUri).pathname;
    if (isOidcCallbackRoute) {
      setLoading(false);
      return cleanup;
    }

    // The OIDC user persists in sessionStorage (see lib/auth.ts), so a page
    // reload normally finds it via getUser(). Only attempt signinSilent()
    // when a stored user EXISTS but its access token expired — that path
    // renews via the refresh token (direct token-endpoint fetch, works
    // cross-site). No stored user means no refresh token, so treat that as
    // signed out immediately rather than attempting a silent sign-in that
    // cannot succeed.
    (async () => {
      let u = await userManager.getUser();
      if (u?.expired) {
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
  const signup = useCallback(() => doSignup(), []);

  const value = useMemo<AuthState>(
    () => ({
      user,
      loading,
      isAuthenticated: !!user && !user.expired,
      sub: subject(user),
      login,
      logout,
      signup,
    }),
    [user, loading, login, logout, signup],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
