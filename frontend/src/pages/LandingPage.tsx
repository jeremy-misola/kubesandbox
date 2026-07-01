import { useEffect } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { useAuth } from "@/hooks/useAuth";

export function LandingPage() {
  const { isAuthenticated, loading, login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;

  useEffect(() => {
    if (isAuthenticated) navigate("/dashboard", { replace: true });
  }, [isAuthenticated, navigate]);

  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center text-center">
      <h1 className="text-4xl font-bold tracking-tight">
        On-demand Kubernetes playgrounds
      </h1>
      <p className="mt-3 max-w-xl text-muted-foreground">
        Ephemeral, isolated vclusters with a browser terminal. Spin one up in
        seconds; it cleans itself up when the timer runs out.
      </p>
      <Button
        className="mt-6"
        disabled={loading}
        onClick={() => login(returnTo)}
      >
        Sign in to get started
      </Button>
    </div>
  );
}
