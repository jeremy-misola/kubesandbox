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
