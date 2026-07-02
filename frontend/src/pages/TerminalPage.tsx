import { useEffect } from "react";
import { useParams, Link } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { TerminalChrome } from "@/components/ui/card";
import { terminalUrl } from "@/config";

// The terminal itself (/s/{id}) is served by the BACKEND, not the SPA. Its auth
// is a backend-owned OIDC/PKCE flow behind ext-authz (docs/06 §4.4). The first
// hit 302s to Authentik's login page, which cannot be framed in an iframe — so
// we hand off with a top-level navigation to a new tab.
export function TerminalPage() {
  const { id = "" } = useParams();
  const url = terminalUrl(id);

  useEffect(() => {
    // Auto-open in a new tab on mount; if the popup is blocked, the button below
    // is the manual fallback.
    window.open(url, "_blank", "noopener");
  }, [url]);

  return (
    <div className="flex min-h-[55vh] flex-col items-center justify-center">
      <div className="w-full max-w-md overflow-hidden rounded-lg border border-border bg-card shadow-card">
        <TerminalChrome title={`s/${id}`} />
        <div className="p-6 text-center">
          <p className="font-mono text-sm">
            <span className="text-primary">❯</span> open terminal
            <span className="cursor-blink -mb-0.5 ml-1 inline-block h-4 w-2 bg-primary align-middle" />
          </p>
          <h1 className="mt-4 font-display text-xl font-bold tracking-tight">
            Opening your terminal…
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            The terminal opens in a new tab. If nothing happened, your browser
            blocked the popup — use the button below.
          </p>
          <a className="mt-5 inline-block" href={url} target="_blank" rel="noreferrer">
            <Button>Open terminal</Button>
          </a>
          <div className="mt-4">
            <Link
              to={`/dashboard/${id}`}
              className="rounded font-mono text-xs text-muted-foreground transition-colors hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
            >
              ← back to session
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
