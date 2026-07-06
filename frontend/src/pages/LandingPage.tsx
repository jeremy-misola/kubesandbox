import { useLayoutEffect, useRef, type ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";
import { animate, stagger } from "animejs";

import { Button } from "@/components/ui/button";
import { TerminalChrome } from "@/components/ui/card";
import { useAuth } from "@/hooks/useAuth";
import { prefersReducedMotion } from "@/lib/utils";

export function LandingPage() {
  const { isAuthenticated, loading, user, login, signup } = useAuth();
  const location = useLocation();
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;

  const heroRef = useRef<HTMLDivElement>(null);

  // Page-load sequence: the hero blocks rise in once, slowly and deliberately.
  useLayoutEffect(() => {
    if (prefersReducedMotion() || !heroRef.current) return;
    const blocks = heroRef.current.querySelectorAll("[data-hero-block]");
    animate(blocks, {
      opacity: [0, 1],
      translateY: [28, 0],
      duration: 1100,
      ease: "out(4)",
      delay: stagger(140),
    });
  }, []);

  return (
    <div className="pb-8">
      {/* ————— Hero: asymmetric, bottom-left weighted ————— */}
      <section
        ref={heroRef}
        className="relative grid gap-12 pt-10 sm:pt-16 lg:grid-cols-12 lg:gap-8"
      >
        {/* Vertical editorial label, desktop only. */}
        <span
          aria-hidden
          className="vertical-label absolute -left-2 top-24 hidden text-[10px] uppercase tracking-overline text-muted-foreground lg:block"
        >
          Editorial — Vol. 01 / Ephemeral Compute
        </span>

        <div className="lg:col-span-7 lg:col-start-2">
          <div data-hero-block className="flex items-center gap-4">
            <span className="h-px w-10 bg-foreground md:w-14" />
            <span className="text-[10px] uppercase tracking-overline text-muted-foreground">
              Ephemeral Kubernetes Sandboxes
            </span>
          </div>

          <h1
            data-hero-block
            className="mt-8 font-display text-5xl font-normal leading-[0.92] tracking-tight sm:text-7xl md:text-8xl"
          >
            A throwaway
            <br />
            cluster, <span className="italic text-accent">ready</span>
            <br />
            in seconds.
          </h1>

          <p
            data-hero-block
            className="drop-cap mt-10 max-w-xl text-lg font-light leading-relaxed text-muted-foreground"
          >
            An isolated Kubernetes cluster and a browser terminal, provisioned
            the moment you ask. It runs entirely on its own, then quietly cleans
            itself up when the timer reaches zero — nothing to tear down,
            nothing left running.
          </p>

          <div
            data-hero-block
            className="mt-12 flex flex-col items-start gap-4 sm:flex-row sm:items-center"
          >
            {isAuthenticated ? (
              <>
                <Link to="/dashboard">
                  <Button size="lg">Open dashboard</Button>
                </Link>
                <span className="max-w-[260px] truncate text-xs uppercase tracking-label text-muted-foreground">
                  {user?.profile?.email ?? user?.profile?.name}
                </span>
              </>
            ) : (
              <>
                <Button size="lg" disabled={loading} onClick={() => login(returnTo)}>
                  Sign in to begin
                </Button>
                <span className="flex flex-col gap-1 text-xs uppercase tracking-label text-muted-foreground sm:flex-row sm:items-center sm:gap-3">
                  Single sign-on via Authentik
                  <button
                    type="button"
                    disabled={loading}
                    onClick={() => signup()}
                    className="underline underline-offset-4 transition-colors hover:text-foreground disabled:pointer-events-none disabled:opacity-50 sm:before:mr-3 sm:before:content-['·']"
                  >
                    New here? Sign up
                  </button>
                </span>
              </>
            )}
          </div>
        </div>

        {/* The product itself — a dark terminal window resting on warm paper. */}
        <div data-hero-block className="lg:col-span-4 lg:col-start-9 lg:pt-16">
          <TerminalPreview />
        </div>
      </section>

      {/* ————— How it works ————— */}
      <section className="mt-28 border-t border-foreground/15 pt-14 md:mt-36">
        <div className="flex items-center gap-4">
          <span className="text-[10px] uppercase tracking-overline text-muted-foreground">
            The Process
          </span>
          <span className="h-px flex-1 bg-foreground/15" />
        </div>

        <div className="mt-12 grid gap-x-8 gap-y-12 sm:grid-cols-3">
          {STEPS.map((step, i) => (
            <div key={step.title} className="border-t border-foreground pt-6">
              <div className="font-display text-5xl font-normal leading-none text-accent">
                0{i + 1}
              </div>
              <h3 className="mt-5 font-display text-2xl font-normal tracking-tight">
                {step.title}
              </h3>
              <p className="mt-3 text-sm font-light leading-relaxed text-muted-foreground">
                {step.body}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* ————— Features ————— */}
      <section className="mt-28 border-t border-foreground/15 pt-14 md:mt-36">
        <div className="flex items-center gap-4">
          <span className="text-[10px] uppercase tracking-overline text-muted-foreground">
            The Details
          </span>
          <span className="h-px flex-1 bg-foreground/15" />
        </div>

        <div className="mt-12 grid gap-x-12 gap-y-12 sm:grid-cols-2">
          {FEATURES.map((f) => (
            <div key={f.title} className="group flex gap-6 border-t border-foreground/70 pt-6">
              <span className="mt-0.5 text-muted-foreground transition-colors duration-500 ease-luxury group-hover:text-accent">
                {f.icon}
              </span>
              <div>
                <h3 className="font-display text-xl font-normal tracking-tight">
                  {f.title}
                </h3>
                <p className="mt-2 text-sm font-light leading-relaxed text-muted-foreground">
                  {f.body}
                </p>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* ————— Closing CTA ————— */}
      {!isAuthenticated && (
        <section className="mt-28 border-t border-foreground/15 pt-16 md:mt-36">
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="font-display text-4xl font-normal leading-tight tracking-tight sm:text-5xl">
              Start with a <span className="italic text-accent">clean</span> cluster.
            </h2>
            <p className="mx-auto mt-5 max-w-md text-sm font-light leading-relaxed text-muted-foreground">
              Sign in and your sandbox is usually handed over the moment you ask
              for it — pre-warmed, isolated, and yours alone.
            </p>
            <div className="mt-10 flex flex-col items-center gap-3">
              <Button size="lg" disabled={loading} onClick={() => login(returnTo)}>
                Sign in to begin
              </Button>
              <button
                type="button"
                disabled={loading}
                onClick={() => signup()}
                className="text-xs uppercase tracking-label text-muted-foreground underline underline-offset-4 transition-colors hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
              >
                New here? Sign up
              </button>
            </div>
          </div>
        </section>
      )}
    </div>
  );
}

/** A static, faithful snapshot of the browser terminal — the product itself,
 *  used as the hero visual. Kept dark: a terminal is a terminal. The gold
 *  prompt caret is the single editorial accent. */
function TerminalPreview() {
  return (
    <div className="relative">
      <span
        aria-hidden
        className="vertical-label absolute -right-7 top-2 hidden text-[10px] uppercase tracking-overline text-muted-foreground xl:block"
      >
        Live Session
      </span>
      <div className="dark overflow-hidden border border-foreground/20 bg-[#1A1A1A] text-[#F9F8F6] shadow-hero">
        <TerminalChrome title="s/ws-4f2a — zsh">
          <span className="ml-auto flex items-center gap-1.5 text-[10px] uppercase tracking-label text-success">
            <i aria-hidden className="h-1.5 w-1.5 rounded-full bg-success" />
            connected
          </span>
        </TerminalChrome>
        <div className="space-y-1.5 p-5 font-mono text-[13px] leading-relaxed">
          <p>
            <span className="text-accent">❯</span> kubectl get nodes
          </p>
          <p className="text-muted-foreground">
            NAME             STATUS   ROLES    AGE   VERSION
          </p>
          <p className="text-muted-foreground">
            vcluster-0       Ready    &lt;none&gt;   6s    v1.30.2
          </p>
          <p className="pt-2">
            <span className="text-accent">❯</span> kubectl create deploy web
            --image=nginx
          </p>
          <p className="text-muted-foreground">deployment.apps/web created</p>
          <p className="pt-2">
            <span className="text-accent">❯</span>
            <span className="cursor-blink -mb-0.5 ml-1 inline-block h-4 w-2 bg-accent align-middle" />
          </p>
        </div>
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

/* Small, literal line icons — one thin weight, no fill. Functional, not decorative. */
function iconProps() {
  return {
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.4,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    className: "h-6 w-6",
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
      <rect x="3" y="4" width="18" height="16" />
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
