import { useSyncExternalStore } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api } from "@/lib/api";
import { queryKeys } from "@/lib/queryClient";
import type { CreateSessionRequest, Session } from "@/lib/schemas";

// ---- pending deletions -----------------------------------------------------
//
// The backend deletes sandboxes asynchronously: DELETE /sessions/:id returns
// before the session is fully gone, so a refetch right after can still include
// it and "resurrect" the card the user just deleted. We track optimistically
// deleted ids here and hide them from every list render until the server stops
// returning them (frontend-design §2.8, design-principles §2.4 Optimistic UI).
// On failure the id is cleared, so the card rolls back into view.

const pendingDeletes = new Set<string>();
let pendingSnapshot: readonly string[] = [];
const pendingListeners = new Set<() => void>();

function notifyPending() {
  pendingSnapshot = [...pendingDeletes];
  pendingListeners.forEach((l) => l());
}

function addPendingDelete(id: string) {
  if (pendingDeletes.has(id)) return;
  pendingDeletes.add(id);
  notifyPending();
}

function clearPendingDelete(id: string) {
  if (!pendingDeletes.delete(id)) return;
  notifyPending();
}

function usePendingDeletes(): readonly string[] {
  return useSyncExternalStore(
    (cb) => {
      pendingListeners.add(cb);
      return () => pendingListeners.delete(cb);
    },
    () => pendingSnapshot,
  );
}

// ---- queries ----------------------------------------------------------------

export function useSessions() {
  const pending = usePendingDeletes();

  const query = useQuery({
    queryKey: queryKeys.sessions,
    queryFn: async () => {
      const sessions = await api.listSessions();
      // Only a real server response may confirm a deletion: once the backend
      // stops returning a pending-delete id, that session is truly gone.
      // (Optimistic cache writes never reach this code path, so they can't
      // clear an id prematurely.)
      const live = new Set(sessions.map((s) => s.id));
      for (const id of [...pendingDeletes]) {
        if (!live.has(id)) clearPendingDelete(id);
      }
      return sessions;
    },
    // While deletions are settling on the backend, poll until they stop
    // coming back so the pending set converges.
    refetchInterval: pending.length > 0 ? 2000 : false,
  });

  return {
    ...query,
    data: query.data?.filter((s) => !pending.includes(s.id)),
  };
}

export function useSession(id: string) {
  return useQuery({
    queryKey: queryKeys.session(id),
    queryFn: () => api.getSession(id),
    enabled: !!id,
  });
}

export function useCreateSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateSessionRequest) => api.createSession(body),
    onSuccess: (session) => {
      qc.setQueryData(queryKeys.session(session.id), session);
      // Seed the list cache so the new card (and its provisioning progress)
      // appears the instant the dialog closes, not after the refetch
      // (design-principles §2.4 Optimistic UI — here with a confirmed row).
      qc.setQueryData<Session[]>(queryKeys.sessions, (old) =>
        old && !old.some((s) => s.id === session.id) ? [...old, session] : old,
      );
      qc.invalidateQueries({ queryKey: queryKeys.sessions });
    },
  });
}

export function useDeleteSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteSession(id),
    // Optimistic removal (design-principles §2.4): the card disappears
    // immediately; `pendingDeletes` keeps it hidden even if a refetch still
    // returns it while the backend finishes tearing it down.
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: queryKeys.sessions });
      const prev = qc.getQueryData<Session[]>(queryKeys.sessions);
      addPendingDelete(id);
      qc.setQueryData<Session[]>(queryKeys.sessions, (old) =>
        (old ?? []).filter((s) => s.id !== id),
      );
      return { prev };
    },
    // Roll back on failure: un-hide the session so the card reappears.
    onError: (_err, id, ctx) => {
      clearPendingDelete(id);
      if (ctx?.prev) qc.setQueryData(queryKeys.sessions, ctx.prev);
    },
    onSuccess: (_data, id) => {
      qc.removeQueries({ queryKey: queryKeys.session(id) });
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: queryKeys.sessions });
    },
  });
}
