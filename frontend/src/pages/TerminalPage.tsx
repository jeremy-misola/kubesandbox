import { useEffect } from "react";
import { useParams, Link } from "react-router-dom";

import { Button } from "@/components/ui/button";
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
    <div className="flex min-h-[50vh] flex-col items-center justify-center text-center">
      <h1 className="text-xl font-semibold">Opening your terminal…</h1>
      <p className="mt-2 max-w-md text-sm text-muted-foreground">
        The terminal opens in a new tab. If nothing happened, your browser may
        have blocked the popup — use the button below.
      </p>
      <a className="mt-4" href={url} target="_blank" rel="noreferrer">
        <Button>Open terminal</Button>
      </a>
      <Link to={`/dashboard/${id}`} className="mt-3 text-sm text-primary hover:underline">
        Back to session
      </Link>
    </div>
  );
}
