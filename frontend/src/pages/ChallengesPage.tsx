import { useMemo } from "react";
import { useSearchParams } from "react-router-dom";

import { Card } from "@/components/ui/card";
import { ChallengeCard } from "@/components/ChallengeCard";
import { useChallenges } from "@/hooks/useChallenges";
import { useReveal } from "@/hooks/useReveal";
import { cn } from "@/lib/utils";
import type { ChallengeMeta } from "@/lib/schemas";

const DIFFICULTY_RANK: Record<string, number> = { easy: 0, medium: 1, hard: 2 };

function uniqueSorted(values: string[]): string[] {
  return [...new Set(values)].sort((a, b) => a.localeCompare(b));
}

/** Underline-style control per the design system (§12) — text options, not a
 *  boxed dropdown; the active one carries a gold underline. */
function FilterRow({
  label,
  options,
  active,
  onSelect,
}: {
  label: string;
  options: string[];
  active: string;
  onSelect: (value: string) => void;
}) {
  const render = (value: string, text: string) => {
    const isActive = active === value;
    return (
      <button
        key={value || "all"}
        type="button"
        onClick={() => onSelect(value)}
        className={cn(
          "eyebrow border-b py-1 font-medium transition-colors duration-500 ease-luxury",
          "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent",
          isActive
            ? "border-accent text-foreground"
            : "border-transparent text-foreground/75 hover:text-accent",
        )}
      >
        {text}
      </button>
    );
  };

  return (
    <div className="flex flex-wrap items-baseline gap-x-5 gap-y-2">
      {/* Field name: serif italic + a hairline divider set it apart from the
          mono, uppercase values so a filter reads distinctly from its options. */}
      <span className="mr-1 shrink-0 border-r border-border/50 pr-4 font-display text-sm italic text-muted-foreground">
        {label}
      </span>
      {render("", "All")}
      {options.map((o) => render(o, o))}
    </div>
  );
}

function CatalogSkeleton() {
  return (
    <div className="grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <Card key={i} className="flex h-full flex-col p-6">
          <div className="skeleton h-3 w-20" />
          <div className="skeleton mt-3 h-6 w-3/4" />
          <div className="skeleton mt-3 h-4 w-full" />
          <div className="skeleton mt-1.5 h-4 w-5/6" />
          <div className="mt-6 flex gap-2 pt-2">
            <div className="skeleton h-4 w-16" />
            <div className="skeleton h-4 w-12" />
          </div>
        </Card>
      ))}
    </div>
  );
}

export function ChallengesPage() {
  const { data, isLoading, isError, refetch } = useChallenges();
  const [params, setParams] = useSearchParams();

  const category = params.get("category") ?? "";
  const difficulty = params.get("difficulty") ?? "";
  const sort = params.get("sort") ?? "difficulty";

  const setParam = (key: string, value: string) => {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (value) next.set(key, value);
        else next.delete(key);
        return next;
      },
      { replace: true },
    );
  };

  const categories = useMemo(
    () => uniqueSorted((data ?? []).map((c) => c.category)),
    [data],
  );
  const difficulties = useMemo(
    () => uniqueSorted((data ?? []).map((c) => c.difficulty)),
    [data],
  );

  const shown = useMemo<ChallengeMeta[]>(() => {
    let list = data ?? [];
    if (category) list = list.filter((c) => c.category === category);
    if (difficulty) list = list.filter((c) => c.difficulty === difficulty);
    return [...list].sort((a, b) => {
      if (sort === "estMinutes") {
        return a.estMinutes - b.estMinutes || a.title.localeCompare(b.title);
      }
      if (sort === "title") return a.title.localeCompare(b.title);
      // difficulty (default)
      const ra = DIFFICULTY_RANK[a.difficulty.toLowerCase()] ?? 99;
      const rb = DIFFICULTY_RANK[b.difficulty.toLowerCase()] ?? 99;
      return ra - rb || a.title.localeCompare(b.title);
    });
  }, [data, category, difficulty, sort]);

  const catalogEmpty = !isLoading && !isError && (data?.length ?? 0) === 0;
  const noMatch = !catalogEmpty && !isLoading && !isError && shown.length === 0;

  // Cards lift in, staggered, whenever the visible set changes (load + filter).
  const gridRef = useReveal<HTMLDivElement>([shown.length]);

  return (
    <div>
      <div className="reveal-up mb-10">
        <p className="eyebrow text-accent">~/challenges</p>
        <h1 className="mt-2 font-display text-4xl font-normal tracking-tight md:text-5xl">
          Guided <span className="italic text-accent">Challenges</span>
        </h1>
        <p className="mt-3 max-w-xl text-sm leading-relaxed text-muted-foreground">
          Hands-on Kubernetes scenarios that seed a broken or half-built cluster
          into your sandbox. Fix it in a real shell, then grade your work.
        </p>
      </div>

      {!isLoading && !isError && !catalogEmpty && (
        <div
          className="reveal-up mb-8 flex flex-col gap-4 border-y border-border/60 py-5"
          style={{ animationDelay: "90ms" }}
        >
          {categories.length > 1 && (
            <FilterRow
              label="Category"
              options={categories}
              active={category}
              onSelect={(v) => setParam("category", v === category ? "" : v)}
            />
          )}
          {difficulties.length > 1 && (
            <FilterRow
              label="Difficulty"
              options={difficulties}
              active={difficulty}
              onSelect={(v) => setParam("difficulty", v === difficulty ? "" : v)}
            />
          )}
        </div>
      )}

      {isLoading && <CatalogSkeleton />}

      {isError && (
        <Card className="border-danger/30">
          <div className="p-6 text-sm">
            <p className="text-danger">Couldn't load the challenge catalog.</p>
            <button
              className="mt-2 text-xs text-accent underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
              onClick={() => refetch()}
            >
              retry →
            </button>
          </div>
        </Card>
      )}

      {catalogEmpty && (
        <Card className="overflow-hidden p-0">
          <div className="px-6 py-16 text-center">
            <p className="text-sm text-muted-foreground">
              <span className="text-accent">❯</span> no challenges published
              <br />
              <span className="text-muted-foreground/70">
                the catalog is empty right now — check back soon
              </span>
            </p>
          </div>
        </Card>
      )}

      {noMatch && (
        <Card className="overflow-hidden p-0">
          <div className="px-6 py-16 text-center">
            <p className="text-sm text-muted-foreground">
              No challenges match these filters.
            </p>
            <button
              className="mt-3 text-xs text-accent underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
              onClick={() => setParams({}, { replace: true })}
            >
              clear filters →
            </button>
          </div>
        </Card>
      )}

      {!isLoading && !isError && shown.length > 0 && (
        <div
          ref={gridRef}
          className="grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-3"
        >
          {shown.map((meta) => (
            <ChallengeCard key={meta.id} meta={meta} />
          ))}
        </div>
      )}
    </div>
  );
}
