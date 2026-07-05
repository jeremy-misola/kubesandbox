import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Card, TerminalChrome } from "@/components/ui/card";
import { ConfirmDeleteDialog } from "@/components/ConfirmDeleteDialog";
import { HintReveal } from "@/components/challenge/HintReveal";
import { StepList } from "@/components/challenge/StepList";
import { useChallenge, useRevealedHints } from "@/hooks/useChallenges";
import { useReveal } from "@/hooks/useReveal";
import {
  useCreateSession,
  useDeleteSession,
  useSessions,
} from "@/hooks/useSessions";
import { ApiError, api } from "@/lib/api";
import { queryKeys } from "@/lib/queryClient";
import { cn } from "@/lib/utils";
import { TTL, type CreateSessionResult } from "@/lib/schemas";

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

function difficultyTone(difficulty: string): string {
  switch (difficulty.toLowerCase()) {
    case "easy":
      return "border-success/30 bg-success/10 text-success";
    case "medium":
      return "border-warning/30 bg-warning/10 text-warning";
    case "hard":
      return "border-danger/30 bg-danger/10 text-danger";
    default:
      return "border-border bg-muted text-muted-foreground";
  }
}

const backLink = (
  <Link
    to="/challenges"
    className="overline text-muted-foreground transition-colors duration-500 ease-luxury hover:text-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
  >
    ← challenges
  </Link>
);

