import { useCallback, useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { ApiError, api } from "@/lib/api";
import { queryKeys } from "@/lib/queryClient";
import type { Session } from "@/lib/schemas";

// Challenge queries + the shared hint-reveal counter. Same layering as
// useSessions/useQueue: plain TanStack queries over the typed api client, Zod
// already applied at the boundary. Grade + reset mutations live alongside these
// (added with their work items).

/**
 * The challenge catalog. Content changes via git push, not user action, so the
 * staleTime is generous; refetchOnWindowFocus is harmless and keeps a
 * long-lived tab current.
 */
export function useChallenges() {
  return useQuery({
    queryKey: queryKeys.challenges,
    queryFn: api.listChallenges,
    staleTime: 5 * 60_000,
    refetchOnWindowFocus: true,
  });
}

/**
 * One challenge's detail. `hints` is part of the key (queryKeys.challenge) so a
 * reveal refetches exactly once with ?hints=n and back-navigation stays cached.
 */
export function useChallenge(id: string, hints = 0) {
  return useQuery({
    queryKey: queryKeys.challenge(id, hints),
    queryFn: () => api.getChallenge(id, hints),
    enabled: !!id,
    staleTime: 5 * 60_000,
  });
}

/**
 * Revealed-hint count for a challenge, backed by sessionStorage keyed by
 * challenge id (`kubesandbox.hints.<id>`). Shared by the detail page and the
 * in-session instructions panel so hints revealed before starting stay revealed
 * inside the workspace. Per-tab and gone on tab close — consistent with the
 * token-storage posture, and deliberately one-way within a tab (revealing only
 * ever increments). Returns [count, reveal].
 */
export function useRevealedHints(challengeId: string): [number, () => void] {
  const key = `kubesandbox.hints.${challengeId}`;
  const read = useCallback(() => {
    try {
      return Math.max(0, Number(sessionStorage.getItem(key)) || 0);
    } catch {
      return 0;
    }
  }, [key]);

  const [count, setCount] = useState(read);

  // Re-sync when the challenge changes (the hook may be reused across ids).
  useEffect(() => {
    setCount(read());
  }, [read]);

  const reveal = useCallback(() => {
    setCount((c) => {
      const next = c + 1;
      try {
        sessionStorage.setItem(key, String(next));
      } catch {
        /* private mode / storage disabled — the in-memory count still holds */
      }
      return next;
    });
  }, [key]);

  return [count, reveal];
}

// ---- grading ---------------------------------------------------------------

/**
 * On-demand grade. The result is retained in the mutation state (not the query
 * cache): grade results are ephemeral by backend design (v1 has no
 * persistence), and grading has no side effects on the session payload, so
 * there is nothing to write back. The one exception is 409 seed_in_progress —
 * the claim's seed state regressed since render (a reset raced from elsewhere),
 * so we refetch the session and let the re-engaged seeding gate be the UI for
 * that state. 429 and other outcomes are surfaced inline by GradePanel (§9).
 */
export function useGradeChallenge(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.gradeChallenge(sessionId),
    onError: (err) => {
      if (err instanceof ApiError && err.status === 409) {
        qc.invalidateQueries({ queryKey: queryKeys.session(sessionId) });
      }
    },
  });
}

export type GradeMutation = ReturnType<typeof useGradeChallenge>;

// ---- reset -----------------------------------------------------------------

/**
 * Reset the challenge state. The 202 body is discarded — the session SSE stream
 * is the progress channel. The optimistic `seedState: "pending"` write is what
 * makes the whole flow work with no further code: it drops the seeding gate
 * (workspace shows SeedingNotice) and re-arms the widened useSessionEvents
 * enablement in TerminalPage, so the existing stream carries the authoritative
 * pending → seeding → seeded transitions. No new poll loop anywhere (§10).
 */
export function useResetChallenge(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.resetChallenge(sessionId),
    onMutate: async () => {
      await qc.cancelQueries({ queryKey: queryKeys.session(sessionId) });
      const prev = qc.getQueryData<Session>(queryKeys.session(sessionId));
      if (prev?.challenge) {
        qc.setQueryData<Session>(queryKeys.session(sessionId), {
          ...prev,
          challenge: { ...prev.challenge, seedState: "pending" },
        });
      }
      return { prev };
    },
    // 409 raced / 5xx: roll back the optimistic write, then invalidate so the
    // authoritative session payload lands. GradePanel surfaces the inline error.
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) {
        qc.setQueryData(queryKeys.session(sessionId), ctx.prev);
      }
      qc.invalidateQueries({ queryKey: queryKeys.session(sessionId) });
    },
  });
}

export type ResetMutation = ReturnType<typeof useResetChallenge>;
