import { useEffect, useMemo, useRef, useState } from "react";
import { animate } from "animejs";

import { cn, prefersReducedMotion } from "@/lib/utils";
import type { Session } from "@/lib/schemas";

// A sandbox takes ~5 minutes to provision. That is deep in the "> 10 seconds"
// tier of design-principles §1, where spinners stop working and the UI must
// show a determinate indicator. This panel renders on the SessionCard while
// the cluster comes up and combines:
//
//   1. a progress bar eased toward the ~5 min estimate (asymptotic, so it
//      keeps creeping instead of stalling at a hard cap),
//   2. a step readout driven by the real pipeline phase (Pending →
//      Provisioning → Ready) — determinate structure, not cosmetics,
//   3. rotating wait copy (sequential text sustains perceived progress
//      between phase changes), preferring the backend's own `message`.
//
// On `workspaceReady` it fills to 100%, flashes success, collapses, and asks
// the parent to unmount it via `onDone`.

/** Typical time for a sandbox cluster to become ready. */
const EXPECTED_MS = 5 * 60_000;
/** Time constant of the progress curve: ~86% at the 5-minute mark. */
const TAU_MS = 2 * 60_000;
/** The curve never exceeds this on its own — only real readiness hits 100. */
const CURVE_MAX = 94;
/** While the pod hasn't been scheduled (phase Pending), stay honest and low. */
const PENDING_CAP = 12;

const WAIT_MESSAGES = [
  "spinning up your cluster…",
  "pulling the workspace image…",
  "starting the shell pod…",
  "wiring up networking…",
  "almost there…",
];

function progressFor(phase: string, ready: boolean, elapsedMs: number): number {
  if (ready) return 100;
  const curve = CURVE_MAX * (1 - Math.exp(-elapsedMs / TAU_MS));
  if (!phase || phase === "Pending") return Math.min(curve, PENDING_CAP);
  // Once provisioning starts, never dip back below the pending cap.
  return Math.max(curve, PENDING_CAP);
}

function formatElapsed(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  const m = Math.floor(s / 60);
  return m > 0 ? `${m}m ${s % 60}s` : `${s}s`;
}

export function ProvisioningProgress({
  session,
  onDone,
}: {
  session: Session;
  /** Called after the completion animation, so the parent can unmount us. */
  onDone?: () => void;
}) {
  const startRef = useRef<number>(
    session.createdAt ? Date.parse(session.createdAt) : Date.now(),
  );
  if (Number.isNaN(startRef.current)) startRef.current = Date.now();

  const [elapsed, setElapsed] = useState(() => Date.now() - startRef.current);
  const [msgIndex, setMsgIndex] = useState(0);
  const finishedRef = useRef(false);

  const rootRef = useRef<HTMLDivElement>(null);
  const fillRef = useRef<HTMLDivElement>(null);
  const msgRef = useRef<HTMLParagraphElement>(null);

  const ready = session.workspaceReady;
  const scheduled = ready || (!!session.phase && session.phase !== "Pending");
  const pct = progressFor(session.phase, ready, elapsed);

  const steps = useMemo(
    () => [
      { label: "request accepted", done: true, active: false },
      { label: "scheduling pod", done: scheduled, active: !scheduled },
      { label: "provisioning cluster", done: ready, active: scheduled && !ready },
      { label: "workspace ready", done: ready, active: false },
    ],
    [scheduled, ready],
  );

  // 1s tick drives elapsed time, which drives the progress target.
  useEffect(() => {
    if (ready) return;
    const t = setInterval(() => setElapsed(Date.now() - startRef.current), 1000);
    return () => clearInterval(t);
  }, [ready]);

  // Glide the fill toward the current target each tick (linear, so back-to-
  // back animations chain into one continuous creep).
  useEffect(() => {
    const el = fillRef.current;
    if (!el) return;
    if (prefersReducedMotion()) {
      el.style.width = `${pct}%`;
      return;
    }
    animate(el, {
      width: `${pct}%`,
      duration: ready ? 450 : 1100,
      ease: ready ? "out(3)" : "linear",
    });
  }, [pct, ready]);

  // Rotate the wait copy while the backend has nothing specific to say.
  useEffect(() => {
    if (ready || session.message) return;
    const t = setInterval(
      () => setMsgIndex((i) => (i + 1) % WAIT_MESSAGES.length),
      8000,
    );
    return () => clearInterval(t);
  }, [ready, session.message]);

  // Fade new copy in as it changes.
  const message = ready
    ? "workspace ready"
    : (session.message ?? WAIT_MESSAGES[msgIndex]);
  useEffect(() => {
    if (msgRef.current && !prefersReducedMotion()) {
      animate(msgRef.current, {
        opacity: [0, 1],
        translateY: [4, 0],
        duration: 350,
        ease: "out(3)",
      });
    }
  }, [message]);

  // Completion: let the 100% fill + success flash land, then collapse.
  useEffect(() => {
    if (!ready || finishedRef.current) return;
    finishedRef.current = true;
    const root = rootRef.current;
    if (prefersReducedMotion() || !root) {
      onDone?.();
      return;
    }
    animate(root, {
      opacity: 0,
      height: 0,
      duration: 400,
      delay: 1400,
      ease: "in(3)",
      onComplete: () => onDone?.(),
    });
  }, [ready, onDone]);

  return (
    <div ref={rootRef} className="overflow-hidden" aria-live="polite">
      <div className="rounded-md border border-border/70 bg-muted/30 p-3 font-mono text-xs">
        <div className="flex items-baseline justify-between gap-2">
          <span className="text-[10px] uppercase tracking-widest text-muted-foreground">
            {ready ? "ready" : "provisioning"}
          </span>
          <span
            className={cn("tabular-nums", ready ? "text-success" : "text-primary")}
          >
            {Math.round(pct)}%
          </span>
        </div>

        <div
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(pct)}
          aria-label="Sandbox provisioning progress"
          className="mt-2 h-1.5 overflow-hidden rounded-full bg-border/70"
        >
          <div
            ref={fillRef}
            className={cn(
              "h-full rounded-full",
              ready ? "bg-success" : "bg-primary",
            )}
            style={{ width: "0%" }}
          />
        </div>

        <ul className="mt-3 space-y-1">
          {steps.map((s) => (
            <li
              key={s.label}
              className={cn(
                "flex gap-2",
                s.done
                  ? "text-success"
                  : s.active
                    ? "text-warning"
                    : "text-muted-foreground/60",
              )}
            >
              <span aria-hidden className="w-8 shrink-0">
                {s.done ? "[ok]" : s.active ? "[··]" : "[  ]"}
              </span>
              {s.label}
              {s.active && <span className="sr-only">(in progress)</span>}
            </li>
          ))}
        </ul>

        <p ref={msgRef} className="mt-2 text-muted-foreground">
          <span className="text-primary">❯</span> {message}
        </p>
        {!ready && (
          <p className="mt-1 text-[10px] text-muted-foreground/70">
            {formatElapsed(elapsed)} elapsed · usually ready in ~
            {Math.round(EXPECTED_MS / 60_000)} min
          </p>
        )}
      </div>
    </div>
  );
}
