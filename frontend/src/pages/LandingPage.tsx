import React, { useEffect, useRef } from 'react';
import {
  Play,
  Cpu,
  Clock,
  Terminal,
  ShieldAlert,
  Server,
  Compass,
  Cloud,
  ChevronRight,
  Sparkles,
  Zap,
  Lock
} from 'lucide-react';
import { animate, stagger, createTimeline } from 'animejs';

interface LandingPageProps {
  onNavigate: (page: string) => void;
}

export const LandingPage: React.FC<LandingPageProps> = ({ onNavigate }) => {
  const heroRef   = useRef<HTMLElement>(null);
  const cardsRef  = useRef<HTMLElement>(null);
  const archRef   = useRef<HTMLElement>(null);

  /* Hero entrance animation */
  useEffect(() => {
    if (!heroRef.current) return;
    const els = heroRef.current.querySelectorAll('.hero-anim');
    createTimeline({ defaults: { ease: 'out(4)' } })
      .add(els, {
        opacity:    [0, 1],
        translateY: [28, 0],
        duration:   700,
      }, stagger(90))
      .init();
  }, []);

  /* Feature cards staggered entrance */
  useEffect(() => {
    if (!cardsRef.current) return;
    const cards = cardsRef.current.querySelectorAll('.feature-card');
    animate(cards, {
      opacity:    [0, 1],
      translateY: [32, 0],
      scale:      [0.96, 1],
      duration:   600,
      ease:       'out(4)',
      delay:      stagger(90, { start: 300 }),
    });
  }, []);

  /* Architecture section entrance on scroll */
  useEffect(() => {
    if (!archRef.current) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (!entry.isIntersecting) return;
        const items = archRef.current!.querySelectorAll('.arch-anim');
        animate(items, {
          opacity:    [0, 1],
          translateY: [20, 0],
          scale:      [0.95, 1],
          duration:   500,
          ease:       'out(4)',
          delay:      stagger(70),
        });
        observer.disconnect();
      },
      { threshold: 0.15 }
    );
    observer.observe(archRef.current);
    return () => observer.disconnect();
  }, []);

  const features = [
    {
      icon: Cpu,
      color: 'primary',
      title: 'Isolated vClusters',
      desc: 'Get your own fully qualified virtual cluster in seconds. Complete admin access and custom namespaces, totally separated from neighbors.',
    },
    {
      icon: Clock,
      color: 'accent',
      title: 'Ephemeral TTL Lifespans',
      desc: 'Define session TTLs from 15 minutes to 24 hours. Clusters auto-terminate on expiry — no resource waste, no manual cleanup.',
    },
    {
      icon: Terminal,
      color: 'secondary',
      title: 'Direct Shell Access',
      desc: 'Workspaces come pre-baked with kubectl, helm, k9s, and dynamic environments. Launch a live web-shell terminal from your dashboard.',
    },
  ];

  const colorMap: Record<string, { icon: string; border: string; glow: string; bg: string }> = {
    primary:   { icon: 'text-primary',   border: 'border-primary/20',   glow: 'var(--primary-glow)',   bg: 'bg-primary/10' },
    accent:    { icon: 'text-accent',    border: 'border-accent/20',    glow: 'var(--accent-glow)',    bg: 'bg-accent/10' },
    secondary: { icon: 'text-secondary', border: 'border-secondary/20', glow: 'var(--secondary-glow)', bg: 'bg-secondary/10' },
  };

  return (
    <div className="flex flex-col gap-20 py-8 md:py-16">

      {/* ── Hero ── */}
      <section ref={heroRef} className="text-center relative max-w-4xl mx-auto flex flex-col items-center gap-7">

        {/* Badge */}
        <div className="hero-anim flex items-center gap-2 border border-primary/25 bg-primary/6 text-primary px-4 py-1.5 rounded-full text-[12px] font-mono select-none" style={{ opacity: 0 }}>
          <Sparkles className="w-3.5 h-3.5" />
          <span>v1.0 Operational Sandbox Platform</span>
        </div>

        {/* Headline */}
        <h1 className="hero-anim text-4xl sm:text-6xl font-extrabold tracking-tight text-white leading-[1.08] max-w-3xl" style={{ opacity: 0 }}>
          On-Demand{' '}
          <br className="sm:hidden" />
          <span
            className="bg-clip-text text-transparent"
            style={{ backgroundImage: 'linear-gradient(135deg, #a4ff4f 0%, #22d3ee 55%, #818cf8 100%)' }}
          >
            Kubernetes Sandboxes
          </span>
        </h1>

        {/* Sub-copy */}
        <p className="hero-anim text-base sm:text-lg text-muted-foreground max-w-xl leading-relaxed" style={{ opacity: 0 }}>
          Create, test, and tear down fully isolated Kubernetes playground sessions in seconds. Managed automatically by Crossplane. Cleaned up on expiry.
        </p>

        {/* CTAs */}
        <div className="hero-anim flex flex-col sm:flex-row items-center gap-4 mt-2 w-full justify-center" style={{ opacity: 0 }}>
          <button
            onClick={() => onNavigate('dashboard')}
            className="w-full sm:w-auto px-8 py-3.5 bg-primary hover:bg-primary/90 text-primary-foreground font-semibold rounded-xl flex items-center justify-center gap-2 group transition-all duration-250 hover:-translate-y-0.5 hover:shadow-neon-sm"
          >
            Launch Playground
            <Play className="w-4 h-4 fill-current group-hover:translate-x-0.5 transition-transform" />
          </button>

          <button
            onClick={() => {
              const el = document.getElementById('architecture-flow');
              if (el) el.scrollIntoView({ behavior: 'smooth' });
            }}
            className="w-full sm:w-auto px-8 py-3.5 bg-white/5 hover:bg-white/8 text-foreground font-medium rounded-xl border border-white/10 hover:border-white/20 flex items-center justify-center gap-2 transition-all duration-250"
          >
            How It Works
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>

        {/* Ambient under-hero glow */}
        <div
          className="absolute -bottom-12 left-1/2 -translate-x-1/2 w-[400px] h-[120px] pointer-events-none blur-3xl"
          style={{ background: 'radial-gradient(ellipse, rgba(164,255,79,0.06) 0%, transparent 70%)' }}
        />
      </section>

      {/* ── Feature Cards ── */}
      <section ref={cardsRef} className="grid grid-cols-1 md:grid-cols-3 gap-5 max-w-6xl mx-auto w-full">
        {features.map((f) => {
          const c = colorMap[f.color];
          return (
            <div
              key={f.title}
              className={`feature-card glass-panel p-6 flex flex-col gap-4 border-white/6 hover:${c.border} transition-all duration-300 group hover:-translate-y-1.5 cursor-default`}
              style={{ opacity: 0 }}
            >
              <div className={`w-11 h-11 rounded-xl ${c.bg} border ${c.border} flex items-center justify-center group-hover:scale-110 transition-transform duration-300`}>
                <f.icon className={`w-5 h-5 ${c.icon}`} />
              </div>
              <h3 className="text-[15px] font-bold text-foreground tracking-tight">{f.title}</h3>
              <p className="text-[12px] text-muted-foreground leading-relaxed">{f.desc}</p>
            </div>
          );
        })}
      </section>

      {/* ── Architecture Flow ── */}
      <section
        id="architecture-flow"
        ref={archRef}
        className="max-w-5xl mx-auto w-full glass-panel p-8 border-white/6 flex flex-col gap-8"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <h2 className="arch-anim text-2xl font-bold tracking-tight text-white flex items-center gap-2" style={{ opacity: 0 }}>
            <Compass className="w-5 h-5 text-primary" /> Orchestration Flow
          </h2>
          <p className="arch-anim text-[12px] text-muted-foreground max-w-lg leading-relaxed" style={{ opacity: 0 }}>
            KubeSandbox coordinates with Crossplane Compositions via a lightweight async micro-control plane to provision sandboxes instantly.
          </p>
        </div>

        {/* Animated SVG flowchart */}
        <div className="arch-anim w-full overflow-x-auto py-2" style={{ opacity: 0 }}>
          <div className="min-w-[740px] max-w-4xl mx-auto h-[220px] rounded-xl border border-white/6 flex items-center justify-between px-8 relative font-mono text-[10px]"
            style={{ background: 'rgba(8,11,20,0.5)' }}
          >
            {/* Background center line */}
            <div className="absolute inset-x-8 top-1/2 h-px bg-white/5 pointer-events-none" />

            {/* Block: Browser UI */}
            <div className="relative z-10 glass-panel p-3.5 w-[120px] flex flex-col items-center gap-1.5">
              <span className="w-8 h-8 rounded-full bg-primary/10 border border-primary/20 flex items-center justify-center text-primary">
                <Compass className="w-4 h-4" />
              </span>
              <span className="font-bold text-foreground text-[11px]">Browser UI</span>
              <span className="text-[9px] text-muted-foreground">User Trigger</span>
            </div>

            {/* SVG connector 1 */}
            <div className="flex-1 px-3 relative flex items-center justify-center h-full">
              <svg className="w-full h-10" overflow="visible">
                <path d="M 0,20 L 180,20" stroke="rgba(164,255,79,0.15)" strokeWidth="1.5" strokeDasharray="4 3" fill="none" />
                <circle r="3.5" fill="#a4ff4f" filter="drop-shadow(0 0 5px #a4ff4f)">
                  <animateMotion dur="2.8s" repeatCount="indefinite" path="M 0,20 L 180,20" />
                </circle>
              </svg>
            </div>

            {/* Block: Envoy & Go API */}
            <div className="relative z-10 glass-panel p-3.5 w-[130px] flex flex-col items-center gap-1.5">
              <span className="w-8 h-8 rounded-full bg-accent/10 border border-accent/20 flex items-center justify-center text-accent">
                <Cloud className="w-4 h-4" />
              </span>
              <span className="font-bold text-foreground text-[11px]">Envoy & Go API</span>
              <span className="text-[9px] text-accent">REST & SSE</span>
            </div>

            {/* SVG connector 2 */}
            <div className="flex-1 px-3 relative flex items-center justify-center h-full">
              <svg className="w-full h-10" overflow="visible">
                <path d="M 0,20 L 180,20" stroke="rgba(34,211,238,0.15)" strokeWidth="1.5" strokeDasharray="4 3" fill="none" />
                <circle r="3.5" fill="#22d3ee" filter="drop-shadow(0 0 5px #22d3ee)">
                  <animateMotion dur="2.2s" repeatCount="indefinite" path="M 0,20 L 180,20" />
                </circle>
              </svg>
            </div>

            {/* Block: Crossplane */}
            <div className="relative z-10 glass-panel p-3.5 w-[130px] flex flex-col items-center gap-1.5">
              <span className="w-8 h-8 rounded-full bg-secondary/10 border border-secondary/20 flex items-center justify-center text-secondary">
                <Server className="w-4 h-4" />
              </span>
              <span className="font-bold text-foreground text-[11px]">Crossplane CRD</span>
              <span className="text-[9px] text-secondary">Composer</span>
            </div>

            {/* SVG connector 3 */}
            <div className="flex-1 px-3 relative flex items-center justify-center h-full">
              <svg className="w-full h-10" overflow="visible">
                <path d="M 0,20 L 180,20" stroke="rgba(129,140,248,0.15)" strokeWidth="1.5" strokeDasharray="4 3" fill="none" />
                <circle r="3.5" fill="#818cf8" filter="drop-shadow(0 0 5px #818cf8)">
                  <animateMotion dur="1.8s" repeatCount="indefinite" path="M 0,20 L 180,20" />
                </circle>
              </svg>
            </div>

            {/* Block: vCluster Ready */}
            <div className="relative z-10 glass-panel-primary p-3.5 w-[120px] flex flex-col items-center gap-1.5">
              <span className="w-8 h-8 rounded-full bg-primary/20 border border-primary/30 flex items-center justify-center text-primary">
                <Terminal className="w-4 h-4" />
              </span>
              <span className="font-bold text-foreground text-[11px]">vCluster Ready</span>
              <span className="text-[9px] text-primary text-neon">Isolated</span>
            </div>
          </div>
        </div>

        {/* Security notice */}
        <div className="arch-anim flex items-center justify-center gap-2 bg-warning/6 border border-warning/15 text-warning/80 rounded-xl p-3 max-w-lg mx-auto text-[11px] font-mono" style={{ opacity: 0 }}>
          <ShieldAlert className="w-4 h-4 flex-shrink-0" />
          <span>Security Reminder: Sandboxes expire automatically. Backup critical experiments before TTL ends.</span>
        </div>
      </section>
    </div>
  );
};
