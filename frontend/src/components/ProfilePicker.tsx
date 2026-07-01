import { cn } from "@/lib/utils";
import { PROFILE_RESOURCES, profileSchema, type Profile } from "@/lib/schemas";

const PROFILES = profileSchema.options;

const blurb: Record<Profile, string> = {
  starter: "Quick experiments and learning",
  standard: "Workshops and typical workloads",
  advanced: "Resource-intensive demos and testing",
};

export function ProfilePicker({
  value,
  onChange,
}: {
  value: Profile;
  onChange: (p: Profile) => void;
}) {
  return (
    <div className="grid gap-2 sm:grid-cols-3">
      {PROFILES.map((p) => {
        const res = PROFILE_RESOURCES[p];
        const selected = value === p;
        return (
          <button
            key={p}
            type="button"
            onClick={() => onChange(p)}
            className={cn(
              "rounded-md border p-3 text-left transition-colors",
              selected
                ? "border-primary bg-primary/10"
                : "border-border hover:border-primary/50",
            )}
          >
            <div className="font-medium capitalize">{p}</div>
            <div className="text-xs text-muted-foreground">{blurb[p]}</div>
            <div className="mt-1 text-xs text-muted-foreground">
              {res.cpu} CPU · {res.memory}
            </div>
          </button>
        );
      })}
    </div>
  );
}
