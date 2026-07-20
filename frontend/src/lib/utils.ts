/** Minimal className joiner (shadcn convention, without extra deps). */
export function cn(...classes: Array<string | false | null | undefined>): string {
  return classes.filter(Boolean).join(" ");
}

/** True when the user asked the OS for less motion — gates anime.js effects. */
export function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

/** Difficulty is metadata, not status: a quiet monochrome scale where only the
 *  hardest tier earns the gold accent, keeping gold as the sole accent. Shared
 *  by the challenge detail page and the in-session instructions panel so both
 *  render the difficulty chip identically. */
export function difficultyTone(difficulty: string): string {
  switch (difficulty.toLowerCase()) {
    case "easy":
      return "border-border bg-muted/40 text-muted-foreground";
    case "medium":
      return "border-border bg-muted/40 text-foreground/90";
    case "hard":
      return "border-accent/40 bg-accent/10 text-accent";
    default:
      return "border-border bg-muted/40 text-muted-foreground";
  }
}

/** Relative "time left" until an RFC3339 timestamp, e.g. "42m left". */
export function timeLeft(expiresAt?: string): string | null {
  if (!expiresAt) return null;
  const ms = new Date(expiresAt).getTime() - Date.now();
  if (Number.isNaN(ms)) return null;
  if (ms <= 0) return "expired";
  const mins = Math.round(ms / 60_000);
  if (mins < 60) return `${mins}m left`;
  const h = Math.floor(mins / 60);
  return `${h}h ${mins % 60}m left`;
}
