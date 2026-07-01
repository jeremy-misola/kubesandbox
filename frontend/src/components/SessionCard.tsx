import { Link } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader } from "@/components/ui/card";
import { StatusBadge } from "@/components/StatusBadge";
import { useDeleteSession } from "@/hooks/useSessions";
import { terminalUrl } from "@/config";
import { timeLeft } from "@/lib/utils";
import type { Session } from "@/lib/schemas";

export function SessionCard({ session }: { session: Session }) {
  const del = useDeleteSession();
  const left = timeLeft(session.expiresAt);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <div>
          <div className="font-medium capitalize">{session.profile} sandbox</div>
          <div className="text-xs text-muted-foreground">{session.id}</div>
        </div>
        <StatusBadge session={session} />
      </CardHeader>
      <CardContent className="text-sm text-muted-foreground">
        <div>
          {session.resources.cpu} CPU · {session.resources.memory}
        </div>
        {left && <div>{left}</div>}
        {session.message && !session.workspaceReady && (
          <div className="mt-1 text-amber-400/80">{session.message}</div>
        )}
      </CardContent>
      <CardFooter className="justify-between">
        <Link
          to={`/dashboard/${session.id}`}
          className="text-sm text-primary hover:underline"
        >
          Details
        </Link>
        <div className="flex gap-2">
          <a href={terminalUrl(session.id)} target="_blank" rel="noreferrer">
            <Button size="sm" disabled={!session.workspaceReady}>
              Open terminal
            </Button>
          </a>
          <Button
            size="sm"
            variant="danger"
            onClick={() => del.mutate(session.id)}
            disabled={del.isPending}
          >
            Delete
          </Button>
        </div>
      </CardFooter>
    </Card>
  );
}
