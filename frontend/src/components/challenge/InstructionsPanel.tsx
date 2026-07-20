import { HintReveal } from "@/components/challenge/HintReveal";
import { StepList } from "@/components/challenge/StepList";
import { useChallenge, useRevealedHints } from "@/hooks/useChallenges";
import { cn, difficultyTone } from "@/lib/utils";
import type { GradeStep } from "@/lib/schemas";

/** Skeleton mirroring the loaded panel geometry — header block above a
 *  divided requirements section (design-principles §2). */
function InstructionsSkeleton() {
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
 * The quiet left column of the workspace: description, the step checklist, and
 * progressive hints. It fetches the challenge detail keyed on the revealed-hint
 * count — served from the query cache when the user arrived via the detail page.
 * `gradeSteps`, when present (workspace grading), merges pass/fail state into the
 * same StepList.
 */
export function InstructionsPanel({
  challengeId,
  gradeSteps,
}: {
  challengeId: string;
  gradeSteps?: GradeStep[];
}) {
  const [revealed, reveal] = useRevealedHints(challengeId);
  const { data, isLoading, isError, refetch } = useChallenge(challengeId, revealed);

  if (isLoading) return <InstructionsSkeleton />;

  if (isError || !data) {
    return (
      <div className="border border-danger/30 bg-danger/5 p-4 text-sm">
        <p className="text-danger">Couldn't load the challenge instructions.</p>
        <button
          className="mt-2 text-xs text-accent underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
          onClick={() => refetch()}
        >
          retry →
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-10">
      <header>
        <p className="eyebrow text-accent">{data.category}</p>
        <div className="eyebrow-xs mt-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-muted-foreground">
          <span className={cn("border px-2 py-0.5", difficultyTone(data.difficulty))}>
            {data.difficulty}
          </span>
          {data.estMinutes > 0 && <span>· {data.estMinutes} min</span>}
        </div>
        <p className="mt-5 text-sm leading-relaxed text-foreground/90">
          {data.description}
        </p>
      </header>

      <section className="border-t border-border/60 pt-8">
        <p className="eyebrow mb-3 text-muted-foreground">Requirements</p>
        <StepList steps={data.steps} results={gradeSteps} />
      </section>

      {data.hintsTotal > 0 && (
        <section className="border-t border-border/60 pt-8">
          <HintReveal
            hintsTotal={data.hintsTotal}
            hints={data.hints}
            revealed={revealed}
            onReveal={reveal}
          />
        </section>
      )}
    </div>
  );
}
