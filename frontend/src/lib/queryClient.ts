import { QueryClient } from "@tanstack/react-query";

import { ApiError } from "@/lib/api";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 10_000,
      retry: (failureCount, error) => {
        // Don't retry auth/validation/not-found errors.
        if (error instanceof ApiError && error.status < 500) return false;
        return failureCount < 2;
      },
      refetchOnWindowFocus: false,
    },
  },
});

export const queryKeys = {
  sessions: ["sessions"] as const,
  session: (id: string) => ["session", id] as const,
  queue: ["queue"] as const,
  challenges: ["challenges"] as const,
  // hints in the key so a reveal refetches exactly once and back-navigation
  // stays cached.
  challenge: (id: string, hints: number) => ["challenge", id, hints] as const,
};
