import { Link } from "react-router-dom";

import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { ChallengeMeta } from "@/lib/schemas";

/** Difficulty tint in the StatusBadge visual idiom (success/warning/danger for
 *  easy/medium/hard) but static — it is NOT a StatusBadge, which is
 *  semantically a live session indicator (design §6.2). */
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

/**
 * Catalog card on the Card primitive with the editorial treatment: category as
 * a tiny uppercase wide-tracked overline, title in font-display, description
 * clamped to two lines, then a mono metadata row. The whole card is the link
 * target; hover follows the house shadow-card-hover + border-accent/30 already
 * built into Card. A slot is reserved for phase-2 completion badges (§5.1) — no
 * layout rework later.
 */
export function ChallengeCard({ meta }: { meta: ChallengeMeta }) {
  return (
    <Link
      to={`/challenges/${meta.id}`}
      className="group block focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
    >
      <Card className="flex h-full flex-col p-6">
        <p className="font-mono text-[10px] uppercase tracking-[0.25em] text-muted-foreground">
          {meta.category}
        </p>
        <h3 className="mt-3 font-display text-2xl font-normal leading-tight tracking-tight transition-colors duration-500 ease-luxury group-hover:text-accent">
          {meta.title}
        </h3>
        <p className="mt-2 line-clamp-2 text-sm leading-relaxed text-muted-foreground">
          {meta.description}
        </p>

        <div className="mt-auto flex flex-wrap items-center gap-x-2 gap-y-1 pt-5 font-mono text-[10px] uppercase tracking-label text-muted-foreground">
          <span className={cn("border px-2 py-0.5", difficultyTone(meta.difficulty))}>
            {meta.difficulty}
          </span>
          {meta.estMinutes > 0 && <span>· {meta.estMinutes} min</span>}
          {meta.tags.length > 0 && (
            <span className="tracking-normal text-muted-foreground/70">
              · {meta.tags.join(" · ")}
            </span>
          )}
        </div>
      </Card>
    </Link>
  );
}
