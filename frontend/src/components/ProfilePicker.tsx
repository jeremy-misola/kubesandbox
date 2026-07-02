import { cn } from "@/lib/utils";
import { PROFILE_RESOURCES, profileSchema, type Profile } from "@/lib/schemas";

const PROFILES = profileSchema.options;

const blurb: Record<Profile, string> = {
  starter: "Quick experiments and learning",
  standard: "Workshops and typical workloads",
  advanced: "Resource-intensive demos and testing",
};

/** Bars sized to the relative capacity of each profile. */
const capacity: Record<Profile, number> = {
  starter: 1,
  standard: 2,
  advanced: 3,
};

export function ProfilePicker({
  value,
  onChange,
}: {
  value: Profile;
  onChange: (p: Profile) => void;
}) {
  return (
    <div role="radiogroup" aria-label="Resource profile" className="grid gap-2 sm:grid-cols-3">
      {PROFILES.map((p) => {
        const res = PROFILE_RESOURCES[p];
        const selected = value === p;
        return (
          <button
            key={p}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onChange(p)}
            className={cn(
              "group relative rounded-md border p-3 text-left transition-all duration-150",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
              selected
                ? "border-primary/60 bg-primary/10 shadow-glow-sm"
                : "border-border bg-muted/40 hover:border-primary/30 hover:bg-muted/70",
            )}
          >
            <div className="flex items-center justify-between">
              <span
                className={cn(
                  "font-display text-sm font-semibold capitalize tracking-tight",
                  selected ? "text-primary" : "text-foreground",
                )}
              >
                {p}
              </span>
              {/* capacity bars — structure that encodes real information */}
              <span className="flex items-end gap-0.5" aria-hidden>
                {[1, 2, 3].map((i) => (
                  <i
                    key={i}
                    className={cn(
                      "w-1 rounded-sm transition-colors",
                      i === 1 ? "h-1.5" : i === 2 ? "h-2.5" : "h-3.5",
                      i <= capacity[p]
                        ? selected
                          ? "bg-primary"
                          : "bg-muted-foreground"
                        : "bg-border",
                    )}
                  />
                ))}
              </span>
            </div>
            <p className="mt-1 text-xs leading-snug text-muted-foreground">{blurb[p]}</p>
            <p className="mt-2 font-mono text-[11px] text-muted-foreground">
              <span className={selected ? "text-primary/90" : ""}>{res.cpu}</span> cpu ·{" "}
              <span className={selected ? "text-primary/90" : ""}>{res.memory}</span>
            </p>
          </button>
        );
      })}
    </div>
  );
}
