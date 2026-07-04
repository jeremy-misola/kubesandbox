import { forwardRef, type ButtonHTMLAttributes } from "react";

import { cn } from "@/lib/utils";

type Variant = "primary" | "secondary" | "ghost" | "danger";
type Size = "sm" | "md" | "lg";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
}

/* Editorial luxury buttons: rectangular, uppercase, widely tracked, cinematic.
   The primary variant reveals a gold layer that slides in from the left on
   hover — the signature CTA gesture of the system. */

const sizes: Record<Size, string> = {
  sm: "h-10 px-6 text-[11px]",
  md: "h-12 px-8 text-xs",
  lg: "h-14 px-10 text-xs",
};

const base = cn(
  "group relative inline-flex select-none items-center justify-center gap-2 overflow-hidden",
  "font-sans font-medium uppercase tracking-button whitespace-nowrap",
  "transition-all duration-500 ease-luxury",
  "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background",
  "disabled:pointer-events-none disabled:opacity-50 disabled:shadow-none",
);

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "primary", size = "md", children, ...props }, ref) => {
    if (variant === "primary") {
      return (
        <button
          ref={ref}
          className={cn(
            base,
            sizes[size],
            "border border-foreground bg-primary text-primary-foreground",
            "shadow-cta hover:shadow-cta-hover",
            className,
          )}
          {...props}
        >
          {/* Gold layer slides in from the left. */}
          <span
            aria-hidden
            className={cn(
              "absolute inset-0 -translate-x-full bg-accent",
              "transition-transform duration-500 ease-luxury",
              "group-hover:translate-x-0",
            )}
          />
          <span className="relative z-10 inline-flex items-center gap-2">
            {children}
          </span>
        </button>
      );
    }

    const variants: Record<Exclude<Variant, "primary">, string> = {
      secondary:
        "border border-foreground/70 bg-transparent text-foreground " +
        "hover:bg-foreground hover:text-background",
      ghost:
        "bg-transparent text-muted-foreground hover:text-accent",
      danger:
        "border border-danger/50 bg-transparent text-danger " +
        "hover:bg-danger hover:text-background",
    };

    return (
      <button
        ref={ref}
        className={cn(base, sizes[size], variants[variant], className)}
        {...props}
      >
        <span className="relative z-10 inline-flex items-center gap-2">
          {children}
        </span>
      </button>
    );
  },
);
Button.displayName = "Button";
