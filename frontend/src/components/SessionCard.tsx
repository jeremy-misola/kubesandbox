import { useState } from "react";
import { Link } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { Card, TerminalChrome } from "@/components/ui/card";
import { ConfirmDeleteDialog } from "@/components/ConfirmDeleteDialog";
import { StatusBadge } from "@/components/StatusBadge";
import { useDeleteSession } from "@/hooks/useSessions";
import { timeLeft } from "@/lib/utils";
import type { Session } from "@/lib/schemas";

/** A session rendered as a live terminal window: chrome bar, dense spec
 *  readout in mono, actions along the bottom. */
export function SessionCard({ session }: { session: Session }) {
  const del = useDeleteSession();
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const left = timeLeft(session.expiresAt);

  return (
    <Card className="flex flex-col overflow-hidden p-0">
      <TerminalChrome title={session.id}>
        <span className="ml-auto">
          <StatusBadge session={session} />
        </span>
      </TerminalChrome>

      <div className="flex flex-1 flex-col gap-3 p-4">
        <div className="flex items-baseline justify-between gap-2">
          <h3 className="font-display text-base font-semibold capitalize tracking-tight">
            {session.profile} sandbox
          </h3>
          {left && (
            <span className="shrink-0 font-mono text-xs text-primary">
              ⏳ {left}
            </span>
          )}
        </div>

        <dl className="grid grid-cols-3 gap-px overflow-hidden rounded-md border border-border/70 bg-border/70 font-mono text-xs">
          {(
            [
              ["cpu", session.resources.cpu],
              ["mem", session.resources.memory],
              ["ttl", `${session.ttlMinutes}m`],
            ] as const
          ).map(([k, v]) => (
            <div key={k} className="bg-card px-2.5 py-1.5">
              <dt className="text-[10px] uppercase tracking-widest text-muted-foreground">
                {k}
              </dt>
              <dd className="mt-0.5 text-foreground">{v}</dd>
            </div>
          ))}
        </dl>

        {session.message && !session.workspaceReady && (
          <p className="rounded-md border border-warning/20 bg-warning/5 px-2.5 py-1.5 font-mono text-xs text-warning/90">
            {session.message}
          </p>
        )}
      </div>

      <div className="flex items-center justify-between gap-2 border-t border-border/70 px-4 py-3">
        <Link
          to={`/dashboard/${session.id}`}
          className="rounded font-mono text-xs text-muted-foreground transition-colors hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
        >
          details →
        </Link>
        <div className="flex gap-2">
          <Link
            to={`/terminal/${session.id}`}
            tabIndex={session.workspaceReady ? undefined : -1}
            aria-disabled={!session.workspaceReady}
            className={!session.workspaceReady ? "pointer-events-none" : undefined}
          >
            <Button size="sm" disabled={!session.workspaceReady}>
              Open terminal
            </Button>
          </Link>
          <Button
            size="sm"
            variant="danger"
            onClick={() => setConfirmingDelete(true)}
            disabled={del.isPending}
          >
            {del.isPending ? "Deleting…" : "Delete"}
          </Button>
        </div>
      </div>

      {confirmingDelete && (
        <ConfirmDeleteDialog
          session={session}
          onConfirm={() => del.mutate(session.id)}
          onClose={() => setConfirmingDelete(false)}
        />
      )}
    </Card>
  );
}
