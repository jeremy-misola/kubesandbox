import { useEffect, useRef } from "react";
import { animate } from "animejs";

import { cn, prefersReducedMotion } from "@/lib/utils";
import type { Session } from "@/lib/schemas";

// Maps a session's phase + workspaceReady to a label + color. The phase
// vocabulary is exactly what the pipeline emits (shell pod phase mapped in
// kubesandbox-session-composition.yaml): Pending (backend default before the
// pod reports), Provisioning, Ready, Error, Unknown — confirmed live, docs/07
// §3.4. The authoritative "usable" signal is `workspaceReady`; deletion never
// surfaces as a phase (the claim is removed and the UI reacts via optimistic
// removal + the SSE `deleted` event).
function tone(session: Session): {
  label: string;
  className: string;
  dotClassName: string;
  busy: boolean;
} {
  if (session.workspaceReady) {
    return {
      label: "Ready",
      className: "border-success/30 bg-success/10 text-success",
      dotClassName: "bg-success text-success",
      busy: false,
    };
  }
  switch (session.phase) {
    case "Error":
      return {
        label: "Error",
        className: "border-danger/30 bg-danger/10 text-danger",
        dotClassName: "bg-danger text-danger",
        busy: false,
      };
    case "Unknown":
      return {
        label: "Unknown",
        className: "border-border bg-muted text-muted-foreground",
        dotClassName: "bg-muted-foreground text-muted-foreground",
        busy: false,
      };
    default:
      // Pending / Provisioning. An unrecognized future phase also lands here,
      // shown verbatim with in-progress styling rather than being hidden.
      return {
        label: session.phase || "Pending",
        className: "border-warning/30 bg-warning/10 text-warning",
        dotClassName: "bg-warning text-warning dot-breathe",
        busy: true,
      };
  }
}

export function StatusBadge({ session }: { session: Session }) {
  const { label, className, dotClassName, busy } = tone(session);
  const ref = useRef<HTMLSpanElement>(null);
  const prevLabel = useRef(label);

  // Organic transition: when the phase changes (SSE update), the badge pops.
  useEffect(() => {
    if (prevLabel.current === label) return;
    prevLabel.current = label;
    if (ref.current && !prefersReducedMotion()) {
      animate(ref.current, {
        scale: [1.35, 1],
        opacity: [0.4, 1],
        duration: 450,
        ease: "out(3)",
      });
    }
  }, [label]);

  return (
    <span
      ref={ref}
      role="status"
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5",
        "font-mono text-[11px] font-medium uppercase tracking-wider",
        className,
      )}
    >
      <i
        aria-hidden
        className={cn("h-1.5 w-1.5 rounded-full", dotClassName)}
      />
      {label}
      {busy && <span className="sr-only">(provisioning in progress)</span>}
    </span>
  );
}
