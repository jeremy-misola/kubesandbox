import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api } from "@/lib/api";
import { queryKeys } from "@/lib/queryClient";
import type { CreateSessionRequest, Session } from "@/lib/schemas";

export function useSessions() {
  return useQuery({
    queryKey: queryKeys.sessions,
    queryFn: api.listSessions,
  });
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
      qc.invalidateQueries({ queryKey: queryKeys.sessions });
    },
  });
}

export function useDeleteSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteSession(id),
    // Optimistic removal from the list (design-principles §2).
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: queryKeys.sessions });
      const prev = qc.getQueryData<Session[]>(queryKeys.sessions);
      qc.setQueryData<Session[]>(queryKeys.sessions, (old) =>
        (old ?? []).filter((s) => s.id !== id),
      );
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) qc.setQueryData(queryKeys.sessions, ctx.prev);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: queryKeys.sessions });
    },
  });
}
