import { cn } from "@/lib/utils";

// Progressive hints. The revealed count is owned by the caller (useRevealedHints,
// sessionStorage-backed) and shared by the detail page and the in-session panel,
// so hints revealed before starting stay revealed inside the workspace. Revealing
// is one-way within a tab. Gold is one of its three sanctioned appearances
// (design §12): the reveal affordance, focus states, the passing verdict.

export function HintReveal({
  hintsTotal,
  hints,
  revealed,
  onReveal,
}: {
  hintsTotal: number;
  hints?: string[];
  revealed: number;
  onReveal: () => void;
}) {
  if (hintsTotal === 0) return null;

  return (
    <div>
      <p className="mb-2 font-mono text-[10px] uppercase tracking-label text-muted-foreground">
        Hints
        <span className="ml-2 text-muted-foreground/60">
          {Math.min(revealed, hintsTotal)}/{hintsTotal}
        </span>
      </p>
      <ol className="space-y-2">
        {Array.from({ length: hintsTotal }).map((_, i) => {
          const text = hints?.[i];
          const isRevealed = i < revealed && !!text;
          const isNext = i === revealed && revealed < hintsTotal;

          if (isRevealed) {
            return (
              <li
                key={i}
                className="border-l border-accent/40 py-0.5 pl-3 text-sm leading-relaxed text-foreground/85"
              >
                {text}
              </li>
            );
          }
          if (isNext) {
            return (
              <li key={i}>
                <button
                  type="button"
                  onClick={onReveal}
                  className={cn(
                    "group inline-flex items-center gap-2 border-l border-border py-0.5 pl-3",
                    "font-mono text-xs uppercase tracking-label text-accent",
                    "transition-colors duration-500 ease-luxury hover:border-accent",
                    "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent",
                  )}
                >
                  Reveal hint {i + 1}
                  <span aria-hidden className="text-accent/60">
                    →
                  </span>
                </button>
              </li>
            );
          }
          return (
            <li
              key={i}
              className="border-l border-border/50 py-0.5 pl-3 font-mono text-xs text-muted-foreground/50"
            >
              Hint {i + 1} — hidden
            </li>
          );
        })}
      </ol>
    </div>
  );
}
