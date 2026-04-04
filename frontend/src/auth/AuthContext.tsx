import { createContext, ReactNode } from 'react';

export interface User {
  id: string | null;
  email: string | null;
  name: string | null;
  groups: string[];
}

export interface AuthContextValue {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: () => void;
  logout: () => void;
}

export const AuthContext = createContext<AuthContextValue | undefined>(undefined);

interface AuthProviderProps {
  children: ReactNode;
  user?: User | null;
}

export function AuthProvider({ children, user }: AuthProviderProps) {
  const isAuthenticated = user !== null && user !== undefined && user.email !== null;

  const login = () => {
    // Envoy Gateway will handle the redirect to Authentik
    // We redirect to a protected route which triggers authentication
    window.location.href = '/terminal';
  };

  const logout = () => {
    // Redirect to the logout path configured in SecurityPolicy
    window.location.href = '/auth/logout';
  };

  const value: AuthContextValue = {
    user: user ?? null,
    isAuthenticated,
    isLoading: false,
    login,
    logout,
  };

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}