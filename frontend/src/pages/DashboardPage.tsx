import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { CreateSessionDialog } from "@/components/CreateSessionDialog";
import { SessionCard } from "@/components/SessionCard";
import { useSessions } from "@/hooks/useSessions";

export function DashboardPage() {
  const [creating, setCreating] = useState(false);
  const { data: sessions, isLoading, isError, error, refetch } = useSessions();

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Your sandboxes</h1>
        <Button onClick={() => setCreating(true)}>New sandbox</Button>
      </div>

      {isLoading && (
        <div className="grid gap-3 sm:grid-cols-2">
          {[0, 1].map((i) => (
            <Card key={i}>
              <CardContent className="h-28 animate-pulse" />
            </Card>
          ))}
        </div>
      )}

      {isError && (
        <Card>
          <CardContent className="text-sm text-red-400">
            Couldn't load your sessions
            {error instanceof Error ? `: ${error.message}` : ""}.{" "}
            <button className="text-primary hover:underline" onClick={() => refetch()}>
              Retry
            </button>
          </CardContent>
        </Card>
      )}

      {!isLoading && !isError && sessions && sessions.length === 0 && (
        <Card>
          <CardContent className="py-12 text-center">
            <p className="text-lg font-medium">No sandboxes yet</p>
            <p className="mt-1 text-sm text-muted-foreground">
              Create your first ephemeral Kubernetes cluster to get started.
            </p>
            <Button className="mt-4" onClick={() => setCreating(true)}>
              Create your first sandbox
            </Button>
          </CardContent>
        </Card>
      )}

      {sessions && sessions.length > 0 && (
        <div className="grid gap-3 sm:grid-cols-2">
          {sessions.map((s) => (
            <SessionCard key={s.id} session={s} />
          ))}
        </div>
      )}

      {creating && <CreateSessionDialog onClose={() => setCreating(false)} />}
    </div>
  );
}
