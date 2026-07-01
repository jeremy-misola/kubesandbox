import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { streamSessionEvents } from "@/lib/api";
import { queryKeys } from "@/lib/queryClient";
import type { Session } from "@/lib/schemas";

/**
 * Subscribes to the session's SSE stream and pushes live updates into the query
 * cache. Falls back to polling GET /sessions/:id if streaming fails
 * (docs/06 §4.3, design-principles §7).
 */
export function useSessionEvents(id: string, enabled = true) {
  const qc = useQueryClient();

  useEffect(() => {
    if (!id || !enabled) return;
    const ctrl = new AbortController();
    let pollTimer: ReturnType<typeof setInterval> | undefined;

    const startPolling = () => {
      if (pollTimer) return;
      pollTimer = setInterval(() => {
        qc.invalidateQueries({ queryKey: queryKeys.session(id) });
      }, 5000);
    };

    streamSessionEvents(
      id,
      (e) => {
        if (e.type === "update") {
          qc.setQueryData<Session>(queryKeys.session(id), e.session);
          qc.setQueryData<Session[]>(queryKeys.sessions, (old) =>
            (old ?? []).map((s) => (s.id === e.session.id ? e.session : s)),
          );
        } else if (e.type === "deleted") {
          qc.setQueryData<Session[]>(queryKeys.sessions, (old) =>
            (old ?? []).filter((s) => s.id !== e.session.id),
          );
        } else {
          startPolling();
        }
      },
      ctrl.signal,
    ).catch(() => {
      if (!ctrl.signal.aborted) startPolling();
    });

    return () => {
      ctrl.abort();
      if (pollTimer) clearInterval(pollTimer);
    };
  }, [id, enabled, qc]);
}
