import { useState } from "react";

import { Button } from "@/components/ui/button";
import { ProfilePicker } from "@/components/ProfilePicker";
import { useCreateSession } from "@/hooks/useSessions";
import { ApiError } from "@/lib/api";
import { TTL, type Profile } from "@/lib/schemas";

export function CreateSessionDialog({ onClose }: { onClose: () => void }) {
  const [profile, setProfile] = useState<Profile>("standard");
  const [ttl, setTtl] = useState<number>(TTL.default);
  const create = useCreateSession();

  const submit = () => {
    create.mutate(
      { profile, ttlMinutes: ttl },
      { onSuccess: onClose },
    );
  };

  const errorMsg =
    create.error instanceof ApiError
      ? create.error.code === "quota_exceeded"
        ? "You've reached your concurrent session limit. Delete one to create another."
        : create.error.message
      : create.error
        ? "Could not create session. Please try again."
        : null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-lg rounded-lg border border-border bg-card p-5">
        <h2 className="text-lg font-semibold">New sandbox</h2>
        <p className="mb-4 text-sm text-muted-foreground">
          Pick a profile and lifetime. Your cluster provisions in the background.
        </p>

        <label className="mb-1 block text-sm font-medium">Profile</label>
        <ProfilePicker value={profile} onChange={setProfile} />

        <label className="mb-1 mt-4 block text-sm font-medium">
          Lifetime (minutes) — {ttl}
        </label>
        <input
          type="range"
          min={TTL.min}
          max={TTL.max}
          step={15}
          value={ttl}
          onChange={(e) => setTtl(Number(e.target.value))}
          className="w-full"
        />
        <div className="flex justify-between text-xs text-muted-foreground">
          <span>{TTL.min}m</span>
          <span>{TTL.max}m</span>
        </div>

        {errorMsg && (
          <p className="mt-3 rounded-md bg-red-500/10 px-3 py-2 text-sm text-red-400">
            {errorMsg}
          </p>
        )}

        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose} disabled={create.isPending}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={create.isPending}>
            {create.isPending ? "Creating…" : "Create sandbox"}
          </Button>
        </div>
      </div>
    </div>
  );
}
