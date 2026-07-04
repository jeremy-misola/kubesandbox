import type { HTMLAttributes } from "react";

import { cn } from "@/lib/utils";

/**
 * Editorial surface. Rectangular, hairline-framed, lifted by a soft ambient
 * shadow that deepens on hover — never a harsh drop. A `featured` card gains a
 * gold top rule to signal importance.
 */
export function Card({
  className,
  featured,
  ...props
}: HTMLAttributes<HTMLDivElement> & { featured?: boolean }) {
  return (
    <div
      className={cn(
        "border border-border bg-card text-card-foreground shadow-card",
        "transition-[box-shadow,border-color] duration-500 ease-luxury hover:shadow-card-hover",
        featured
          ? "border-t-2 border-t-accent"
          : "hover:border-accent/30",
        className,
      )}
      {...props}
    />
  );
}

export function CardContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("p-6", className)} {...props} />;
}

/**
 * Terminal-window chrome: three traffic dots + a mono title strip. Kept as the
 * app's signature "session = terminal window" motif, squared to the editorial
 * grid and framed with a hairline rule.
 */
export function TerminalChrome({
  title,
  className,
  children,
  ...props
}: HTMLAttributes<HTMLDivElement> & { title?: string }) {
  return (
    <div
      className={cn(
        "flex items-center gap-2 border-b border-border/60 bg-muted/40 px-4 py-2.5",
        className,
      )}
      {...props}
    >
      <span aria-hidden className="flex gap-1.5">
        <i className="h-2 w-2 rounded-full bg-danger/70" />
        <i className="h-2 w-2 rounded-full bg-warning/70" />
        <i className="h-2 w-2 rounded-full bg-success/70" />
      </span>
      {title && (
        <span className="ml-1 truncate font-mono text-[11px] tracking-wide text-muted-foreground">
          {title}
        </span>
      )}
      {children}
    </div>
  );
}
