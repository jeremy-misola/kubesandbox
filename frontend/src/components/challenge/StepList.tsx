import { useEffect, useRef } from "react";
import { animate, stagger } from "animejs";

import { cn, prefersReducedMotion } from "@/lib/utils";
import type { ChallengeStep, GradeStep } from "@/lib/schemas";

// The step checklist, keyed by step id. Detail step ids and GradeStep ids are
// the SAME ids (both come from the bundle's validate[]), so the last grade
// result merges straight into the checklist: untested → neutral dot, pass →
// success, fail → danger with the failure message inline. One component, both
// pages — the detail page renders it without results, the workspace with them.

type StepState = "neutral" | "pass" | "fail";

function StepDot({ state }: { state: StepState }) {
  return (
    <i
      aria-hidden
      className={cn(
        "mt-1.5 h-2 w-2 shrink-0 rounded-full",
        state === "pass" && "bg-success text-success",
        state === "fail" && "bg-danger text-danger",
        state === "neutral" && "bg-muted-foreground/40",
      )}
    />
  );
}

export function StepList({
  steps,
  results,
}: {
  steps: ChallengeStep[];
  results?: GradeStep[];
}) {
  const byId = new Map((results ?? []).map((r) => [r.id, r]));
  const listRef = useRef<HTMLOListElement>(null);

  // When a fresh grade result lands, animate the rows in with the same
  // restrained, staggered pop the boot lines use (design §12).
  const resultsKey = results
    ? results.map((r) => `${r.id}:${r.pass}`).join("|")
    : "";
  useEffect(() => {
    if (!resultsKey || !listRef.current || prefersReducedMotion()) return;
    animate(listRef.current.querySelectorAll("[data-step-row]"), {
      opacity: [0.55, 1],
      translateY: [4, 0],
      duration: 450,
      ease: "out(3)",
      delay: stagger(60),
    });
  }, [resultsKey]);

  return (
    <ol
      ref={listRef}
      className="divide-y divide-border/60 border-y border-border/60"
    >
      {steps.map((s) => {
        const r = byId.get(s.id);
        const state: StepState = !r ? "neutral" : r.pass ? "pass" : "fail";
        return (
          <li key={s.id} data-step-row className="flex gap-3 py-3.5">
            <StepDot state={state} />
            <div className="min-w-0">
              <p
                className={cn(
                  "text-sm leading-relaxed",
                  state === "pass"
                    ? "text-foreground/90"
                    : state === "fail"
                      ? "text-foreground"
                      : "text-foreground/85",
                )}
              >
                {s.description}
              </p>
              {state === "fail" && r?.message && (
                <p className="mt-1 font-mono text-xs leading-relaxed text-danger/90">
                  {r.message}
                </p>
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
