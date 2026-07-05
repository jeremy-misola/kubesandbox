import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";

import { useAuth } from "@/hooks/useAuth";

export function ProtectedRoute({ children }: { children: ReactNode }) {
  const { isAuthenticated, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return (
      <div className="flex min-h-[60vh] flex-col items-center justify-center gap-3">
        <i
          aria-hidden
          className="h-6 w-6 animate-spin rounded-full border-2 border-border border-t-primary"
        />
        <p className="text-xs text-muted-foreground">
          checking credentials…
        </p>
      </div>
    );
  }
  if (!isAuthenticated) {
    return <Navigate to="/" replace state={{ returnTo: location.pathname }} />;
  }
  return <>{children}</>;
}
