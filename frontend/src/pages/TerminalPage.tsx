import { Link, useParams } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { Card, TerminalChrome } from "@/components/ui/card";
import { StatusBadge } from "@/components/StatusBadge";
import { TerminalFrame } from "@/components/TerminalFrame";
import { useSession } from "@/hooks/useSessions";
import { useSessionEvents } from "@/hooks/useSessionEvents";
import { isTerminalEmbeddable, terminalUrl } from "@/config";
import { timeLeft } from "@/lib/utils";

/**
 * Full-page embedded terminal. The ttyd shell renders inside the site via
 * TerminalFrame (auth probe + themed iframe). Cross-origin deployments —
 * where embedding is impossible — keep the old new-tab handoff.
 */
export function TerminalPage() {
  const { id = "" } = useParams();
  const { data: session, isLoading } = useSession(id);
  // Live phase updates while provisioning; falls back to polling.
  useSessionEvents(id, !!session && !session.workspaceReady);

  // Cross-origin terminal: embedding can't work, hand off to a new tab.
  if (!isTerminalEmbeddable()) {
    return <NewTabHandoff id={id} />;
  }

  // Mirrors the loaded layout below (header row + terminal card) so the page
  // structure registers before the session data lands (design-principles §2).
  if (isLoading) {
    return (
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex flex-wrap items-center justify-between gap-2 pb-3">
          <div className="flex items-center gap-3">
            <Link
              to="/dashboard"
              className="font-mono text-[11px] uppercase tracking-label text-muted-foreground transition-colors duration-500 ease-luxury hover:text-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
            >
              ← dashboard
            </Link>
            <div className="skeleton h-4 w-32" />
          </div>
          <div className="skeleton h-6 w-24" />
        </div>
        <Card className="overflow-hidden p-0">
          <TerminalChrome title={`s/${id}`} />
          <div className="space-y-3 p-5">
            <div className="skeleton h-4 w-3/4" />
            <div className="skeleton h-4 w-1/2" />
          </div>
        </Card>
      </div>
    );
  }

  const provisioning = session && !session.workspaceReady;
  const left = session ? timeLeft(session.expiresAt) : null;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex flex-wrap items-center justify-between gap-2 pb-3">
        <div className="flex items-center gap-3">
          <Link
            to="/dashboard"
            className="font-mono text-[11px] uppercase tracking-label text-muted-foreground transition-colors duration-500 ease-luxury hover:text-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
          >
            ← dashboard
          </Link>
          <h1 className="font-display text-xl font-normal tracking-tight">
            {session ? "Kubernetes sandbox" : `s/${id}`}
          </h1>
        </div>
        <div className="flex items-center gap-3">
          {left && (
            <span className="font-mono text-xs text-accent">{left}</span>
          )}
          {session && <StatusBadge session={session} />}
        </div>
      </div>

      {provisioning ? (
        <Card className="overflow-hidden p-0">
          <TerminalChrome title={`s/${id}`} />
          <div className="flex items-start gap-3 p-5">
            <i
              aria-hidden
              className="dot-breathe mt-1 h-2 w-2 shrink-0 rounded-full bg-warning text-warning"
            />
            <div>
              <p className="text-sm text-warning/90">
                Provisioning — vcluster cold-boot can take a few minutes.
              </p>
              <p className="mt-1 text-sm text-warning/70">
                This page updates live; the terminal connects here the moment
                your workspace is ready.
              </p>
            </div>
          </div>
        </Card>
      ) : (
        <TerminalFrame
          id={id}
          className="min-h-[420px] flex-1 basis-[calc(100vh-230px)]"
        />
      )}
    </div>
  );
}

/** Legacy handoff for deployments where the terminal lives on another origin. */
function NewTabHandoff({ id }: { id: string }) {
  const url = terminalUrl(id);
  return (
    <div className="flex min-h-[55vh] flex-col items-center justify-center">
      <div className="w-full max-w-md overflow-hidden border border-border bg-card shadow-hero">
        <TerminalChrome title={`s/${id}`} />
        <div className="p-6 text-center">
          <p className="font-mono text-sm">
            <span className="text-accent">❯</span> open terminal
            <span className="cursor-blink -mb-0.5 ml-1 inline-block h-4 w-2 bg-accent align-middle" />
          </p>
          <h1 className="mt-4 font-display text-2xl font-normal tracking-tight">
            Your terminal opens in a new tab
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            This deployment serves the terminal from a different origin, so it
            can't be embedded here.
          </p>
          <a className="mt-5 inline-block" href={url} target="_blank" rel="noreferrer">
            <Button>Open terminal</Button>
          </a>
          <div className="mt-4">
            <Link
              to="/dashboard"
              className="font-mono text-[11px] uppercase tracking-label text-muted-foreground transition-colors duration-500 ease-luxury hover:text-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
            >
              ← back to dashboard
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
