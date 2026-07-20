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

/** Live "time left" until an RFC3339 timestamp as a counting-down clock:
 *  "12:04 left" under an hour, "1:05:30 left" past it. Seconds are floored so a
 *  per-second tick shows each value once; pair with `useTimeLeft` to make it
 *  count down. */
export function timeLeft(expiresAt?: string): string | null {
  if (!expiresAt) return null;
  const ms = new Date(expiresAt).getTime() - Date.now();
  if (Number.isNaN(ms)) return null;
  if (ms <= 0) return "expired";
  const total = Math.floor(ms / 1000);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  const clock = h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
  return `${clock} left`;
}
