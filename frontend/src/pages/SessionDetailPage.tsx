import { useLayoutEffect, useRef } from "react";
import { Link, useParams } from "react-router-dom";
import { animate, stagger } from "animejs";

import { Button } from "@/components/ui/button";
import { Card, TerminalChrome } from "@/components/ui/card";
import { StatusBadge } from "@/components/StatusBadge";
import { useSession } from "@/hooks/useSessions";
import { useSessionEvents } from "@/hooks/useSessionEvents";
import { terminalUrl } from "@/config";
import { prefersReducedMotion, timeLeft } from "@/lib/utils";

export function SessionDetailPage() {
  const { id = "" } = useParams();
  const { data: session, isLoading, isError } = useSession(id);
  // Live updates stream into the cache; falls back to polling on failure.
  useSessionEvents(id, !!session);

  const specRef = useRef<HTMLDListElement>(null);
  const animatedRef = useRef(false);

  useLayoutEffect(() => {
    if (animatedRef.current || !session || !specRef.current) return;
    animatedRef.current = true;
    if (prefersReducedMotion()) return;
    animate(Array.from(specRef.current.children), {
      opacity: [0, 1],
      translateX: [-12, 0],
      duration: 350,
      ease: "out(3)",
      delay: stagger(40),
    });
  }, [session]);

  if (isLoading) {
    return (
      <div className="flex min-h-[40vh] flex-col items-center justify-center gap-3">
        <i
          aria-hidden
          className="h-6 w-6 animate-spin rounded-full border-2 border-border border-t-primary"
        />
        <p className="font-mono text-xs text-muted-foreground">loading session…</p>
      </div>
    );
  }
  if (isError || !session) {
    return (
      <Card className="border-danger/30 p-6">
        <p className="text-danger">
          Session not found — it may have expired and cleaned itself up.
        </p>
        <Link
          to="/dashboard"
          className="mt-2 inline-block rounded font-mono text-xs text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
        >
          ← back to dashboard
        </Link>
      </Card>
    );
  }

  const rows: Array<[string, string]> = [
    ["id", session.id],
    ["profile", session.profile],
    ["resources", `${session.resources.cpu} CPU · ${session.resources.memory}`],
    ["ttl", `${session.ttlMinutes} min`],
    ["phase", session.phase],
    ["time left", timeLeft(session.expiresAt) ?? "—"],
    ["namespace", session.sessionNamespace ?? "—"],
    ["image", session.workspaceImage],
  ];

  return (
    <div>
      <Link
        to="/dashboard"
        className="rounded font-mono text-xs text-muted-foreground transition-colors hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
      >
        ← ~/dashboard
      </Link>

      <Card className="mt-4 overflow-hidden p-0">
        <TerminalChrome title={session.id}>
          <span className="ml-auto">
            <StatusBadge session={session} />
          </span>
        </TerminalChrome>

        <div className="p-5">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h1 className="font-display text-xl font-bold capitalize tracking-tight">
              {session.profile} sandbox
            </h1>
            {timeLeft(session.expiresAt) && (
              <span className="font-mono text-sm text-primary">
                ⏳ {timeLeft(session.expiresAt)}
              </span>
            )}
          </div>

          <dl
            ref={specRef}
            className="mt-4 grid gap-x-8 gap-y-0 font-mono text-[13px] sm:grid-cols-2"
          >
            {rows.map(([k, v]) => (
              <div
                key={k}
                className="flex items-baseline justify-between gap-3 border-b border-border/50 py-2"
              >
                <dt className="shrink-0 text-[11px] uppercase tracking-widest text-muted-foreground">
                  {k}
                </dt>
                <dd className="truncate text-foreground" title={v}>
                  {v}
                </dd>
              </div>
            ))}
          </dl>

          {!session.workspaceReady && (
            <div className="mt-5 flex items-start gap-3 rounded-md border border-warning/25 bg-warning/5 px-3.5 py-3">
              <i
                aria-hidden
                className="dot-breathe mt-1 h-2 w-2 shrink-0 rounded-full bg-warning text-warning"
              />
              <p className="text-sm text-warning/90">
                Provisioning — vcluster cold-boot can take a few minutes.{" "}
                <span className="text-warning/70">
                  This page updates live; the terminal button unlocks the moment
                  your workspace is ready.
                </span>
              </p>
            </div>
          )}

          <div className="mt-6 flex items-center gap-3">
            <a href={terminalUrl(session.id)} target="_blank" rel="noreferrer">
              <Button disabled={!session.workspaceReady}>Open terminal</Button>
            </a>
            {session.workspaceReady && (
              <span className="font-mono text-xs text-muted-foreground">
                opens in a new tab
              </span>
            )}
          </div>
        </div>
      </Card>
    </div>
  );
}
