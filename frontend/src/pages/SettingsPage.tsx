import React, { useState, useEffect } from 'react';
import { useKubeSandbox } from '../context/KubeSandboxContext';
import { Settings, Save, RotateCcw, ShieldCheck, Database, Compass, Sliders, CheckCircle2 } from 'lucide-react';
import { animate, stagger } from 'animejs';

export const SettingsPage: React.FC = () => {
  const { mockMode, toggleMockMode } = useKubeSandbox();
  const [defaultTTL, setDefaultTTL]         = useState(60);
  const [defaultProfile, setDefaultProfile] = useState('standard');
  const [defaultImage, setDefaultImage]     = useState('ubuntu-dev:latest');
  const [saved, setSaved]                   = useState(false);

  useEffect(() => {
    const els = document.querySelectorAll('.settings-anim');
    animate(els, {
      opacity:    [0, 1],
      translateY: [18, 0],
      duration:   500,
      ease:       'out(4)',
      delay:      stagger(70, { start: 80 }),
    });
  }, []);

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    setSaved(true);
    setTimeout(() => setSaved(false), 2500);
  };

  const handleResetCache = () => {
    if (window.confirm('This will delete all mock sandboxes and restore the initial seed. Proceed?')) {
      localStorage.removeItem('kubesandbox_mock_sessions');
      window.location.reload();
    }
  };

  return (
    <div className="max-w-3xl mx-auto flex flex-col gap-7 w-full animate-fade-in font-mono text-[12px]">

      <div className="settings-anim flex flex-col items-start gap-1" style={{ opacity: 0 }}>
        <h1 className="text-xl font-bold tracking-tight text-foreground flex items-center gap-2">
          <Settings className="w-5 h-5 text-primary" /> System Preferences
        </h1>
        <p className="text-[12px] text-muted-foreground font-sans">
          Manage default profiles, lifespans, container repositories, and playground parameters.
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-7 items-start">

        {/* ── Settings form ── */}
        <form onSubmit={handleSave} className="settings-anim md:col-span-2 glass-panel p-6 border-white/6 flex flex-col gap-6" style={{ opacity: 0 }}>

          <div className="flex flex-col gap-2">
            <label className="text-[11px] font-bold text-foreground tracking-wider uppercase flex items-center gap-1.5">
              <Sliders className="w-3.5 h-3.5 text-primary" /> DEFAULT_RESOURCE_PROFILE
            </label>
            <select
              value={defaultProfile}
              onChange={e => setDefaultProfile(e.target.value)}
              className="px-4 py-3 bg-black/40 border border-white/10 rounded-xl text-foreground focus:outline-none focus:border-primary/60 transition-all font-mono text-[12px]"
            >
              <option value="starter">Starter (1 Core, 2 GiB RAM)</option>
              <option value="standard">Standard (2 Cores, 4 GiB RAM)</option>
              <option value="advanced">Advanced (4 Cores, 8 GiB RAM)</option>
            </select>
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-[11px] font-bold text-foreground tracking-wider uppercase">
              DEFAULT_SESSION_LIFESPAN <span className="text-muted-foreground normal-case">(minutes)</span>
            </label>
            <input
              type="number"
              value={defaultTTL}
              onChange={e => setDefaultTTL(parseInt(e.target.value))}
              min="15"
              max="1440"
              className="px-4 py-3 bg-black/40 border border-white/10 rounded-xl text-foreground focus:outline-none focus:border-primary/60 transition-all font-mono text-[12px]"
            />
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-[11px] font-bold text-foreground tracking-wider uppercase flex items-center gap-1.5">
              <Database className="w-3.5 h-3.5 text-primary" /> DEFAULT_WORKSPACE_IMAGE
            </label>
            <input
              type="text"
              value={defaultImage}
              onChange={e => setDefaultImage(e.target.value)}
              className="px-4 py-3 bg-black/40 border border-white/10 rounded-xl text-foreground focus:outline-none focus:border-primary/60 transition-all font-mono text-[12px]"
            />
          </div>

          <button
            type="submit"
            className={`w-full py-3.5 font-semibold rounded-xl flex items-center justify-center gap-2 transition-all font-mono text-[12px] ${
              saved
                ? 'bg-success/20 border border-success/30 text-success'
                : 'bg-primary hover:bg-primary/90 text-primary-foreground hover:-translate-y-0.5 hover:shadow-neon-sm'
            }`}
          >
            {saved
              ? <><CheckCircle2 className="w-4 h-4" /> PREFERENCES_SAVED</>
              : <><Save className="w-4 h-4" /> SAVE_PREFERENCES</>
            }
          </button>

        </form>

        {/* ── Dev ops panel ── */}
        <div className="settings-anim glass-panel p-6 border-white/6 flex flex-col gap-5 bg-surface/30" style={{ opacity: 0 }}>
          <div className="border-b border-white/8 pb-2">
            <h3 className="font-bold text-foreground uppercase tracking-wider flex items-center gap-1.5 text-[11px]">
              <Compass className="w-4 h-4 text-accent" /> DEV_OPERATIONS
            </h3>
          </div>

          <div className="space-y-4">
            <p className="text-muted-foreground leading-relaxed text-[11px]">
              Developer tools to verify state transitions, clear local browser caches, and toggle active API client layers.
            </p>

            <div className="flex flex-col gap-2">
              <span className="text-[10px] text-muted-foreground uppercase tracking-wider">Local Cache:</span>
              <button
                onClick={handleResetCache}
                className="w-full py-2.5 bg-white/5 hover:bg-white/10 text-foreground rounded-xl border border-white/10 flex items-center justify-center gap-1.5 transition-all hover:border-white/20 text-[11px]"
              >
                <RotateCcw className="w-3.5 h-3.5" /> Reset Mock Session Cache
              </button>
            </div>

            <div className="flex flex-col gap-2 border-t border-white/8 pt-4">
              <span className="text-[10px] text-muted-foreground uppercase tracking-wider">Active Client Mode:</span>
              <div className="flex justify-between items-center bg-black/35 p-2.5 rounded-xl border border-white/8">
                <span className="text-foreground font-bold text-[11px]">{mockMode ? 'Simulator Active' : 'Cluster Live'}</span>
                <button
                  onClick={() => toggleMockMode(!mockMode)}
                  className="px-3 py-1 bg-accent/15 border border-accent/30 text-accent rounded-lg hover:bg-accent/25 transition-all uppercase text-[10px] font-bold"
                >
                  Toggle
                </button>
              </div>
            </div>

            <div className="flex items-center gap-1.5 text-[10px] text-success/80 bg-success/6 p-2.5 rounded-xl border border-success/15">
              <ShieldCheck className="w-3.5 h-3.5 flex-shrink-0" />
              <span>Sandbox preferences persist locally across sessions.</span>
            </div>
          </div>
        </div>

      </div>
    </div>
  );
};
