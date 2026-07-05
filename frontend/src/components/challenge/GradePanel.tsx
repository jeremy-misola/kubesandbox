import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ConfirmResetDialog } from "@/components/challenge/ConfirmResetDialog";
import { ApiError } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { GradeMutation, ResetMutation } from "@/hooks/useChallenges";
import type { Session } from "@/lib/schemas";

// Matches the backend's per-session min interval (§7/§9). Self-disabling the
// button for this long after each submit makes 429 effectively unreachable from
// a single tab; the luxury system's slow transitions make the 2s disabled state
// feel deliberate rather than laggy.
const COOLDOWN_MS = 2000;

function gradedAtLabel(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

/**
 * The workspace's primary action. Grade → merge the result into the StepList
 * (handled by the parent, which owns the mutation) and show the verdict here.
 * The backend's guards are expected states, not errors (§9): a self-imposed
 * cooldown prevents 429; a 429 that lands anyway (second tab) is a quiet inline
 * note, never a toast or red; 409 seed_in_progress renders nothing here (the
 * mutation invalidates the session and the seeding gate takes over).
 */
export function GradePanel({
  session,
  grade,
  reset,
}: {
  session: Session;
  grade: GradeMutation;
  reset: ResetMutation;
}) {
  const [cooldownUntil, setCooldownUntil] = useState(0);
  const [now, setNow] = useState(() => Date.now());
  const [confirmingReset, setConfirmingReset] = useState(false);

  // Tick only while cooling; the interval clears itself the moment it lapses.
  useEffect(() => {
    if (cooldownUntil <= Date.now()) return;
    const t = setInterval(() => setNow(Date.now()), 100);
    return () => clearInterval(t);
  }, [cooldownUntil]);

  const cooling = now < cooldownUntil;
  const coolPct = cooling ? 100 - ((cooldownUntil - now) / COOLDOWN_MS) * 100 : 100;

  const submit = () => {
    if (cooling || grade.isPending) return;
    const t = Date.now();
    setNow(t);
    setCooldownUntil(t + COOLDOWN_MS);
    grade.mutate();
  };

  const err = grade.error;
  const is429 = err instanceof ApiError && err.status === 429;
  const is409 = err instanceof ApiError && err.status === 409;
  const hardError = !!err && !is429 && !is409;

  const result = grade.data;
  const disabled = grade.isPending || cooling;

  return (
    <Card className="p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="font-mono text-[10px] uppercase tracking-label text-muted-foreground">
            Grading
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            Check your work against the challenge's steps — grade as often as you
            like.
          </p>
        </div>
        <div className="flex flex-col items-stretch">
          <Button onClick={submit} disabled={disabled}>
            {grade.isPending ? (
              <>
                <i
                  aria-hidden
                  className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-primary-foreground/30 border-t-primary-foreground"
                />
                Grading…
              </>
            ) : (
              "Grade challenge"
            )}
          </Button>
          {/* Cooldown affordance: a thin bar that fills over ~2s (§9). */}
          {cooling && !grade.isPending && (
            <span aria-hidden className="mt-1 block h-px w-full bg-border">
              <span
                className="block h-px bg-accent transition-[width] duration-100 ease-linear"
                style={{ width: `${coolPct}%` }}
              />
            </span>
          )}
        </div>
      </div>

      {/* Verdict line. Pass earns the one celebratory moment — gold, sparingly. */}
      {result && (
        <div
          className={cn(
            "mt-4 flex items-center gap-2 border-t pt-4",
            result.pass ? "border-accent/40" : "border-border/60",
          )}
        >
          <i
            aria-hidden
            className={cn(
              "h-2 w-2 rounded-full",
              result.pass ? "bg-accent" : "bg-danger",
            )}
          />
          <p
            className={cn(
              "text-sm",
              result.pass
                ? "font-medium text-accent"
                : "text-foreground/90",
            )}
          >
            {result.pass
              ? "All steps passed — challenge complete."
              : `${result.steps.filter((s) => !s.pass).length} of ${result.steps.length} step${result.steps.length === 1 ? "" : "s"} still to fix.`}
          </p>
          {gradedAtLabel(result.gradedAt) && (
            <span className="ml-auto font-mono text-[10px] uppercase tracking-label text-muted-foreground/70">
              graded {gradedAtLabel(result.gradedAt)}
            </span>
          )}
        </div>
      )}

      {/* 429: quiet, never a failure. The next click works. */}
      {is429 && (
        <p className="mt-3 font-mono text-xs text-muted-foreground">
          Just a moment between grade runs…
        </p>
      )}

      {/* Network / 5xx: inline + retry, house style. 409 renders nothing. */}
      {hardError && (
        <p className="mt-3 text-sm text-danger">
          Couldn't run the grader — please try again in a moment.
        </p>
      )}

      {/* Reset: secondary, confirmed. Rides the existing SSE stream back through
          the seeding gate — no new poll loop (§10). */}
      <div className="mt-4 flex items-center justify-between gap-3 border-t border-border/60 pt-4">
        <p className="min-w-0 text-sm text-danger">
          {reset.isError
            ? "Couldn't reset the challenge — please try again."
            : ""}
        </p>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => setConfirmingReset(true)}
          disabled={reset.isPending}
        >
          {reset.isPending ? "Resetting…" : "Reset challenge"}
        </Button>
      </div>

      {confirmingReset && (
        <ConfirmResetDialog
          session={session}
          onConfirm={() => reset.mutate()}
          onClose={() => setConfirmingReset(false)}
        />
      )}
    </Card>
  );
}
