import type { HTMLAttributes } from "react";

import { cn } from "@/lib/utils";

/**
 * Base surface. Cards read as instrument panels: hairline border, soft inner
 * highlight, slight lift + emerald edge on hover.
 */
export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "rounded-lg border border-border bg-card text-card-foreground shadow-card",
        "transition-colors duration-200 hover:border-primary/25",
        className,
      )}
      {...props}
    />
  );
}

export function CardContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("p-4 pt-2", className)} {...props} />;
}

/**
 * Terminal-window chrome: the three traffic dots + a mono title strip.
 * Used by SessionCard and the landing hero so "session = terminal window"
 * reads consistently across the app.
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
        "flex items-center gap-2 rounded-t-lg border-b border-border bg-muted/50 px-4 py-2.5",
        className,
      )}
      {...props}
    >
      <span aria-hidden className="flex gap-1.5">
        <i className="h-2.5 w-2.5 rounded-full bg-danger/70" />
        <i className="h-2.5 w-2.5 rounded-full bg-warning/70" />
        <i className="h-2.5 w-2.5 rounded-full bg-success/70" />
      </span>
      {title && (
        <span className="ml-1 truncate font-mono text-xs text-muted-foreground">
          {title}
        </span>
      )}
      {children}
    </div>
  );
}
