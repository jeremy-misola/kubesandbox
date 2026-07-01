import { cn } from "@/lib/utils";
import type { Session } from "@/lib/schemas";

// Maps a session's phase + workspaceReady to a label + color. `phase` is a
// display label from the Crossplane pipeline; the authoritative "usable" signal
// is `workspaceReady` (docs/06 §8, action item 6).
function tone(session: Session): { label: string; className: string } {
  if (session.workspaceReady) {
    return { label: "Ready", className: "bg-green-500/15 text-green-400" };
  }
  const phase = session.phase.toLowerCase();
  if (phase.includes("fail") || phase.includes("error")) {
    return { label: session.phase, className: "bg-red-500/15 text-red-400" };
  }
  if (phase.includes("terminat") || phase.includes("delet")) {
    return { label: session.phase, className: "bg-zinc-500/15 text-zinc-400" };
  }
  // Pending / Provisioning / Creating …
  return {
    label: session.phase || "Pending",
    className: "bg-amber-500/15 text-amber-400 animate-pulse",
  };
}

export function StatusBadge({ session }: { session: Session }) {
  const { label, className } = tone(session);
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium",
        className,
      )}
    >
      {label}
    </span>
  );
}
