import { useLayoutEffect, useRef } from "react";
import { animate, stagger, utils } from "animejs";

import { prefersReducedMotion } from "@/lib/utils";

/**
 * Subtle staggered entrance for a container's children. On mount (and whenever
 * `deps` change) the matched elements fade and lift into place, one after the
 * next — the editorial system's slow, deliberate motion applied to lists and
 * card grids. A no-op under prefers-reduced-motion: content simply stays put.
 *
 * `selector` defaults to direct children (`:scope > *`), so it drops onto a grid
 * or list wrapper without tagging every item.
 */
export function useReveal<T extends HTMLElement = HTMLDivElement>(
  deps: unknown[] = [],
  selector = ":scope > *",
) {
  const ref = useRef<T>(null);

  useLayoutEffect(() => {
    const root = ref.current;
    if (!root) return;

    const targets = root.querySelectorAll<HTMLElement>(selector);
    if (!targets.length || prefersReducedMotion()) return;

    // Set the pre-animation state before paint to avoid a flash of final layout.
    utils.set(targets, { opacity: 0, translateY: 14 });
    animate(targets, {
      opacity: [0, 1],
      translateY: [14, 0],
      duration: 620,
      ease: "out(3)",
      delay: stagger(70),
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return ref;
}
