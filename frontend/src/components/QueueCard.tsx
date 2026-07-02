import { useEffect, useRef } from "react";
import { animate } from "animejs";

import { Card, TerminalChrome } from "@/components/ui/card";
import { prefersReducedMotion } from "@/lib/utils";
import type { QueueStatus } from "@/lib/schemas";

/**
 * The "in line" state (design-principles §1: waits past ~10s need a
 * determinate indicator, not a looping spinner — the queue position IS the
 * progress). Rendered by the dashboard while the warm pool is drained; the
 * position updates live over SSE and the card is replaced by the sandbox
 * card the moment one is handed over.
 */
export function QueueCard({ queue }: { queue: QueueStatus }) {
  const posRef = useRef<HTMLSpanElement>(null);
  const prevPos = useRef(queue.position);

  // The position ticking down is the moment of progress — make it land.
  useEffect(() => {
    if (prevPos.current === queue.position) return;
    prevPos.current = queue.position;
    if (posRef.current && !prefersReducedMotion()) {
      animate(posRef.current, {
        scale: [1.4, 1],
        opacity: [0.3, 1],
        duration: 500,
        ease: "out(3)",
      });
    }
  }, [queue.position]);

  return (
    <Card className="overflow-hidden p-0">
      <TerminalChrome title="kubesandbox — waiting">
        <span className="ml-auto inline-flex items-center gap-1.5 rounded-full border border-warning/30 bg-warning/10 px-2.5 py-0.5 font-mono text-[11px] font-medium uppercase tracking-wider text-warning">
          <i aria-hidden className="dot-breathe h-1.5 w-1.5 rounded-full bg-warning text-warning" />
          In line
        </span>
      </TerminalChrome>

      <div className="px-6 py-10 text-center" role="status" aria-live="polite">
        <p className="font-mono text-sm text-muted-foreground">
          <span className="text-primary">❯</span> kubesandbox create
          <br />
          <span className="text-muted-foreground/70">
            every sandbox is in use right now
          </span>
        </p>

        <p className="mt-6 font-display text-lg font-semibold tracking-tight">
          You're{" "}
          <span ref={posRef} className="inline-block font-mono text-2xl text-primary">
            #{queue.position}
          </span>{" "}
          in line
        </p>

        <p className="mx-auto mt-3 max-w-sm text-sm text-muted-foreground">
          A replacement is already being built in the background. This page
          updates live — your sandbox appears here the moment one is handed
          over. Leaving the page is fine; your place is kept.
        </p>
      </div>
    </Card>
  );
}
