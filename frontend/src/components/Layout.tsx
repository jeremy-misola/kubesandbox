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
      className="h-6 w-6 text-accent"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
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

  // Editorial surfaces render light (the public landing); the app interior —
  // dashboard, terminal, dialogs — stays dark and legible for tooling.
  const editorialLight = pathname === "/";
  const isTerminal = pathname.startsWith("/terminal");

  return (
    <div
      className={cn(
        "relative flex min-h-screen flex-col bg-background text-foreground",
        editorialLight ? "theme-light" : "dark",
      )}
    >
      {/* Architectural grid + paper grain — the editorial substrate. */}
      <div aria-hidden className="gridlines">
        <span style={{ left: "12%" }} />
        <span style={{ left: "37%" }} />
        <span style={{ left: "63%" }} />
        <span style={{ left: "88%" }} />
      </div>
      <div aria-hidden className="paper-noise" />

      <header className="sticky top-0 z-40 border-b border-border/20 bg-background/80 backdrop-blur-md">
        <div className="mx-auto flex w-full max-w-[1600px] items-center justify-between px-6 py-4 md:px-12">
          <Link
            to="/"
            className="group flex items-center gap-3 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
          >
            <BrandMark />
            <span className="text-xs font-medium uppercase tracking-label transition-colors duration-500 ease-luxury group-hover:text-accent">
              Kubesandbox
            </span>
          </Link>

          {!isAuthenticated && (
            <nav className="flex items-center gap-2" aria-label="Main">
              <Button size="sm" disabled={loading} onClick={() => login()}>
                Sign in
              </Button>
            </nav>
          )}

          {isAuthenticated && (
            <nav className="flex items-center gap-4 sm:gap-6" aria-label="Main">
              <Link
                to="/dashboard"
                className={cn(
                  "text-[11px] uppercase tracking-label transition-colors duration-500 ease-luxury",
                  "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent",
                  pathname.startsWith("/dashboard")
                    ? "text-foreground"
                    : "text-muted-foreground hover:text-accent",
                )}
              >
                Dashboard
              </Link>
              <span className="hidden items-center gap-2.5 border-l border-border/30 pl-4 sm:flex">
                <i className="h-1.5 w-1.5 rounded-full bg-success" aria-hidden />
                <span className="max-w-[180px] truncate text-[11px] tracking-wide text-muted-foreground">
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

      {/* The embedded terminal wants room; editorial surfaces want air. */}
      <main
        className={cn(
          "relative z-10 mx-auto flex w-full flex-1 flex-col px-6 md:px-12",
          isTerminal
            ? "max-w-[1600px] py-6"
            : editorialLight
              ? "max-w-[1600px] py-12 md:py-20"
              : "max-w-5xl py-12",
        )}
      >
        {children}
      </main>

      <footer className="relative z-10 border-t border-border/20 py-8">
        <div className="mx-auto flex w-full max-w-[1600px] flex-col gap-2 px-6 md:flex-row md:items-center md:justify-between md:px-12">
          <p className="text-[10px] uppercase tracking-label text-muted-foreground">
            Sandboxes are ephemeral — everything is deleted when the timer runs out.
          </p>
          <p className="text-[10px] uppercase tracking-overline text-muted-foreground/70">
            Kubesandbox — Vol. 01
          </p>
        </div>
      </footer>
    </div>
  );
}
