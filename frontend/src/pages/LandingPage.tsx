import { useLayoutEffect, useRef } from "react";
import { Link, useLocation } from "react-router-dom";
import { animate, stagger } from "animejs";

import { Button } from "@/components/ui/button";
import { TerminalChrome } from "@/components/ui/card";
import { useAuth } from "@/hooks/useAuth";
import { prefersReducedMotion } from "@/lib/utils";

const TERMINAL_LINES = [
  { prompt: true, text: "kubesandbox create --profile standard --ttl 60m" },
  { prompt: false, text: "✓ vcluster provisioned in 12s" },
  { prompt: true, text: "kubectl get nodes" },
  { prompt: false, text: "NAME        STATUS   AGE\nsandbox-0   Ready    12s" },
  { prompt: true, text: "", cursor: true },
];

const FEATURES: Array<[string, string, string]> = [
  ["isolated", "Real vclusters", "Every sandbox is its own Kubernetes control plane — break it freely."],
  ["ephemeral", "Self-cleaning", "Pick a lifetime from 15 minutes to 24 hours. Expiry deletes everything."],
  ["instant", "Browser terminal", "kubectl is ready in a tab. No kubeconfig, no local install."],
];

export function LandingPage() {
  const { isAuthenticated, loading, user, login } = useAuth();
  const location = useLocation();
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;

  const heroRef = useRef<HTMLDivElement>(null);

  // Page-load sequence: headline block rises, then terminal lines "type" in.
  useLayoutEffect(() => {
    if (prefersReducedMotion() || !heroRef.current) return;
    const blocks = heroRef.current.querySelectorAll("[data-hero-block]");
    const lines = heroRef.current.querySelectorAll("[data-term-line]");
    animate(blocks, {
      opacity: [0, 1],
      translateY: [24, 0],
      duration: 600,
      ease: "out(4)",
      delay: stagger(90),
    });
    animate(lines, {
      opacity: [0, 1],
      translateX: [-10, 0],
      duration: 350,
      ease: "out(3)",
      delay: stagger(280, { start: 500 }),
    });
  }, []);

  return (
    <div ref={heroRef} className="py-8 sm:py-14">
      <div className="grid items-center gap-10 lg:grid-cols-2">
        {/* Thesis */}
        <div>
          <p
            data-hero-block
            className="font-mono text-[11px] uppercase tracking-[0.25em] text-primary"
          >
            $ ephemeral kubernetes playgrounds
          </p>
          <h1
            data-hero-block
            className="mt-4 font-display text-4xl font-bold leading-[1.05] tracking-tight sm:text-5xl"
          >
            A throwaway cluster,
            <br />
            <span className="bg-gradient-to-r from-primary to-accent bg-clip-text text-transparent">
              alive in seconds.
            </span>
          </h1>
          <p data-hero-block className="mt-5 max-w-md text-muted-foreground">
            Isolated vclusters with a browser terminal. Spin one up for a
            workshop, a demo, or an experiment — it cleans itself up when the
            timer runs out.
          </p>
          <div data-hero-block className="mt-7 flex items-center gap-4">
            {isAuthenticated ? (
              <>
                <Link to="/dashboard">
                  <Button>Open dashboard →</Button>
                </Link>
                <span className="max-w-[220px] truncate font-mono text-xs text-muted-foreground">
                  signed in as {user?.profile?.email ?? user?.profile?.name}
                </span>
              </>
            ) : (
              <>
                <Button disabled={loading} onClick={() => login(returnTo)}>
                  Sign in to get started
                </Button>
                <span className="font-mono text-xs text-muted-foreground">
                  via Authentik SSO
                </span>
              </>
            )}
          </div>
        </div>

        {/* The subject itself: a live terminal */}
        <div
          data-hero-block
          className="overflow-hidden rounded-lg border border-border bg-card shadow-card"
        >
          <TerminalChrome title="sandbox — kubectl" />
          <div className="space-y-2.5 p-4 font-mono text-[13px] leading-relaxed">
            {TERMINAL_LINES.map((line, i) => (
              <div key={i} data-term-line className="whitespace-pre-wrap">
                {line.prompt && <span className="text-primary">❯ </span>}
                <span className={line.prompt ? "text-foreground" : "text-muted-foreground"}>
                  {line.text}
                </span>
                {line.cursor && (
                  <span className="cursor-blink -mb-0.5 inline-block h-4 w-2 bg-primary align-middle" />
                )}
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Feature strip */}
      <section id="features" aria-label="Features" className="mt-16 scroll-mt-24">
        <div className="grid gap-3 sm:grid-cols-3">
          {FEATURES.map(([tag, title, body]) => (
            <div
              key={tag}
              data-hero-block
              className="rounded-lg border border-border/70 bg-card/60 p-4 transition-colors hover:border-primary/25"
            >
              <p className="font-mono text-[10px] uppercase tracking-[0.2em] text-primary">
                {tag}
              </p>
              <h2 className="mt-1.5 font-display text-sm font-semibold tracking-tight">
                {title}
              </h2>
              <p className="mt-1 text-sm leading-snug text-muted-foreground">{body}</p>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
