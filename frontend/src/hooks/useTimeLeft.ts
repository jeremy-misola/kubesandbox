import { useEffect, useState } from "react";

import { timeLeft } from "@/lib/utils";

/**
 * Live "time left" until an RFC3339 expiry, recomputed every second so the
 * countdown ticks in real time rather than freezing at the value it held on
 * mount. Returns the same clock string `timeLeft` produces (e.g. "12:04 left"),
 * or null when there is no expiry. The interval self-clears once it lapses,
 * so an expired session isn't left ticking forever.
 */
export function useTimeLeft(expiresAt?: string): string | null {
  // A bare tick counter: the value we render is derived from `timeLeft` on
  // every render, and each tick is only here to force one.
  const [, setTick] = useState(0);

  useEffect(() => {
    if (!expiresAt) return;
    if (new Date(expiresAt).getTime() - Date.now() <= 0) return;
    const id = setInterval(() => {
      setTick((n) => n + 1);
      if (new Date(expiresAt).getTime() - Date.now() <= 0) clearInterval(id);
    }, 1000);
    return () => clearInterval(id);
  }, [expiresAt]);

  return timeLeft(expiresAt);
}
