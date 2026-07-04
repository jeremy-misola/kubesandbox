import type { ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { useAuth } from "@/hooks/useAuth";
import { cn } from "@/lib/utils";

/** Hexagonal node mark — a nod to the k8s heptagon, drawn as a live cluster. */
function BrandMark() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-6 w-6 text-primary"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      aria-hidden
    >
      <path d="M12 2.5 20.2 7v10L12 21.5 3.8 17V7L12 2.5Z" />
      <path d="M12 8v4l3 2.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function Layout({ children }: { children: ReactNode }) {
  const { isAuthenticated, loading, user, login, logout } = useAuth();
  const { pathname } = useLocation();

  return (
    <div className="relative flex min-h-screen flex-col text-foreground">
      <header className="sticky top-0 z-40 border-b border-border/70 bg-background/80 backdrop-blur-md">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-4 py-3">
          <Link
            to="/"
            className="group flex items-center gap-2.5 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
          >
            <BrandMark />
            <span className="text-[15px] font-semibold tracking-tight">
              kube<span className="text-primary">sandbox</span>
            </span>
          </Link>

          {!isAuthenticated && (
            <nav className="flex items-center gap-1 sm:gap-2" aria-label="Main">
              <Button
                size="sm"
                className="ml-1"
                disabled={loading}
                onClick={() => login()}
              >
                Sign in
              </Button>
            </nav>
          )}

          {isAuthenticated && (
            <nav className="flex items-center gap-1 sm:gap-3" aria-label="Main">
              <Link
                to="/dashboard"
                className={cn(
                  "rounded-md px-2 py-1 text-sm transition-colors",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
                  pathname.startsWith("/dashboard")
                    ? "text-foreground"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                Dashboard
              </Link>
              <span className="hidden items-center gap-2 border-l border-border pl-3 sm:flex">
                <i className="h-1.5 w-1.5 rounded-full bg-success" aria-hidden />
                <span className="max-w-[180px] truncate text-sm text-muted-foreground">
                  {user?.profile?.email ?? user?.profile?.name}
                </span>
              </span>
              <Button variant="ghost" size="sm" onClick={() => logout()}>
                Sign out
              </Button>
            </nav>
          )}
        </div>
      </header>

      {/* The embedded terminal wants room: wider column, tighter padding,
          and a flex column so the frame can fill the viewport height. */}
      <main
        className={cn(
          "mx-auto flex w-full flex-1 flex-col px-4",
          pathname.startsWith("/terminal")
            ? "max-w-7xl py-5"
            : "max-w-5xl py-8",
        )}
      >
        {children}
      </main>

      <footer className="border-t border-border/60 py-4">
        <p className="mx-auto max-w-5xl px-4 text-xs text-muted-foreground/70">
          Sandboxes are ephemeral. Everything is deleted when the timer runs out.
        </p>
      </footer>
    </div>
  );
}
