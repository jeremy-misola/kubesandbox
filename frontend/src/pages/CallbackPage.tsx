import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";

import { completeLogin } from "@/lib/auth";

export function CallbackPage() {
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);
  const ran = useRef(false);

  useEffect(() => {
    if (ran.current) return; // guard against React 18/19 StrictMode double-run
    ran.current = true;

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
        <div className="max-w-md border border-danger/30 bg-danger/5 p-8">
          <p className="font-mono text-[11px] uppercase tracking-label text-danger">
            auth error
          </p>
          <p className="mt-3 text-sm text-foreground">Sign-in failed: {error}</p>
          <a
            href="/"
            className="mt-5 inline-block font-mono text-xs uppercase tracking-label text-accent underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
          >
            ← back to sign in
          </a>
        </div>
      ) : (
        <div className="flex flex-col items-center gap-3">
          <i
            aria-hidden
            className="h-6 w-6 animate-spin rounded-full border-2 border-border border-t-accent"
          />
          <p className="font-mono text-xs text-muted-foreground">
            completing sign-in…
          </p>
        </div>
      )}
    </div>
  );
}
