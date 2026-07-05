import { useEffect } from "react";

import { Button } from "@/components/ui/button";
import { TerminalChrome } from "@/components/ui/card";
import { useDialogTransition } from "@/hooks/useDialogTransition";
import type { Session } from "@/lib/schemas";

/**
 * Confirm before resetting a challenge. Reset destroys the user's modifications
 * to the seeded objects and re-applies the bundle from scratch, so it gets an
 * explicit confirm (same posture as delete). Copy reflects the backend caveat:
 * only the labeled seed state is torn down — unlabeled objects the user created
 * themselves survive (backend §7).
 */
export function ConfirmResetDialog({
  session,
  onConfirm,
  onClose,
}: {
  session: Session;
  onConfirm: () => void;
  onClose: () => void;
}) {
  const { overlayRef, panelRef, requestClose } = useDialogTransition(onClose);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") requestClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [requestClose]);

  const title = session.challenge?.title ?? "this challenge";

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-[60] flex items-center justify-center bg-[#1A1A1A]/75 p-4 backdrop-blur-sm"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) requestClose();
      }}
    >
      <div
        ref={panelRef}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-reset-title"
        aria-describedby="confirm-reset-desc"
        className="dark w-full max-w-md overflow-hidden border border-border bg-card text-card-foreground shadow-hero"
      >
        <TerminalChrome title={`${session.id} — reset`} />

        <div className="p-6">
          <h2
            id="confirm-reset-title"
            className="font-display text-2xl font-normal tracking-tight"
          >
            Reset this challenge?
          </h2>
          <p
            id="confirm-reset-desc"
            className="mt-3 text-sm font-light text-muted-foreground"
          >
            This tears down the challenge's seeded objects and re-applies{" "}
            <span className="font-mono text-xs text-foreground">{title}</span>{" "}
            fresh — any changes you made to them are lost. Objects you created
            yourself are left alone. It takes a few seconds.
          </p>

          <div className="mt-6 flex justify-end gap-2">
            <Button variant="ghost" autoFocus onClick={() => requestClose()}>
              Cancel
            </Button>
            <Button variant="danger" onClick={() => requestClose(onConfirm)}>
              Reset challenge
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
