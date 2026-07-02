import { cn } from "@/lib/utils";
import type { Session } from "@/lib/schemas";

// Maps a session's phase + workspaceReady to a label + color. The phase
// vocabulary is exactly what the pipeline emits (shell pod phase mapped in
// kubesandbox-session-composition.yaml): Pending (backend default before the
// pod reports), Provisioning, Ready, Error, Unknown — confirmed live, docs/07
// §3.4. The authoritative "usable" signal is `workspaceReady`; deletion never
// surfaces as a phase (the claim is removed and the UI reacts via optimistic
// removal + the SSE `deleted` event).
function tone(session: Session): { label: string; className: string } {
  if (session.workspaceReady) {
    return { label: "Ready", className: "bg-green-500/15 text-green-400" };
  }
  switch (session.phase) {
    case "Error":
      return { label: "Error", className: "bg-red-500/15 text-red-400" };
    case "Unknown":
      return { label: "Unknown", className: "bg-zinc-500/15 text-zinc-400" };
    default:
      // Pending / Provisioning. An unrecognized future phase also lands here,
      // shown verbatim with in-progress styling rather than being hidden.
      return {
        label: session.phase || "Pending",
        className: "bg-amber-500/15 text-amber-400 animate-pulse",
      };
  }
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
