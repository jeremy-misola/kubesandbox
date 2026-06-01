import React, { useEffect, useRef } from 'react';
import { useKubeSandbox } from '../context/KubeSandboxContext';
import {
  PlusCircle,
  Terminal,
  RefreshCw,
  Trash2,
  AlertOctagon,
  Layers,
  Cpu,
  Clock,
  AlertCircle,
  ArrowUpRight,
  Zap
} from 'lucide-react';
import { animate, stagger, createTimeline } from 'animejs';

interface DashboardPageProps {
  onNavigate: (page: string) => void;
  onSelectSession: (name: string) => void;
}

export const DashboardPage: React.FC<DashboardPageProps> = ({ onNavigate, onSelectSession }) => {
  const {
    sessions,
    loading,
    error,
    deleteSession,
    fetchSessions,
    clearError,
    networkStatus
  } = useKubeSandbox();

  const metricsRef = useRef<HTMLDivElement>(null);
  const cardsRef   = useRef<HTMLDivElement>(null);

  /* Animate metric cards on load */
  useEffect(() => {
    if (!metricsRef.current || loading) return;
    const cards = metricsRef.current.querySelectorAll('.metric-card');
    animate(cards, {
      opacity:    [0, 1],
      translateY: [20, 0],
      scale:      [0.97, 1],
      duration:   500,
      ease:       'out(4)',
      delay:      stagger(80, { start: 100 }),
    });
  }, [sessions, loading]);

  /* Animate session cards on load */
  useEffect(() => {
    if (!cardsRef.current || loading || sessions.length === 0) return;
    const cards = cardsRef.current.querySelectorAll('.session-card');
    animate(cards, {
      opacity:    [0, 1],
      translateY: [28, 0],
      scale:      [0.96, 1],
      duration:   550,
      ease:       'out(4)',
      delay:      stagger(70, { start: 200 }),
    });
  }, [sessions, loading]);

  const getPhaseConfig = (phase: string) => {
    switch (phase) {
      case 'Ready':        return { cls: 'bg-success/10 text-success border-success/25', dot: 'bg-success' };
      case 'Provisioning': return { cls: 'bg-accent/10 text-accent border-accent/25',   dot: 'bg-accent'  };
      case 'Error':        return { cls: 'bg-error/10  text-error border-error/25',     dot: 'bg-error'   };
      default:             return { cls: 'bg-white/5 text-muted-foreground border-white/10', dot: 'bg-muted-foreground' };
    }
  };

  const getProfileBadge = (profile: string) => {
    switch (profile) {
      case 'starter':  return 'border-success/25  text-success  bg-success/8';
      case 'standard': return 'border-accent/25   text-accent   bg-accent/8';
      case 'advanced': return 'border-secondary/25 text-secondary bg-secondary/8';
      default: return 'border-white/10 text-muted-foreground';
    }
  };

  const getRemainingTime = (expiryString: string) => {
    try {
      const diff = new Date(expiryString).getTime() - Date.now();
      if (diff <= 0) return 'Expired';
      const mins = Math.floor(diff / 60000);
      if (mins < 60) return `${mins}m left`;
      const hrs = Math.floor(mins / 60);
      return `${hrs}h ${mins % 60}m left`;
    } catch { return 'N/A'; }
  };

  /* ── Error State ── */
  if (error) {
    return (
      <div className="max-w-md mx-auto py-12 flex flex-col gap-6 text-center animate-fade-in">
        <div className="glass-panel-error p-8 flex flex-col items-center gap-5">
          <div className="w-14 h-14 rounded-2xl bg-error/10 border border-error/25 flex items-center justify-center text-error">
            <AlertOctagon className="w-7 h-7" />
          </div>
          <div className="flex flex-col gap-2">
            <h3 className="text-base font-bold text-foreground font-mono tracking-wide">CONNECTION_FAILURE</h3>
            <p className="text-[12px] text-muted-foreground leading-relaxed">{error}</p>
          </div>
          <div className="flex items-center gap-3 w-full">
            <button
              onClick={() => { clearError(); fetchSessions(); }}
              className="flex-1 px-4 py-2.5 bg-white/5 hover:bg-white/10 text-foreground rounded-xl border border-white/10 flex items-center justify-center gap-2 text-[12px] font-mono transition-all"
            >
              <RefreshCw className="w-3.5 h-3.5" /> Reconnect
            </button>
            <button
              onClick={clearError}
              className="flex-1 px-4 py-2.5 bg-error/10 hover:bg-error/20 text-error rounded-xl border border-error/20 text-[12px] font-mono transition-all"
            >
              Acknowledge
            </button>
          </div>
        </div>
      </div>
    );
  }

  /* ── Loading Skeleton ── */
  if (loading && sessions.length === 0) {
    return (
      <div className="flex flex-col gap-8 w-full animate-fade-in">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
          {[1, 2, 3].map(i => (
            <div key={i} className="glass-panel p-6 border-white/5 relative overflow-hidden h-24">
              <div className="absolute inset-0 skeleton-shimmer rounded-xl" />
            </div>
          ))}
        </div>
        <div className="flex items-center justify-between border-b border-white/8 pb-4">
          <div className="h-5 w-36 bg-white/8 rounded skeleton-shimmer" />
          <div className="h-9 w-32 bg-white/8 rounded-xl skeleton-shimmer" />
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
          {[1, 2, 3, 4].map(i => (
            <div key={i} className="glass-panel p-6 border-white/5 relative overflow-hidden h-44">
              <div className="absolute inset-0 skeleton-shimmer rounded-xl" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  /* ── Empty State ── */
  if (sessions.length === 0) {
    return (
      <div className="max-w-2xl mx-auto py-10 flex flex-col gap-8 items-center text-center animate-fade-in">
        <div className="glass-panel p-10 max-w-md w-full border-white/6 flex flex-col items-center gap-6 relative overflow-hidden">
          <div className="absolute top-0 right-0 w-40 h-40 pointer-events-none"
            style={{ background: 'radial-gradient(circle at top right, rgba(164,255,79,0.05), transparent 70%)' }}
          />

          <div className="w-16 h-16 rounded-2xl bg-primary/10 border border-primary/20 flex items-center justify-center text-primary relative">
            <Layers className="w-8 h-8" />
            <span className="absolute -top-1.5 -right-1.5 flex h-3.5 w-3.5">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary/60" />
              <span className="relative inline-flex rounded-full h-3.5 w-3.5 bg-primary" />
            </span>
          </div>

          <div className="flex flex-col gap-2">
            <h2 className="text-xl font-bold text-foreground tracking-tight">No Active Sandbox Sessions</h2>
            <p className="text-[12px] text-muted-foreground leading-relaxed max-w-sm">
              Launch a fast, dedicated virtual cluster using pre-packaged presets. Your workspace will be operational in under a minute.
            </p>
          </div>

          <button
            onClick={() => onNavigate('sessions/new')}
            className="w-full py-3.5 bg-primary hover:bg-primary/90 text-primary-foreground font-semibold rounded-xl flex items-center justify-center gap-2 group transition-all hover:-translate-y-0.5 hover:shadow-neon-sm"
          >
            <PlusCircle className="w-4 h-4" />
            Create First Sandbox
          </button>
        </div>

        {networkStatus === 'mocked' && (
          <div className="flex items-center gap-2 border border-accent/20 bg-accent/8 text-accent px-4 py-3 rounded-xl text-[12px] font-mono">
            <AlertCircle className="w-4 h-4 flex-shrink-0" />
            <span>Simulator Mode is active — sandbox creation will be instantly simulated.</span>
          </div>
        )}
      </div>
    );
  }

  /* ── Sessions Grid ── */
  return (
    <div className="flex flex-col gap-8 w-full animate-fade-in font-mono text-[12px]">

      {/* Metric Cards */}
      <div ref={metricsRef} className="grid grid-cols-1 sm:grid-cols-3 gap-5">
        {[
          {
            label: 'Total Active',
            icon: Layers,
            value: sessions.length,
            valueClass: 'text-foreground',
          },
          {
            label: 'Allocated Cores',
            icon: Cpu,
            value: sessions.reduce((acc, s) => acc + parseInt(s.resources?.cpu || '0'), 0),
            valueClass: 'text-primary text-neon',
          },
          {
            label: 'Expires Soonest',
            icon: Clock,
            value: sessions.length > 0 ? getRemainingTime(sessions[0].expiresAt) : 'N/A',
            valueClass: 'text-foreground truncate',
          },
        ].map((m, i) => (
          <div
            key={m.label}
            className="metric-card glass-panel p-6 border-white/6 flex flex-col gap-1.5 relative overflow-hidden"
            style={{ opacity: 0 }}
          >
            <div className="absolute inset-0 pointer-events-none"
              style={{ background: i === 1 ? 'radial-gradient(circle at top right, rgba(164,255,79,0.04), transparent 60%)' : 'none' }}
            />
            <span className="text-[10px] text-muted-foreground uppercase font-bold tracking-wider flex items-center gap-1.5">
              <m.icon className="w-3.5 h-3.5" /> {m.label}
            </span>
            <span className={`text-3xl font-extrabold font-sans mt-1 ${m.valueClass}`}>{m.value}</span>
          </div>
        ))}
      </div>

      {/* Session list header */}
      <div className="flex items-center justify-between border-b border-white/8 pb-4">
        <h2 className="text-base font-bold text-foreground tracking-tight flex items-center gap-2">
          <Terminal className="w-4 h-4 text-primary" /> Active Sandboxes
        </h2>
        <div className="flex items-center gap-2.5">
          <button
            onClick={fetchSessions}
            className="p-2.5 bg-white/5 hover:bg-white/10 text-foreground rounded-xl border border-white/10 hover:border-white/20 transition-all"
            title="Sync Environment"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={() => onNavigate('sessions/new')}
            className="px-4 py-2.5 bg-primary hover:bg-primary/90 text-primary-foreground font-semibold rounded-xl flex items-center gap-1.5 text-[12px] transition-all hover:shadow-neon-sm"
          >
            <PlusCircle className="w-3.5 h-3.5" /> New Sandbox
          </button>
        </div>
      </div>

      {/* Session cards grid */}
      <div ref={cardsRef} className="grid grid-cols-1 md:grid-cols-2 gap-5">
        {sessions.map((session, index) => {
          const phase = getPhaseConfig(session.phase);
          return (
            <div
              key={session.name}
              className="session-card glass-panel p-6 border-white/6 hover:border-white/12 flex flex-col gap-4 relative overflow-hidden group cursor-pointer select-none transition-all duration-200 hover:-translate-y-1"
              onClick={() => onSelectSession(session.name)}
              style={{ opacity: 0 }}
            >
              {/* Provisioning top bar */}
              {session.phase === 'Provisioning' && (
                <div className="absolute top-0 inset-x-0 h-[2px] bg-accent/15 overflow-hidden">
                  <div className="h-full bg-accent animate-stripe rounded" />
                </div>
              )}

              {/* Hover ambient glow */}
              <div className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none rounded-xl"
                style={{ background: 'radial-gradient(circle at top left, rgba(164,255,79,0.03), transparent 60%)' }}
              />

              {/* Header row */}
              <div className="flex justify-between items-start">
                <div className="flex flex-col gap-0.5">
                  <span className="text-[14px] font-bold text-foreground tracking-tight flex items-center gap-1.5 group-hover:text-primary transition-colors">
                    {session.name}
                    <ArrowUpRight className="w-3.5 h-3.5 opacity-0 group-hover:opacity-100 transition-all group-hover:translate-x-0.5 group-hover:-translate-y-0.5" />
                  </span>
                  <span className="text-[10px] text-muted-foreground">{session.tenantRef} • {session.workspaceImage}</span>
                </div>

                <div className={`border px-2.5 py-1 rounded-lg text-[10px] uppercase font-bold tracking-wider flex items-center gap-1.5 ${phase.cls}`}>
                  <span className={`w-1.5 h-1.5 rounded-full ${phase.dot}`} />
                  {session.phase}
                </div>
              </div>

              {/* Specs row */}
              <div className="flex items-center gap-3 text-[11px] text-muted-foreground flex-wrap">
                <span className={`border px-2.5 py-1 rounded-lg font-mono text-[10px] ${getProfileBadge(session.profile)}`}>
                  {session.profile}
                </span>
                <span>CPU: <strong className="text-foreground">{session.resources?.cpu || 'N/A'}</strong></span>
                <span>RAM: <strong className="text-foreground">{session.resources?.memory || 'N/A'}</strong></span>
              </div>

              {/* Footer row */}
              <div className="border-t border-white/6 pt-3 flex items-center justify-between text-[10px]">
                <span className="text-muted-foreground max-w-[200px] truncate">{session.message}</span>
                <span className="text-primary font-bold text-neon">{getRemainingTime(session.expiresAt)}</span>
              </div>

              {/* Hover action buttons */}
              <div className="absolute top-4 right-4 flex items-center gap-1.5 opacity-0 group-hover:opacity-100 transition-opacity bg-background/90 backdrop-blur-sm px-2 py-1.5 rounded-lg border border-white/8">
                <button
                  onClick={e => { e.stopPropagation(); deleteSession(session.name); }}
                  className="p-1 text-muted-foreground hover:text-error hover:bg-error/10 rounded transition-all"
                  title="Tear Down Sandbox"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
