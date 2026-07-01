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

    // If this document is the hidden iframe spawned by signinSilent() (see
    // lib/auth.ts), it's only here to run CallbackPage's relay-to-parent logic
    // — not to render the app. Skip the getUser()/signinSilent() dance so we
    // don't recursively spawn another silent-renew iframe from inside this one.
    if (window.self !== window.top) {
      setLoading(false);
      return;
    }

    // Access tokens live in memory only (see lib/auth.ts), so a page reload
    // always starts with no user here. Before treating that as "signed out",
    // try a silent renew against the Authentik SSO session — if the user is
    // still logged in there, this resolves without any visible redirect.
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

    const onLoaded = (u: User) => setUser(u);
    const onUnloaded = () => setUser(null);
    userManager.events.addUserLoaded(onLoaded);
    userManager.events.addUserUnloaded(onUnloaded);
    userManager.events.addAccessTokenExpired(onUnloaded);

    return () => {
      active = false;
      userManager.events.removeUserLoaded(onLoaded);
      userManager.events.removeUserUnloaded(onUnloaded);
      userManager.events.removeAccessTokenExpired(onUnloaded);
    };
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
