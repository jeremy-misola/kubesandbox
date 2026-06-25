import React, { useState, useEffect, useRef } from 'react';
import { useKubeSandbox } from '../context/KubeSandboxContext';
import { AnimeText } from './AnimeText';
import { 
  Terminal, 
  User, 
  Settings, 
  Cpu, 
  Globe, 
  Database,
  ShieldCheck,
  ToggleLeft,
  ToggleRight
} from 'lucide-react';
import { animate, stagger } from 'animejs';

interface AppShellProps {
  children: React.ReactNode;
  activePage: string;
  onNavigate: (page: string) => void;
}

export const AppShell: React.FC<AppShellProps> = ({ children, activePage, onNavigate }) => {
  const { user, networkStatus, mockMode, toggleMockMode } = useKubeSandbox();
  const [showConfigPanel, setShowConfigPanel] = useState(false);
  const navRef = useRef<HTMLElement>(null);
  const headerRef = useRef<HTMLDivElement>(null);

  /* Animate nav items on mount */
  useEffect(() => {
    if (!navRef.current) return;
    const items = navRef.current.querySelectorAll('.nav-item');
    animate(items, {
      opacity:    [0, 1],
      translateY: [-12, 0],
      duration:   500,
      ease:       'out(4)',
      delay:      stagger(60, { start: 200 }),
    });
  }, []);

  /* Subtle header entrance */
  useEffect(() => {
    if (!headerRef.current) return;
    animate(headerRef.current, {
      opacity:    [0, 1],
      translateY: [-8, 0],
      duration:   600,
      ease:       'out(4)',
    });
  }, []);

  const getStatusConfig = () => {
    switch (networkStatus) {
      case 'connected': return {
        bar:   'text-success bg-success/10 border-success/25',
        dot:   'bg-success',
        label: 'Cluster Live',
        pulse: 'pulse-ring',
      };
      case 'mocked': return {
        bar:   'text-accent bg-accent/10 border-accent/25',
        dot:   'bg-accent',
        label: 'Simulator',
        pulse: 'pulse-ring-cyan',
      };
      case 'offline': return {
        bar:   'text-error bg-error/10 border-error/25',
        dot:   'bg-error',
        label: 'Offline',
        pulse: '',
      };
    }
  };

  const status = getStatusConfig();

  const navLinks = [
    { href: 'landing',    label: '/home',     match: (p: string) => p === 'landing' },
    { href: 'dashboard',  label: '/sandboxes', match: (p: string) => p === 'dashboard' || p.startsWith('sessions') },
    { href: 'settings',   label: '/settings',  match: (p: string) => p === 'settings' },
  ];

  return (
    <div className="min-h-screen flex flex-col relative text-foreground">
      {/* Cyber grid overlay */}
      <div className="absolute inset-0 cyber-grid opacity-25 pointer-events-none z-[-5]" />

      {/* Ambient radial top glow */}
      <div className="absolute top-0 left-1/2 -translate-x-1/2 w-full max-w-[900px] h-[320px] pointer-events-none z-[-5]"
        style={{
          background: 'radial-gradient(ellipse at top, rgba(164,255,79,0.04) 0%, rgba(34,211,238,0.02) 50%, transparent 70%)',
        }}
      />

      {/* ── Header ── */}
      <header className="sticky top-0 z-40 w-full px-4 sm:px-6 py-3">
        <div
          ref={headerRef}
          className="max-w-7xl mx-auto flex items-center justify-between glass-panel px-5 py-2.5"
          style={{ opacity: 0 }}
        >
          {/* Logo */}
          <div
            className="flex items-center gap-3 cursor-pointer select-none group"
            onClick={() => onNavigate('landing')}
          >
            <div className="relative flex items-center justify-center w-9 h-9 rounded-lg bg-primary/10 border border-primary/25 group-hover:border-primary/60 group-hover:bg-primary/15 transition-all duration-300">
              <Terminal className="w-4.5 h-4.5 text-primary group-hover:scale-110 transition-transform duration-300" />
              <div className="absolute inset-0 rounded-lg bg-primary/20 blur-sm opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            </div>
            <div className="flex flex-col">
              <span className="text-[17px] font-bold tracking-tight text-white flex items-center gap-1">
                KUBE
                <span className="text-primary text-neon font-extrabold">
                  <AnimeText text="SANDBOX" triggerOnMount={true} triggerOnHover={true} />
                </span>
              </span>
              <span className="text-[9px] tracking-[0.18em] text-muted-foreground uppercase font-mono leading-none">
                Control Plane v1.0
              </span>
            </div>
          </div>

          {/* Nav */}
          <nav ref={navRef} className="hidden md:flex items-center gap-0.5 font-mono text-[13px]">
            {navLinks.map((link, i) => {
              const active = link.match(activePage);
              return (
                <button
                  key={link.href}
                  onClick={() => onNavigate(link.href)}
                  className={`nav-item px-4 py-2 rounded-md transition-all duration-200 relative ${
                    active
                      ? 'text-primary bg-primary/10'
                      : 'text-muted-foreground hover:text-foreground hover:bg-white/5'
                  }`}
                  style={{ animationDelay: `${i * 60 + 200}ms` }}
                >
                  {link.label}
                  {active && (
                    <span className="absolute bottom-0 left-3 right-3 h-[1.5px] bg-primary/60 rounded-full" />
                  )}
                </button>
              );
            })}
          </nav>

          {/* Right controls */}
          <div className="flex items-center gap-3">
            {/* Status badge */}
            <div className={`flex items-center gap-2 border px-2.5 py-1 rounded-full text-[11px] font-mono select-none ${status.bar}`}>
              <span className="relative flex w-1.5 h-1.5">
                <span className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${status.dot}`} />
                <span className={`relative inline-flex rounded-full h-1.5 w-1.5 ${status.dot}`} />
              </span>
              <span>{status.label}</span>
            </div>

            {/* User chip */}
            {user ? (
              <div className="flex items-center gap-2.5 bg-white/5 border border-white/8 px-2.5 py-1.5 rounded-lg">
                <div className="w-6 h-6 rounded bg-primary/20 flex items-center justify-center text-primary font-mono text-[10px] border border-primary/25 font-bold">
                  {user.name.split(' ').map(n => n[0]).join('')}
                </div>
                <div className="hidden sm:flex flex-col text-left">
                  <span className="text-[11px] font-medium text-foreground max-w-[90px] truncate leading-none">{user.name}</span>
                  <span className="text-[9px] text-muted-foreground truncate max-w-[90px] leading-none mt-0.5">{user.email}</span>
                </div>
              </div>
            ) : (
              <div className="w-8 h-8 rounded-lg bg-white/5 flex items-center justify-center border border-white/8">
                <User className="w-4 h-4 text-muted-foreground" />
              </div>
            )}
          </div>
        </div>
      </header>

      {/* ── Main Content ── */}
      <main className="flex-1 w-full max-w-7xl mx-auto px-4 sm:px-6 py-8 relative">
        {children}
      </main>

      {/* ── Footer ── */}
      <footer className="w-full border-t border-white/5 mt-auto">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-5 flex flex-col sm:flex-row items-center justify-between text-[11px] text-muted-foreground font-mono gap-3">
          <div className="flex items-center gap-3">
            <Database className="w-3.5 h-3.5 text-muted-foreground/60" />
            <span>Tenant: <span className="text-foreground/80">demo-tenant</span></span>
            <span className="text-white/15">|</span>
            <Cpu className="w-3.5 h-3.5 text-muted-foreground/60" />
            <span>Composition: <span className="text-foreground/80">vcluster-compose</span></span>
          </div>
          <div className="flex items-center gap-3">
            <span className="hover:text-primary transition-colors cursor-pointer">API docs</span>
            <span className="text-white/15">•</span>
            <span className="hover:text-primary transition-colors cursor-pointer">Security</span>
            <span className="text-white/15">•</span>
            <span className="text-primary font-bold text-neon">KubeSandbox v1.0.0</span>
          </div>
        </div>
      </footer>

      {/* ── Dev Panel FAB ── */}
      <div className="fixed bottom-5 right-5 z-50">
        <button
          onClick={() => setShowConfigPanel(!showConfigPanel)}
          title="Environment Console"
          className={`flex items-center justify-center w-10 h-10 rounded-full cursor-pointer transition-all duration-300 border shadow-lg ${
            mockMode
              ? 'bg-accent/15 border-accent/40 text-accent hover:bg-accent/25 shadow-cyan-sm pulse-ring-cyan'
              : 'bg-surface border-white/10 text-muted-foreground hover:bg-white/10 hover:text-foreground'
          }`}
        >
          <Settings className={`w-4.5 h-4.5 transition-transform duration-300 ${showConfigPanel ? 'rotate-90' : ''}`} />
        </button>

        {showConfigPanel && (
          <div className="absolute bottom-12 right-0 w-80 glass-panel-accent p-4 shadow-2xl animate-fade-in">
            <div className="flex items-center justify-between border-b border-white/8 pb-2 mb-3">
              <span className="text-[11px] font-bold font-mono tracking-wider text-accent flex items-center gap-1.5">
                <Globe className="w-3.5 h-3.5" /> ENVIRONMENT CONSOLE
              </span>
              <span className="text-[10px] bg-accent/10 text-accent px-1.5 py-0.5 rounded font-mono uppercase tracking-wider">Dev</span>
            </div>

            <div className="space-y-3 font-mono text-[11px]">
              <p className="text-muted-foreground leading-relaxed">
                KubeSandbox requires a running Kubernetes cluster. Use simulator mode to test the full application locally.
              </p>

              <div className="flex items-center justify-between bg-white/5 p-3 rounded-lg border border-white/8">
                <div className="flex flex-col gap-0.5">
                  <span className="text-foreground font-medium">Simulator Engine</span>
                  <span className="text-[10px] text-muted-foreground">Fakes Go REST API & SSE</span>
                </div>
                <button
                  onClick={() => toggleMockMode(!mockMode)}
                  className={`flex items-center transition-colors ${mockMode ? 'text-accent' : 'text-muted-foreground hover:text-foreground'}`}
                >
                  {mockMode ? <ToggleRight className="w-8 h-8" /> : <ToggleLeft className="w-8 h-8" />}
                </button>
              </div>

              <div className="space-y-1 text-[10px] bg-black/40 p-2.5 rounded-lg border border-white/8">
                {[
                  ['Go Endpoint', 'http://localhost:8080', 'text-surface-raised'],
                  ['Active Mock', mockMode ? 'Enabled (Stateless)' : 'Disabled', mockMode ? 'text-accent' : 'text-muted-foreground'],
                  ['SSE Updates', mockMode ? 'Mock SSE Streaming' : 'Native EventSource', mockMode ? 'text-accent' : 'text-muted-foreground'],
                ].map(([key, val, cls]) => (
                  <div key={key as string} className="flex justify-between">
                    <span className="text-muted-foreground">{key}:</span>
                    <span className={cls as string}>{val}</span>
                  </div>
                ))}
              </div>

              {mockMode && (
                <div className="flex items-center gap-1.5 text-[10px] text-accent/80 bg-accent/8 p-2 rounded-lg border border-accent/15">
                  <ShieldCheck className="w-3.5 h-3.5 flex-shrink-0" />
                  <span>Interactive mock is persistent. Changes are stored locally.</span>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
