import { HintReveal } from "@/components/challenge/HintReveal";
import { StepList } from "@/components/challenge/StepList";
import { useChallenge, useRevealedHints } from "@/hooks/useChallenges";
import type { GradeStep } from "@/lib/schemas";

/** Skeleton mirroring the loaded panel geometry (design-principles §2). */
function InstructionsSkeleton() {
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <div className="skeleton h-3 w-24" />
        <div className="skeleton h-4 w-full" />
        <div className="skeleton h-4 w-5/6" />
      </div>
      <div className="space-y-3">
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
          className="mt-2 font-mono text-xs text-accent underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
          onClick={() => refetch()}
        >
          retry →
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div>
        <p className="overline text-accent">
          {data.category}
          <span className="ml-2 text-muted-foreground">
            {data.difficulty}
            {data.estMinutes > 0 ? ` · ${data.estMinutes} min` : ""}
          </span>
        </p>
        <p className="mt-3 text-sm leading-relaxed text-foreground/90">
          {data.description}
        </p>
      </div>

      <div>
        <p className="overline mb-2 text-muted-foreground">Steps</p>
        <StepList steps={data.steps} results={gradeSteps} />
      </div>

      <HintReveal
        hintsTotal={data.hintsTotal}
        hints={data.hints}
        revealed={revealed}
        onReveal={reveal}
      />
    </div>
  );
}
