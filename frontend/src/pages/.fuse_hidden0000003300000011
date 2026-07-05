import { useLayoutEffect, useRef, useState } from "react";
import { animate } from "animejs";

import { Button } from "@/components/ui/button";
import { Card, CardContent, TerminalChrome } from "@/components/ui/card";
import { CreateSessionDialog } from "@/components/CreateSessionDialog";
import { QueueCard } from "@/components/QueueCard";
import { SessionCard } from "@/components/SessionCard";
import { useQueueStatus, useQueueWatcher } from "@/hooks/useQueue";
import { useSessions } from "@/hooks/useSessions";
import { prefersReducedMotion } from "@/lib/utils";

/** Skeleton mirrors the real SessionCard layout so the page structure
 *  registers before data lands (design-principles §2). */
function SessionCardSkeleton() {
  return (
    <Card className="overflow-hidden p-0">
      <div className="flex items-center gap-2 border-b border-border bg-muted/50 px-4 py-2.5">
        <span className="flex gap-1.5">
          {[0, 1, 2].map((i) => (
            <i key={i} className="h-2.5 w-2.5 rounded-full bg-border" />
          ))}
        </span>
        <div className="skeleton h-3 w-32" />
      </div>
      <div className="space-y-3 p-4">
        <div className="skeleton h-5 w-40" />
        <div className="skeleton h-12 w-full" />
        <div className="flex justify-end gap-2 pt-1">
          <div className="skeleton h-8 w-28" />
          <div className="skeleton h-8 w-20" />
        </div>
      </div>
    </Card>
  );
}

/** One sandbox per user: the dashboard shows either the active sandbox or a
 *  create panel — never a list. */
export function DashboardPage() {
  const [creating, setCreating] = useState(false);
  const { data: sessions, isLoading, isError, error, refetch } = useSessions();
  // Warm-pool queue: restores the "in line" view on reload and follows the
  // live stream until a sandbox is handed over (or the line fails).
  const { data: queued } = useQueueStatus();
  const { queueError } = useQueueWatcher();

  // The backend enforces a single session per user; take the first defensively.
  const session = sessions?.[0];

  const cardRef = useRef<HTMLDivElement>(null);
  const animatedRef = useRef(false);

  // The sandbox card rises in once, on the first populated render.
  useLayoutEffect(() => {
    if (animatedRef.current || !session || !cardRef.current) return;
    animatedRef.current = true;
    if (prefersReducedMotion()) return;
    animate(cardRef.current, {
      opacity: [0, 1],
      translateY: [28, 0],
      scale: [0.97, 1],
      duration: 500,
      ease: "out(4)",
    });
  }, [session]);

  return (
    <div>
      <div className="mb-10 flex items-center gap-4">
        <div>
          <p className="overline text-accent">~/dashboard</p>
          <h1 className="mt-2 font-display text-4xl font-normal tracking-tight">
            Your sandbox
            {session && (
              <span className="overline ml-3 align-middle text-muted-foreground">
                [active]
              </span>
            )}
            {!session && queued && (
              <span className="overline ml-3 align-middle text-warning">
                [in line]
              </span>
            )}
          </h1>
        </div>
      </div>

      {isLoading && (
        <div className="mx-auto max-w-xl">
          <SessionCardSkeleton />
        </div>
      )}

      {isError && (
        <Card className="border-danger/30">
          <CardContent className="p-4 text-sm">
            <p className="text-danger">
              Couldn't load your sandbox
              {error instanceof Error ? `: ${error.message}` : ""}.
            </p>
            <button
              className="mt-2 font-mono text-xs text-accent underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
              onClick={() => refetch()}
            >
              retry →
            </button>
          </CardContent>
        </Card>
      )}

      {/* In line for a sandbox: the queue position is the progress. */}
      {!isLoading && !isError && !session && queued && (
        <div className="mx-auto max-w-xl">
          <QueueCard queue={queued} />
        </div>
      )}

      {!isLoading && !isError && !session && !queued && (
        <Card className="overflow-hidden p-0">
          <TerminalChrome title="kubesandbox — empty" />
          <div className="px-6 py-16 text-center">
            <p className="font-mono text-sm text-muted-foreground">
              <span className="text-accent">❯</span> kubesandbox status
              <br />
              <span className="text-muted-foreground/70">
                no sandbox running — nothing is costing anything
              </span>
            </p>
            {queueError && (
              <p
                role="alert"
                className="mx-auto mt-4 max-w-sm rounded-md border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger"
              >
                Your place in line ended: {queueError}
              </p>
            )}
            <Button className="mt-6" onClick={() => setCreating(true)}>
              Create your sandbox
            </Button>
            <p className="mt-3 font-mono text-xs text-muted-foreground/70">
              pre-provisioned &amp; ready in seconds
            </p>
          </div>
        </Card>
      )}

      {session && (
        <div className="mx-auto max-w-xl">
          <div ref={cardRef}>
            <SessionCard session={session} />
          </div>
          <p className="mt-3 text-center font-mono text-xs text-muted-foreground">
            one sandbox per user — delete this one to start fresh
          </p>
        </div>
      )}

      {creating && <CreateSessionDialog onClose={() => setCreating(false)} />}
    </div>
  );
}
