import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react";
import { animate } from "animejs";

import { Button } from "@/components/ui/button";
import { TerminalChrome } from "@/components/ui/card";
import { useCreateSession } from "@/hooks/useSessions";
import { ApiError } from "@/lib/api";
import { prefersReducedMotion } from "@/lib/utils";
import { SANDBOX_RESOURCES, TTL } from "@/lib/schemas";

function formatTtl(minutes: number): string {
  if (minutes < 60) return `${minutes} min`;
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return m === 0 ? `${h}h` : `${h}h ${m}m`;
}

/**
 * One decision, one dialog: sandboxes are uniform and pre-provisioned (hot
 * pool, rev 20), so the only thing to choose is the lifetime. Create either
 * hands over a running sandbox instantly (201) or places the user in line
 * (202) — both settle on the dashboard, so the dialog closes on success.
 */
export function CreateSessionDialog({ onClose }: { onClose: () => void }) {
  const [ttl, setTtl] = useState<number>(TTL.default);
  const create = useCreateSession();

  const overlayRef = useRef<HTMLDivElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const closingRef = useRef(false);

  // Organic open: overlay fades, panel scales up from slightly below.
  useEffect(() => {
    if (prefersReducedMotion()) return;
    if (overlayRef.current) {
      animate(overlayRef.current, { opacity: [0, 1], duration: 250, ease: "out(2)" });
    }
    if (panelRef.current) {
      animate(panelRef.current, {
        opacity: [0, 1],
        scale: [0.92, 1],
        translateY: [24, 0],
        duration: 400,
        ease: "out(4)",
      });
    }
  }, []);

  // Mirror-image close, then unmount via onClose.
  const requestClose = useCallback(() => {
    if (closingRef.current) return;
    closingRef.current = true;
    if (prefersReducedMotion() || !panelRef.current || !overlayRef.current) {
      onClose();
      return;
    }
    animate(overlayRef.current, { opacity: 0, duration: 200, ease: "in(2)" });
    animate(panelRef.current, {
      opacity: 0,
      scale: 0.94,
      translateY: 16,
      duration: 220,
      ease: "in(3)",
      onComplete: onClose,
    });
  }, [onClose]);

  // Escape closes (unless a create is in flight).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !create.isPending) requestClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [create.isPending, requestClose]);

  const submit = () => {
    // Both outcomes (created / queued) render on the dashboard, which sits
    // right behind this dialog — close and let it take over.
    create.mutate({ ttlMinutes: ttl }, { onSuccess: requestClose });
  };

  const errorMsg =
    create.error instanceof ApiError
      ? create.error.code === "session_exists"
        ? "You already have a sandbox (it may still be cleaning up). Delete it or wait a moment before creating a new one."
        : create.error.message
      : create.error
        ? "Could not create the sandbox. Please try again."
        : null;

  const fillPct = ((ttl - TTL.min) / (TTL.max - TTL.min)) * 100;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && !create.isPending) requestClose();
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="create-session-title"
        className="w-full max-w-lg overflow-hidden rounded-lg border border-border bg-card shadow-card"
      >
        <TerminalChrome title="kubesandbox — create" />

        <div className="p-5">
          <h2
            id="create-session-title"
            className="font-display text-lg font-semibold tracking-tight"
          >
            New sandbox
          </h2>
          <p className="mb-5 mt-1 text-sm text-muted-foreground">
            Sandboxes are kept running and handed over on request — yours is
            usually ready the moment you click create.
          </p>

          {/* The uniform spec, stated up front — no tiers to pick from. */}
          <dl className="grid grid-cols-3 gap-px overflow-hidden rounded-md border border-border/70 bg-border/70 font-mono text-xs">
            {(
              [
                ["cluster", "private vcluster"],
                ["cpu", SANDBOX_RESOURCES.cpu],
                ["mem", SANDBOX_RESOURCES.memory],
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

          <div className="mb-2 mt-5 flex items-baseline justify-between">
            <label
              htmlFor="ttl-slider"
              className="font-mono text-[11px] uppercase tracking-widest text-muted-foreground"
            >
              Lifetime
            </label>
            <span className="font-mono text-sm text-primary">{formatTtl(ttl)}</span>
          </div>
          <input
            id="ttl-slider"
            type="range"
            min={TTL.min}
            max={TTL.max}
            step={15}
            value={ttl}
            onChange={(e) => setTtl(Number(e.target.value))}
            className="ttl-slider"
            style={{ "--fill": `${fillPct}%` } as CSSProperties}
            aria-valuetext={formatTtl(ttl)}
          />
          <div className="mt-1 flex justify-between font-mono text-[10px] text-muted-foreground">
            <span>{TTL.min}m</span>
            <span>the clock starts now — auto-deletes at zero</span>
            <span>{formatTtl(TTL.max)}</span>
          </div>

          {errorMsg && (
            <p
              role="alert"
              className="mt-4 rounded-md border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger"
            >
              {errorMsg}
            </p>
          )}

          <div className="mt-6 flex justify-end gap-2">
            <Button variant="ghost" onClick={requestClose} disabled={create.isPending}>
              Cancel
            </Button>
            <Button onClick={submit} disabled={create.isPending}>
              {create.isPending ? (
                <>
                  <i
                    aria-hidden
                    className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-primary-foreground/30 border-t-primary-foreground"
                  />
                  Creating…
                </>
              ) : (
                "Create sandbox"
              )}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
