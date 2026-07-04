import { useCallback, useEffect, useRef } from "react";
import { animate } from "animejs";

import { prefersReducedMotion } from "@/lib/utils";

/**
 * Shared open/close motion for modal dialogs: overlay fades, panel scales up
 * from slightly below on mount; both mirror back out before `onClose` fires.
 * Used by CreateSessionDialog and ConfirmDeleteDialog so both share one
 * transition instead of two copies of the same animation code.
 */
export function useDialogTransition(onClose: () => void) {
  const overlayRef = useRef<HTMLDivElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const closingRef = useRef(false);

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

  return { overlayRef, panelRef, requestClose };
}
