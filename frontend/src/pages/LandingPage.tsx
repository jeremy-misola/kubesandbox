import { useLayoutEffect, useRef, type ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";
import { animate, stagger } from "animejs";

import { Button } from "@/components/ui/button";
import { TerminalChrome } from "@/components/ui/card";
import { useAuth } from "@/hooks/useAuth";
import { prefersReducedMotion } from "@/lib/utils";

export function LandingPage() {
  const { isAuthenticated, loading, user, login } = useAuth();
  const location = useLocation();
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;

  const heroRef = useRef<HTMLDivElement>(null);

  // Page-load sequence: the hero block rises in once.
  useLayoutEffect(() => {
    if (prefersReducedMotion() || !heroRef.current) return;
    const blocks = heroRef.current.querySelectorAll("[data-hero-block]");
    animate(blocks, {
      opacity: [0, 1],
      translateY: [20, 0],
      duration: 550,
      ease: "out(4)",
      delay: stagger(80),
    });
  }, []);

  return (
    <div className="pb-4">
      {/* ————— Hero ————— */}
      <section ref={heroRef} className="grid gap-10 py-10 sm:py-16 lg:grid-cols-2 lg:items-center">
        <div className="max-w-xl">
          <p
            data-hero-block
            className="text-sm font-medium text-primary"
          >
            Ephemeral Kubernetes sandboxes
          </p>
          <h1
            data-hero-block
            className="mt-3 text-4xl font-bold leading-[1.08] tracking-tight sm:text-5xl"
          >
            A throwaway cluster,
            <br />
            ready in seconds.
          </h1>
          <p data-hero-block className="mt-5 text-base leading-relaxed text-muted-foreground">
            An isolated vcluster with a browser terminal — no install, no local
            setup. Spin one up for a workshop, a demo, or an experiment. It
            deletes itself when the timer runs out, so nothing lingers and
            nothing costs you while it sits idle.
          </p>
          <div data-hero-block className="mt-8 flex flex-wrap items-center gap-4">
            {isAuthenticated ? (
              <>
                <Link to="/dashboard">
                  <Button size="md">Open dashboard</Button>
                </Link>
                <span className="max-w-[220px] truncate text-sm text-muted-foreground">
                  Signed in as {user?.profile?.email ?? user?.profile?.name}
                </span>
              </>
            ) : (
              <>
                <Button size="md" disabled={loading} onClick={() => login(returnTo)}>
                  Sign in to get started
                </Button>
                <span className="text-sm text-muted-foreground">
                  Single sign-on via Authentik
                </span>
              </>
            )}
          </div>
        </div>

        {/* Honest product visual: the actual browser terminal the product
            hands you, not a decorative graphic. */}
        <div data-hero-block className="lg:justify-self-end lg:w-full">
          <TerminalPreview />
        </div>
      </section>

      {/* ————— How it works (a real, ordered sequence) ————— */}
      <section className="border-t border-border/60 py-14">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
          How it works
        </h2>
        <div className="mt-8 grid gap-8 sm:grid-cols-3">
          {STEPS.map((step, i) => (
            <div key={step.title}>
              <div className="font-mono text-sm text-primary">
                0{i + 1}
              </div>
              <h3 className="mt-2 text-base font-semibold tracking-tight">
                {step.title}
              </h3>
              <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
                {step.body}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* ————— Features ————— */}
      <section className="border-t border-border/60 py-14">
        <div className="grid gap-x-10 gap-y-9 sm:grid-cols-2">
          {FEATURES.map((f) => (
            <div key={f.title} className="flex gap-4">
              <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-border bg-muted/50 text-primary">
                {f.icon}
              </span>
              <div>
                <h3 className="text-base font-semibold tracking-tight">
                  {f.title}
                </h3>
                <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
                  {f.body}
                </p>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* ————— Closing CTA ————— */}
      {!isAuthenticated && (
        <section className="border-t border-border/60 py-14 text-center">
          <h2 className="text-2xl font-bold tracking-tight">
            Start with a clean cluster.
          </h2>
          <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">
            Sign in and your sandbox is usually ready the moment you ask for it.
          </p>
          <Button
            size="md"
            className="mt-6"
            disabled={loading}
            onClick={() => login(returnTo)}
          >
            Sign in to get started
          </Button>
        </section>
      )}
    </div>
  );
}

/** A static, faithful snapshot of the browser terminal — the product itself,
 *  used as the hero visual instead of an abstract graphic. */
function TerminalPreview() {
  return (
    <div className="overflow-hidden rounded-lg border border-border bg-card shadow-card">
      <TerminalChrome title="s/ws-4f2a — zsh">
        <span className="ml-auto flex items-center gap-1.5 text-xs text-success">
          <i aria-hidden className="h-1.5 w-1.5 rounded-full bg-success" />
          connected
        </span>
      </TerminalChrome>
      <div className="space-y-1.5 p-4 font-mono text-[13px] leading-relaxed">
        <p>
          <span className="text-primary">❯</span> kubectl get nodes
        </p>
        <p className="text-muted-foreground">
          NAME             STATUS   ROLES    AGE   VERSION
        </p>
        <p className="text-muted-foreground">
          vcluster-0       Ready    &lt;none&gt;   6s    v1.30.2
        </p>
        <p className="pt-2">
          <span className="text-primary">❯</span> kubectl create deploy web
          --image=nginx
        </p>
        <p className="text-muted-foreground">deployment.apps/web created</p>
        <p className="pt-2">
          <span className="text-primary">❯</span>
          <span className="cursor-blink -mb-0.5 ml-1 inline-block h-4 w-2 bg-primary align-middle" />
        </p>
      </div>
    </div>
  );
}

const STEPS: { title: string; body: string }[] = [
  {
    title: "Create a sandbox",
    body: "Pick a lifetime and confirm. A pre-warmed vcluster is handed over — no cold boot, no waiting on the cluster to build.",
  },
  {
    title: "Work in the browser",
    body: "A full terminal opens right in the page. Run kubectl, deploy things, break things — it's entirely your own isolated cluster.",
  },
  {
    title: "It cleans itself up",
    body: "When the timer hits zero the whole sandbox is deleted. Nothing to tear down, nothing left running, nothing to pay for idle.",
  },
];

const FEATURES: { title: string; body: string; icon: ReactNode }[] = [
  {
    title: "Fully isolated",
    body: "Each sandbox is its own private vcluster with a fixed CPU and memory budget — separate from every other user's.",
    icon: <IconShield />,
  },
  {
    title: "Nothing to install",
    body: "The terminal runs in your browser over a live connection. No kubeconfig to wire up, no local tooling to keep current.",
    icon: <IconTerminal />,
  },
  {
    title: "Lifetime you set",
    body: "Choose how long it lives when you create it. The clock starts immediately and the sandbox auto-deletes at zero.",
    icon: <IconClock />,
  },
  {
    title: "Ready on request",
    body: "Sandboxes are kept warm in a pool and handed over on demand, so create usually resolves in seconds rather than minutes.",
    icon: <IconBolt />,
  },
];

/* Small, literal line icons — one weight, no fill. */
function iconProps() {
  return {
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.6,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    className: "h-[18px] w-[18px]",
    "aria-hidden": true,
  };
}
function IconShield() {
  return (
    <svg {...iconProps()}>
      <path d="M12 3 5 6v5c0 4 3 7 7 8 4-1 7-4 7-8V6l-7-3Z" />
    </svg>
  );
}
function IconTerminal() {
  return (
    <svg {...iconProps()}>
      <rect x="3" y="4" width="18" height="16" rx="2" />
      <path d="m7 9 3 3-3 3M13 15h4" />
    </svg>
  );
}
function IconClock() {
  return (
    <svg {...iconProps()}>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7.5V12l3 2" />
    </svg>
  );
}
function IconBolt() {
  return (
    <svg {...iconProps()}>
      <path d="M13 3 5 13h6l-1 8 8-10h-6l1-8Z" />
    </svg>
  );
}
