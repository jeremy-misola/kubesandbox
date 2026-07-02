import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { animate, stagger } from "animejs";

import { Button } from "@/components/ui/button";
import { TerminalChrome } from "@/components/ui/card";
import { embeddedTerminalUrl, terminalUrl } from "@/config";
import { cn, prefersReducedMotion } from "@/lib/utils";

/**
 * The ttyd terminal, embedded and dressed to match the app.
 *
 * The terminal (/s/{id}) is served by the backend behind its own OIDC
 * ext-authz: the first unauthenticated hit 302s to Authentik, whose login
 * page refuses to be framed. So before rendering the iframe we PROBE the
 * terminal URL with `redirect: "manual"`:
 *
 *   200            → session cookie is warm, safe to embed
 *   opaqueredirect → needs the backend's own login — hand off to a popup,
 *                    then re-probe until the cookie lands
 *
 * Once embeddable, the iframe loads ttyd with theme/font query overrides
 * (see config.ts) so the shell inherits the Ink/Seafoam palette, behind a
 * short boot-sequence overlay that fades to reveal the live pty.
 */

type Phase = "probing" | "auth" | "booting" | "live" | "error";

const MIN_BOOT_MS = 1100;

function bootLines(id: string): string[] {
  return [
    `$ kubesandbox attach ${id}`,
    "▸ resolving session endpoint … ok",
    "▸ checking authorization … ok",
    "▸ streaming pty over websocket …",
  ];
}

