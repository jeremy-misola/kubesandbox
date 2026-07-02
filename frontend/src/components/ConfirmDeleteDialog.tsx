import { useCallback, useEffect, useRef } from "react";
import { animate } from "animejs";

import { Button } from "@/components/ui/button";
import { TerminalChrome } from "@/components/ui/card";
import { prefersReducedMotion } from "@/lib/utils";
import type { Session } from "@/lib/schemas";

/** Confirmation step before destroying a sandbox. Deletion is irreversible,
 *  so it gets an explicit confirm (destructive actions shouldn't be one
 *  accidental click away). Once confirmed, the deletion itself is optimistic —
 *  the dialog closes and the card disappears immediately. */
export function ConfirmDeleteDialog({
  session,
  onConfirm,
  onClose,
}: {
  session: Session;
  onConfirm: () => void;
  onClose: () => void;
}) {
  const overlayRef = useRef<HTMLDivElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const closingRef = useRef(false);

  // Same open/close motion as CreateSessionDialog for consistency.
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

  const requestClose = useCallback(
    (after?: () => void) => {
      if (closingRef.current) return;
      closingRef.current = true;
      const done = () => {
        after?.();
        onClose();
      };
      if (prefersReducedMotion() || !panelRef.current || !overlayRef.current) {
        done();
        return;
      }
      animate(overlayRef.current, { opacity: 0, duration: 200, ease: "in(2)" });
      animate(panelRef.current, {
        opacity: 0,
        scale: 0.94,
        translateY: 16,
        duration: 220,
        ease: "in(3)",
        onComplete: done,
      });
    },
    [onClose],
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") requestClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [requestClose]);

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) requestClose();
      }}
    >
      <div
        ref={panelRef}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-delete-title"
        aria-describedby="confirm-delete-desc"
        className="w-full max-w-md overflow-hidden rounded-lg border border-border bg-card shadow-card"
      >
        <TerminalChrome title={`${session.id} — delete`} />

        <div className="p-5">
          <h2
            id="confirm-delete-title"
            className="font-display text-lg font-semibold tracking-tight"
          >
            Delete this sandbox?
          </h2>
          <p id="confirm-delete-desc" className="mt-2 text-sm text-muted-foreground">
            This destroys{" "}
            <span className="font-mono text-xs text-foreground">{session.id}</span>{" "}
            and everything inside it. This can't be undone.
          </p>

          <div className="mt-6 flex justify-end gap-2">
            <Button variant="ghost" autoFocus onClick={() => requestClose()}>
              Cancel
            </Button>
            <Button variant="danger" onClick={() => requestClose(onConfirm)}>
              Delete sandbox
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