export function ChallengeDetailPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const qc = useQueryClient();

  const [revealed, reveal] = useRevealedHints(id);
  const { data, isLoading, isError, refetch } = useChallenge(id, revealed);

  // Stagger the intro (category → title → meta → description) in on load.
  const introRef = useReveal<HTMLDivElement>([data?.id]);

  const { data: sessions } = useSessions();
  const existing = sessions?.[0];

  const create = useCreateSession();
  const del = useDeleteSession();

  const [startError, setStartError] = useState<string | null>(null);
  const [confirmingEnd, setConfirmingEnd] = useState(false);
  const [switching, setSwitching] = useState(false);

  const routeResult = (res: CreateSessionResult) => {
    if (res.outcome === "created") navigate(`/terminal/${res.session.id}`);
    else navigate("/dashboard"); // 202 queued: the dashboard's QueueCard takes over
  };

  const handleStartError = (err: unknown) => {
    if (err instanceof ApiError && err.status === 409) {
      // Raced from another tab: a sandbox now exists. Refetch so the explicit
      // choice (open / end & start) renders.
      qc.invalidateQueries({ queryKey: queryKeys.sessions });
    } else if (err instanceof ApiError && err.status === 400) {
      // Unknown challengeId — the catalog went stale mid-click (§4).
      qc.invalidateQueries({ queryKey: queryKeys.challenges });
      setStartError(
        "This challenge is no longer available — the catalog has been refreshed.",
      );
    } else {
      setStartError("Couldn't start the challenge. Please try again.");
    }
  };

  const start = async () => {
    setStartError(null);
    try {
      routeResult(
        await create.mutateAsync({ ttlMinutes: TTL.default, challengeId: id }),
      );
    } catch (err) {
      handleStartError(err);
    }
  };

  // End the current sandbox and start this challenge. Deletion is async
  // (pendingDeletes): wait for the server to stop returning the session before
  // creating, rather than hammering create into 409s (§5.2).
  const endAndStart = async (existingId: string) => {
    setStartError(null);
    setSwitching(true);
    try {
      await del.mutateAsync(existingId);
      for (let i = 0; i < 30; i++) {
        const live = await api.listSessions();
        if (!live.some((s) => s.id === existingId)) break;
        await sleep(1000);
      }
      routeResult(
        await create.mutateAsync({ ttlMinutes: TTL.default, challengeId: id }),
      );
    } catch (err) {
      handleStartError(err);
    } finally {
      setSwitching(false);
    }
  };

  const busy = create.isPending || switching || del.isPending;

  if (isLoading) {
    return (
      <div>
        <div className="pb-6">{backLink}</div>
        <div className="skeleton h-4 w-24" />
        <div className="skeleton mt-4 h-9 w-2/3" />
        <div className="skeleton mt-4 h-4 w-full" />
        <div className="skeleton mt-2 h-4 w-5/6" />
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div>
        <div className="pb-6">{backLink}</div>
        <Card className="border-danger/30">
          <div className="p-6 text-sm">
            <p className="text-danger">Couldn't load this challenge.</p>
            <button
              className="mt-2 font-mono text-xs text-accent underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
              onClick={() => refetch()}
            >
              retry →
            </button>
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="max-w-4xl">
      <div className="pb-6">{backLink}</div>

      <div ref={introRef}>
        <p className="overline text-muted-foreground">{data.category}</p>
        <h1 className="mt-3 font-display text-4xl font-normal leading-tight tracking-tight">
          {data.title}
        </h1>

        <div className="overline-xs mt-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-muted-foreground">
          <span className={cn("border px-2 py-0.5", difficultyTone(data.difficulty))}>
            {data.difficulty}
          </span>
          {data.estMinutes > 0 && <span>· {data.estMinutes} min</span>}
          {data.tags.length > 0 && (
            <span className="tracking-normal text-muted-foreground">
              · {data.tags.join(" · ")}
            </span>
          )}
        </div>

        <p className="mt-6 text-base leading-relaxed text-foreground/90">
          {data.description}
        </p>
      </div>

      <section className="mt-10">
        <p className="overline mb-3 text-muted-foreground">Steps</p>
        <StepList steps={data.steps} />
      </section>

      {data.hintsTotal > 0 && (
        <section className="mt-10">
          <HintReveal
            hintsTotal={data.hintsTotal}
            hints={data.hints}
            revealed={revealed}
            onReveal={reveal}
          />
        </section>
      )}

      {/* Start flow. */}
      <Card className="mt-12 overflow-hidden p-0">
        <TerminalChrome title={`challenge — ${data.id}`} />
        <div className="p-6">
          {existing ? (
            <>
              <h2 className="font-display text-xl font-normal tracking-tight">
                You already have a sandbox
              </h2>
              <p className="mt-2 text-sm text-muted-foreground">
                One sandbox per user. Open the one you have, or end it and start
                this challenge fresh.
              </p>
              <div className="mt-5 flex flex-wrap gap-2">
                <Link to={`/terminal/${existing.id}`}>
                  <Button variant="secondary" disabled={busy}>
                    Open current sandbox
                  </Button>
                </Link>
                <Button onClick={() => setConfirmingEnd(true)} disabled={busy}>
                  {switching ? "Switching sandbox…" : "End it & start this challenge"}
                </Button>
              </div>
            </>
          ) : (
            <>
              <h2 className="font-display text-xl font-normal tracking-tight">
                Start this challenge
              </h2>
              <p className="mt-2 text-sm text-muted-foreground">
                A fresh sandbox is handed over and seeded with this scenario —
                usually ready in seconds.
              </p>
              <div className="mt-5">
                <Button onClick={start} disabled={busy}>
                  {create.isPending ? (
                    <>
                      <i
                        aria-hidden
                        className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-primary-foreground/30 border-t-primary-foreground"
                      />
                      Starting…
                    </>
                  ) : (
                    "Start challenge"
                  )}
                </Button>
              </div>
            </>
          )}

          {startError && (
            <p
              role="alert"
              className="mt-4 rounded-md border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger"
            >
              {startError}
            </p>
          )}
        </div>
      </Card>

      {confirmingEnd && existing && (
        <ConfirmDeleteDialog
          session={existing}
          onConfirm={() => endAndStart(existing.id)}
          onClose={() => setConfirmingEnd(false)}
        />
      )}
    </div>
  );
}