export function TerminalFrame({
  id,
  className,
}: {
  id: string;
  className?: string;
}) {
  const [phase, setPhase] = useState<Phase>("probing");
  const [frameKey, setFrameKey] = useState(0); // bump to force-reload iframe
  const [isFullscreen, setIsFullscreen] = useState(false);

  const shellRef = useRef<HTMLDivElement>(null);
  const overlayRef = useRef<HTMLDivElement>(null);
  const bootStartedAt = useRef(0);
  const revealTimer = useRef<number | undefined>(undefined);

  const probe = useCallback(async (): Promise<"ok" | "auth" | "error"> => {
    try {
      const res = await fetch(terminalUrl(id), {
        redirect: "manual",
        cache: "no-store",
      });
      if (res.type === "opaqueredirect") return "auth";
      if (res.status === 401 || res.status === 403) return "auth";
      if (res.ok) return "ok";
      return "error";
    } catch {
      return "error";
    }
  }, [id]);

  const connect = useCallback(async () => {
    setPhase("probing");
    const result = await probe();
    if (result === "ok") {
      bootStartedAt.current = Date.now();
      setFrameKey((k) => k + 1);
      setPhase("booting");
    } else {
      setPhase(result);
    }
  }, [probe]);

  useEffect(() => {
    void connect();
    return () => window.clearTimeout(revealTimer.current);
  }, [connect]);

  // While waiting on the popup login: re-probe on focus + a slow poll, and
  // flip straight into the boot sequence the moment the cookie lands.
  useEffect(() => {
    if (phase !== "auth") return;
    let cancelled = false;
    const check = async () => {
      if ((await probe()) === "ok" && !cancelled) void connect();
    };
    const interval = window.setInterval(check, 3000);
    window.addEventListener("focus", check);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
      window.removeEventListener("focus", check);
    };
  }, [phase, probe, connect]);

  // Boot overlay: type the sequence in, staggered.
  useLayoutEffect(() => {
    if (phase !== "booting" || !overlayRef.current) return;
    if (prefersReducedMotion()) return;
    animate(overlayRef.current.querySelectorAll("[data-boot-line]"), {
      opacity: [0, 1],
      translateY: [6, 0],
      duration: 260,
      ease: "out(2)",
      delay: stagger(180),
    });
  }, [phase, frameKey]);

  // The iframe finished loading — hold the overlay to its minimum beat,
  // then fade it out to reveal the pty.
  const handleFrameLoad = useCallback(() => {
    const reveal = () => {
      if (prefersReducedMotion() || !overlayRef.current) {
        setPhase("live");
        return;
      }
      animate(overlayRef.current, {
        opacity: [1, 0],
        duration: 420,
        ease: "out(2)",
        onComplete: () => setPhase("live"),
      });
    };
    const elapsed = Date.now() - bootStartedAt.current;
    revealTimer.current = window.setTimeout(
      reveal,
      Math.max(0, MIN_BOOT_MS - elapsed),
    );
  }, []);

  const toggleFullscreen = useCallback(() => {
    if (document.fullscreenElement) void document.exitFullscreen();
    else void shellRef.current?.requestFullscreen();
  }, []);

  useEffect(() => {
    const onChange = () => setIsFullscreen(!!document.fullscreenElement);
    document.addEventListener("fullscreenchange", onChange);
    return () => document.removeEventListener("fullscreenchange", onChange);
  }, []);

  const showFrame = phase === "booting" || phase === "live";

  return (
    <div
      ref={shellRef}
      className={cn(
        "term-shell flex flex-col overflow-hidden rounded-lg border border-border bg-card shadow-card",
        "focus-within:border-primary/40",
        className,
      )}
    >
      <TerminalChrome title={`s/${id} — zsh`} className="shrink-0">
        <span className="ml-auto flex items-center gap-1">
          {showFrame && (
            <span
              className="mr-2 hidden items-center gap-1.5 font-mono text-[10px] uppercase tracking-widest text-success sm:flex"
              aria-hidden
            >
              <i className="h-1.5 w-1.5 rounded-full bg-success" />
              connected
            </span>
          )}
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 font-mono text-[11px]"
            onClick={() => void connect()}
            title="Reconnect"
          >
            ⟳ reconnect
          </Button>
          <a href={terminalUrl(id)} target="_blank" rel="noreferrer">
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2 font-mono text-[11px]"
              title="Open in a new tab"
            >
              ↗ new tab
            </Button>
          </a>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 font-mono text-[11px]"
            onClick={toggleFullscreen}
            title={isFullscreen ? "Exit full screen" : "Full screen"}
          >
            {isFullscreen ? "⤡ exit" : "⤢ full screen"}
          </Button>
        </span>
      </TerminalChrome>

      <div className="term-viewport relative min-h-0 flex-1 bg-[#080d0c]">
        {showFrame && (
          <iframe
            key={frameKey}
            src={embeddedTerminalUrl(id)}
            title={`Terminal for session ${id}`}
            className="block h-full w-full border-0 bg-[#080d0c]"
            allow="clipboard-read; clipboard-write"
            onLoad={handleFrameLoad}
          />
        )}

        {/* Boot-sequence overlay — sits on top of the iframe while it loads. */}
        {phase !== "live" && (
          <div
            ref={overlayRef}
            className="term-scanlines absolute inset-0 z-10 flex flex-col bg-[#080d0c] p-5"
          >
            {phase === "probing" && (
              <p className="font-mono text-sm text-muted-foreground">
                <span className="text-primary">❯</span> connecting
                <span className="cursor-blink -mb-0.5 ml-1 inline-block h-4 w-2 bg-primary align-middle" />
              </p>
            )}

            {phase === "booting" && (
              <div className="space-y-1.5 font-mono text-[13px] leading-relaxed">
                {bootLines(id).map((line, i) => (
                  <p
                    key={line}
                    data-boot-line
                    className={cn(
                      i === 0 ? "text-foreground" : "text-muted-foreground",
                    )}
                  >
                    {line.startsWith("$") ? (
                      <>
                        <span className="text-primary">$</span>
                        {line.slice(1)}
                      </>
                    ) : (
                      line
                    )}
                  </p>
                ))}
                <p className="pt-1 font-mono text-[13px]" data-boot-line>
                  <span className="cursor-blink inline-block h-4 w-2 bg-primary align-middle" />
                </p>
              </div>
            )}

            {phase === "auth" && (
              <div className="m-auto max-w-sm text-center">
                <p className="font-mono text-sm text-muted-foreground">
                  <span className="text-primary">❯</span> authorization required
                </p>
                <h2 className="mt-3 font-display text-lg font-bold tracking-tight">
                  Unlock this terminal
                </h2>
                <p className="mt-2 text-sm text-muted-foreground">
                  The terminal has its own sign-in that can't run inside an
                  embedded frame. Authorize it once in a popup — this panel
                  connects by itself when you're done.
                </p>
                <Button
                  className="mt-5"
                  onClick={() =>
                    window.open(terminalUrl(id), "_blank", "noopener")
                  }
                >
                  Authorize terminal
                </Button>
                <p className="mt-3 font-mono text-[11px] text-muted-foreground/70">
                  waiting for authorization
                  <span className="cursor-blink -mb-0.5 ml-1 inline-block h-3 w-1.5 bg-muted-foreground/70 align-middle" />
                </p>
              </div>
            )}

            {phase === "error" && (
              <div className="m-auto max-w-sm text-center">
                <p className="font-mono text-sm text-danger">
                  ✗ couldn't reach the terminal
                </p>
                <p className="mt-2 text-sm text-muted-foreground">
                  The session endpoint didn't answer — the sandbox may still be
                  provisioning, or it may have expired.
                </p>
                <Button
                  variant="secondary"
                  className="mt-5"
                  onClick={() => void connect()}
                >
                  Try again
                </Button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
