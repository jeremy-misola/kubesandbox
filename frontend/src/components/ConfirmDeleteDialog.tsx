import { useEffect } from "react";

import { Button } from "@/components/ui/button";
import { TerminalChrome } from "@/components/ui/card";
import { useDialogTransition } from "@/hooks/useDialogTransition";
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
  // Same open/close motion as CreateSessionDialog for consistency.
  const { overlayRef, panelRef, requestClose } = useDialogTransition(onClose);

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
      className="fixed inset-0 z-[60] flex items-center justify-center bg-[#1A1A1A]/75 p-4 backdrop-blur-sm"
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
        className="dark w-full max-w-md overflow-hidden border border-border bg-card text-card-foreground shadow-hero"
      >
        <TerminalChrome title={`${session.id} — delete`} />

        <div className="p-6">
          <h2
            id="confirm-delete-title"
            className="font-display text-2xl font-normal tracking-tight"
          >
            Delete this sandbox?
          </h2>
          <p id="confirm-delete-desc" className="mt-3 text-sm font-light text-muted-foreground">
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
