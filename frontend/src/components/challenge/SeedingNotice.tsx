import { Card, TerminalChrome } from "@/components/ui/card";
import type { Session } from "@/lib/schemas";

/**
 * The seeding gate's down-state — the exact parallel of the provisioning card
 * (design §6.6/§8): same skeleton-mirrors-loaded-layout discipline, a
 * TerminalChrome-topped card with the dot-breathe warning dot and the backend's
 * own synthetic message (session.message), falling back to calm static copy.
 *
 * The terminal iframe is deliberately NOT mounted while gated: the pty would
 * work (authz doesn't gate on seeding), but showing a live shell into a
 * half-seeded cluster is exactly what the gate exists to prevent. It's a ~2s
 * state — the copy stays calm, no progress bars, no percentage theater; the
 * dot-breathe is the activity signal, as everywhere else.
 */
export function SeedingNotice({ session }: { session: Session }) {
  const message = session.message?.trim() || "Preparing your challenge…";

  return (
    <Card className="flex min-h-[420px] flex-1 flex-col overflow-hidden p-0">
      <TerminalChrome title={`s/${session.id}`} />
      <div className="flex flex-1 items-start gap-3 p-5">
        <i
          aria-hidden
          className="dot-breathe mt-1 h-2 w-2 shrink-0 rounded-full bg-warning text-warning"
        />
        <div>
          <p className="text-sm text-warning/90">{message}</p>
          <p className="mt-1 text-sm text-warning/70">
            Your challenge environment is being set up. The terminal and grading
            unlock here the moment it's ready — this page updates live.
          </p>
        </div>
      </div>
    </Card>
  );
}
