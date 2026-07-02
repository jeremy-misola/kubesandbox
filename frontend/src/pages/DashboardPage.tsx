import { useLayoutEffect, useRef, useState } from "react";
import { animate, stagger } from "animejs";

import { Button } from "@/components/ui/button";
import { Card, CardContent, TerminalChrome } from "@/components/ui/card";
import { CreateSessionDialog } from "@/components/CreateSessionDialog";
import { SessionCard } from "@/components/SessionCard";
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

export function DashboardPage() {
  const [creating, setCreating] = useState(false);
  const { data: sessions, isLoading, isError, error, refetch } = useSessions();

  const gridRef = useRef<HTMLDivElement>(null);
  const animatedRef = useRef(false);

  // Cards ripple in once, on the first populated render.
  useLayoutEffect(() => {
    if (animatedRef.current || !sessions?.length || !gridRef.current) return;
    animatedRef.current = true;
    if (prefersReducedMotion()) return;
    animate(Array.from(gridRef.current.children), {
      opacity: [0, 1],
      translateY: [28, 0],
      scale: [0.97, 1],
      duration: 500,
      ease: "out(4)",
      delay: stagger(70),
    });
  }, [sessions]);

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="font-mono text-[11px] uppercase tracking-[0.25em] text-primary">
            ~/dashboard
          </p>
          <h1 className="mt-1 font-display text-2xl font-bold tracking-tight">
            Your sandboxes
            {sessions && sessions.length > 0 && (
              <span className="ml-2 align-middle font-mono text-sm font-normal text-muted-foreground">
                [{sessions.length} active]
              </span>
            )}
          </h1>
        </div>
        <Button onClick={() => setCreating(true)}>+ New sandbox</Button>
      </div>

      {isLoading && (
        <div className="grid gap-4 sm:grid-cols-2">
          {[0, 1].map((i) => (
            <SessionCardSkeleton key={i} />
          ))}
        </div>
      )}

      {isError && (
        <Card className="border-danger/30">
          <CardContent className="p-4 text-sm">
            <p className="text-danger">
              Couldn't load your sessions
              {error instanceof Error ? `: ${error.message}` : ""}.
            </p>
            <button
              className="mt-2 rounded font-mono text-xs text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
              onClick={() => refetch()}
            >
              retry →
            </button>
          </CardContent>
        </Card>
      )}

      {!isLoading && !isError && sessions && sessions.length === 0 && (
        <Card className="overflow-hidden p-0">
          <TerminalChrome title="kubesandbox — empty" />
          <div className="px-6 py-12 text-center">
            <p className="font-mono text-sm text-muted-foreground">
              <span className="text-primary">❯</span> kubesandbox list
              <br />
              <span className="text-muted-foreground/70">
                no sandboxes running — nothing is costing anything
              </span>
            </p>
            <Button className="mt-6" onClick={() => setCreating(true)}>
              Create your first sandbox
            </Button>
          </div>
        </Card>
      )}

      {sessions && sessions.length > 0 && (
        <div ref={gridRef} className="grid gap-4 sm:grid-cols-2">
          {sessions.map((s) => (
            <SessionCard key={s.id} session={s} />
          ))}
        </div>
      )}

      {creating && <CreateSessionDialog onClose={() => setCreating(false)} />}
    </div>
  );
}
