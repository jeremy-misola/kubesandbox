import { useEffect } from "react";

import { GradePanel } from "@/components/challenge/GradePanel";
import { InstructionsPanel } from "@/components/challenge/InstructionsPanel";
import { SeedingNotice } from "@/components/challenge/SeedingNotice";
import { TerminalFrame } from "@/components/TerminalFrame";
import { useGradeChallenge, useResetChallenge } from "@/hooks/useChallenges";
import { isChallengeSeeded, type Session } from "@/lib/schemas";

/** The gated left column mirrors the loaded instructions geometry with skeleton
 *  lines while seeding (design §6.6) — same discipline as the provisioning
 *  state, so the layout registers before the environment is ready. Kept in step
 *  with InstructionsSkeleton: header block over a divided requirements list. */
function GatedInstructionsSkeleton() {
  return (
    <div className="space-y-10">
      <div className="space-y-3">
        <div className="skeleton h-3 w-24" />
        <div className="skeleton h-5 w-16" />
        <div className="skeleton mt-4 h-4 w-full" />
        <div className="skeleton h-4 w-5/6" />
      </div>
      <div className="space-y-3 border-t border-border/60 pt-8">
        <div className="skeleton h-3 w-28" />
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex gap-3">
            <div className="skeleton mt-1.5 h-2 w-2 rounded-full" />
            <div className="skeleton h-4 w-3/4" />
          </div>
        ))}
      </div>
    </div>
  );
}

/**
 * The challenge session layout, rendered by TerminalPage when session.challenge
 * is present. Asymmetric two-column per the design system's no-50/50 rule:
 * instructions ~4 cols left, terminal ~8 cols right; stacked on mobile. The two
 * panes are set apart with a generous column gap and a hairline vertical rule
 * (with breathing room inside the instructions column) so the reading side
 * never feels crowded against the terminal.
 *
 * The seeding gate is the exact parallel of the provisioning gate: while
 * !isChallengeSeeded the terminal is NOT mounted and the right column shows the
 * SeedingNotice, with skeleton instruction lines on the left. The gate lifts
 * when the session SSE stream flips seedState to "seeded". Grading + reset are
 * layered on by later work items.
 */
export function ChallengeWorkspace({ session }: { session: Session }) {
  const challenge = session.challenge;
  // The grade mutation is owned here so its retained result feeds two places:
  // the verdict in GradePanel and the per-step merge in the InstructionsPanel's
  // StepList (§6.4 — one list, not two). Reset is owned here too so its state
  // survives the GradePanel unmount while the gate is down.
  const grade = useGradeChallenge(session.id);
  const reset = useResetChallenge(session.id);

  const seeded = isChallengeSeeded(session);

  // When the gate goes down (a reset cycling through Seeding), clear the
  // retained grade result — the old verdict describes a cluster that no longer
  // exists (§10 step 4).
  const gradeReset = grade.reset;
  useEffect(() => {
    if (!seeded) gradeReset();
  }, [seeded, gradeReset]);

  if (!challenge) return null;

  return (
    <div className="grid min-h-0 flex-1 grid-cols-1 gap-8 lg:grid-cols-12 lg:gap-10">
      <aside className="min-h-0 lg:col-span-4 lg:overflow-y-auto lg:border-r lg:border-border/60 lg:pb-8 lg:pr-8">
        {seeded ? (
          <InstructionsPanel
            challengeId={challenge.id}
            gradeSteps={grade.data?.steps}
          />
        ) : (
          <GatedInstructionsSkeleton />
        )}
      </aside>

      <div className="flex min-h-0 flex-col gap-5 lg:col-span-8">
        {seeded ? (
          <>
            <TerminalFrame id={session.id} className="min-h-[420px] flex-1" />
            <GradePanel session={session} grade={grade} reset={reset} />
          </>
        ) : (
          <SeedingNotice session={session} />
        )}
      </div>
    </div>
  );
}
