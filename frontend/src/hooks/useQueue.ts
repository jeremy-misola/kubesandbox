import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { api, streamQueueEvents } from "@/lib/api";
import { queryKeys } from "@/lib/queryClient";
import type { QueueStatus, Session } from "@/lib/schemas";

// Warm-pool queue state (backend Phase E). When every hot sandbox is taken,
// POST /sessions returns 202 and the user waits in a FIFO line. This hook is
// the single source of truth for that wait:
//
//   - the cache entry under queryKeys.queue holds the QueueStatus (or null),
//     written by useCreateSandbox (202 response) and refreshed here;
//   - while queued, an SSE stream (/api/queue/events) pushes live position
//     updates and, on hand-over, the assigned session straight into the
//     session cache — the dashboard swaps the queue card for the sandbox
//     card without a refetch;
//   - if the stream can't be held open, we degrade to polling every few
//     seconds (design-principles §7 — graceful degradation, never a dead UI).
//
// The queue is also re-checked on mount so a page reload while waiting
// restores the "in line" view instead of an empty dashboard.

/** Read (and keep fresh) the caller's place in line. */
export function useQueueStatus() {
  return useQuery<QueueStatus | null>({
    queryKey: queryKeys.queue,
    queryFn: api.getQueueStatus,
    // The SSE watcher below is the primary update channel; this is the
    // mount-time check + polling backstop.
    staleTime: 5_000,
  });
}

/**
 * Mount once (dashboard): while the user is queued, follow the SSE stream
 * and settle the queue into either an assigned session or an error.
 * Returns a user-facing error message when the line failed, if any.
 */
export function useQueueWatcher(): { queueError: string | null } {
  const qc = useQueryClient();
  const { data: queued } = useQueueStatus();
  // Key the effect on the boolean, not the object: position updates write new
  // QueueStatus objects into the cache, and re-running the effect for each
  // would tear down and reopen the SSE stream on every tick.
  const isQueued = !!queued;
  const [queueError, setQueueError] = useState<string | null>(null);

  useEffect(() => {
    if (!isQueued) return;
    setQueueError(null);

    const ctrl = new AbortController();
    let pollTimer: ReturnType<typeof setInterval> | undefined;

    const settleAssigned = (session?: Session) => {
      if (session) {
        qc.setQueryData(queryKeys.session(session.id), session);
      }
      qc.setQueryData(queryKeys.queue, null);
      qc.invalidateQueries({ queryKey: queryKeys.sessions });
    };

    // Fallback: poll the queue position; leaving the queue (404 → null)
    // means either assigned (sessions refetch finds it) or failed.
    const startPolling = () => {
      if (pollTimer) return;
      pollTimer = setInterval(async () => {
        try {
          const status = await api.getQueueStatus();
          qc.setQueryData(queryKeys.queue, status);
          if (!status) settleAssigned();
        } catch {
          /* keep polling; the queue query's own retry surfaces hard errors */
        }
      }, 3000);
    };

    streamQueueEvents(
      (e) => {
        if (e.type === "queued") {
          const next: QueueStatus = { status: "queued", position: e.position };
          qc.setQueryData<QueueStatus | null>(queryKeys.queue, next);
        } else if (e.type === "assigned") {
          settleAssigned(e.session);
        } else {
          // Terminal error from the backend (e.g. a session appeared through
          // another tab). Clear the line and let the dashboard explain.
          setQueueError(e.message);
          settleAssigned();
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
  }, [isQueued, qc]);

  return { queueError };
}
