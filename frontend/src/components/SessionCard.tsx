import { useState } from "react";
import { Link } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { Card, TerminalChrome } from "@/components/ui/card";
import { ConfirmDeleteDialog } from "@/components/ConfirmDeleteDialog";
import { ProvisioningProgress } from "@/components/ProvisioningProgress";
import { StatusBadge } from "@/components/StatusBadge";
import { useDeleteSession } from "@/hooks/useSessions";
import { useSessionEvents } from "@/hooks/useSessionEvents";
import { terminalUrl } from "@/config";
import { timeLeft } from "@/lib/utils";
import type { Session } from "@/lib/schemas";

/** A session rendered as a live terminal window: chrome bar, dense spec
 *  readout in mono, actions along the bottom. */
export function SessionCard({ session }: { session: Session }) {
  const del = useDeleteSession();
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const left = timeLeft(session.expiresAt);

  // Provisioning takes ~5 min (well past the 10s threshold in
  // design-principles §1), so the card carries a determinate indicator until
  // the workspace is ready. It stays mounted through the ready flip so its
  // completion animation can play, then dismisses itself via onDone.
  const provisioning =
    !session.workspaceReady &&
    session.phase !== "Error" &&
    session.phase !== "Unknown";
  const [progressVisible, setProgressVisible] = useState(provisioning);

  // Live phase updates for this card while it provisions (SSE, with the
  // hook's built-in polling fallback). Without this, the list only learns
  // about readiness on a manual refetch.
  useSessionEvents(session.id, provisioning);

  const showProgress =
    progressVisible && session.phase !== "Error" && session.phase !== "Unknown";

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

        {showProgress && (
          <ProvisioningProgress
            session={session}
            onDone={() => setProgressVisible(false)}
          />
        )}

        {session.message && !session.workspaceReady && !showProgress && (
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
          <a href={terminalUrl(session.id)} target="_blank" rel="noreferrer">
            <Button size="sm" disabled={!session.workspaceReady}>
              Open terminal
            </Button>
          </a>
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
