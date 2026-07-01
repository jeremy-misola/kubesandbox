import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";

import { completeLogin, userManager } from "@/lib/auth";

export function CallbackPage() {
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);
  const ran = useRef(false);

  useEffect(() => {
    if (ran.current) return; // guard against React 18/19 StrictMode double-run
    ran.current = true;

    // No separate silent_redirect_uri is configured (see lib/auth.ts), so
    // AuthProvider's signinSilent() reuses this same redirect_uri and loads it
    // in a hidden iframe. In that case this page's only job is to relay the
    // auth response back to the parent window — not to run the top-level
    // redirect-callback flow or navigate anywhere.
    if (window.self !== window.top) {
      userManager.signinSilentCallback().catch(() => {
        // Parent's signinSilent() call will reject/timeout on its own;
        // AuthProvider treats that as "not signed in" and nothing else
        // needs to happen from inside the iframe.
      });
      return;
    }

    completeLogin()
      .then((user) => {
        const returnTo = (user.state as { returnTo?: string } | undefined)?.returnTo;
        navigate(returnTo ?? "/dashboard", { replace: true });
      })
      .catch((e: unknown) => {
        setError(e instanceof Error ? e.message : "Sign-in failed");
      });
  }, [navigate]);

  return (
    <div className="flex min-h-[60vh] items-center justify-center text-center">
      {error ? (
        <div>
          <p className="text-red-400">Sign-in failed: {error}</p>
          <a href="/" className="mt-2 inline-block text-primary hover:underline">
            Back to sign in
          </a>
        </div>
      ) : (
        <p className="text-muted-foreground">Completing sign-in…</p>
      )}
    </div>
  );
}
