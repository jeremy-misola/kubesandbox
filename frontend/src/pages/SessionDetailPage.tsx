import { Link, useParams } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { StatusBadge } from "@/components/StatusBadge";
import { useSession } from "@/hooks/useSessions";
import { useSessionEvents } from "@/hooks/useSessionEvents";
import { terminalUrl } from "@/config";
import { timeLeft } from "@/lib/utils";

export function SessionDetailPage() {
  const { id = "" } = useParams();
  const { data: session, isLoading, isError } = useSession(id);
  // Live updates stream into the cache; falls back to polling on failure.
  useSessionEvents(id, !!session);

  if (isLoading) {
    return <div className="text-muted-foreground">Loading session…</div>;
  }
  if (isError || !session) {
    return (
      <div>
        <p className="text-red-400">Session not found.</p>
        <Link to="/dashboard" className="text-primary hover:underline">
          Back to dashboard
        </Link>
      </div>
    );
  }

  const rows: Array<[string, string]> = [
    ["ID", session.id],
    ["Profile", session.profile],
    ["Resources", `${session.resources.cpu} CPU · ${session.resources.memory}`],
    ["TTL", `${session.ttlMinutes} min`],
    ["Phase", session.phase],
    ["Time left", timeLeft(session.expiresAt) ?? "—"],
    ["Namespace", session.sessionNamespace ?? "—"],
    ["Workspace image", session.workspaceImage],
  ];

  return (
    <div>
      <Link to="/dashboard" className="text-sm text-primary hover:underline">
        ← Back
      </Link>
      <Card className="mt-3">
        <CardHeader className="flex flex-row items-center justify-between">
          <h1 className="text-xl font-semibold capitalize">
            {session.profile} sandbox
          </h1>
          <StatusBadge session={session} />
        </CardHeader>
        <CardContent>
          <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2">
            {rows.map(([k, v]) => (
              <div key={k} className="flex justify-between border-b border-border/50 py-1">
                <dt className="text-muted-foreground">{k}</dt>
                <dd className="ml-2 truncate font-mono">{v}</dd>
              </div>
            ))}
          </dl>

          {!session.workspaceReady && (
            <p className="mt-4 text-sm text-amber-400">
              Provisioning… vcluster cold-boot can take a few minutes. This page
              updates live.
            </p>
          )}

          <div className="mt-5">
            <a href={terminalUrl(session.id)} target="_blank" rel="noreferrer">
              <Button disabled={!session.workspaceReady}>Open terminal</Button>
            </a>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
