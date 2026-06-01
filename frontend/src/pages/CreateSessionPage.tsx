import React, { useEffect, useRef } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useKubeSandbox } from '../context/KubeSandboxContext';
import {
  ArrowLeft,
  Layers,
  Clock,
  Server,
  HelpCircle,
  FolderLock,
  PlusCircle,
  Cpu,
  HardDrive
} from 'lucide-react';
import { animate, stagger, createTimeline } from 'animejs';

const dns1035Regex = /^[a-z]([-a-z0-9]*[a-z0-9])?$/;

const createSessionSchema = z.object({
  name: z.string()
    .min(3, 'Name must be at least 3 characters')
    .max(30, 'Name must not exceed 30 characters')
    .regex(dns1035Regex, 'Lowercase letters, numbers, dashes only — must start with a letter'),
  tenantRef: z.string().min(2, 'Tenant reference is required'),
  profile: z.enum(['starter', 'standard', 'advanced'] as const),
  ttlMinutes: z.number().min(15).max(1440),
  workspaceImage: z.string().min(2, 'Workspace image is required'),
});

type CreateSessionFormValues = z.infer<typeof createSessionSchema>;

interface CreateSessionPageProps {
  onNavigate: (page: string) => void;
  onSessionCreated: (name: string) => void;
}

export const CreateSessionPage: React.FC<CreateSessionPageProps> = ({ onNavigate, onSessionCreated }) => {
  const { createSession } = useKubeSandbox();
  const formRef    = useRef<HTMLFormElement>(null);
  const previewRef = useRef<HTMLDivElement>(null);

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    formState: { errors, isValid, isSubmitting },
  } = useForm<CreateSessionFormValues>({
    resolver: zodResolver(createSessionSchema),
    mode: 'onChange',
    defaultValues: {
      name: '',
      tenantRef: 'demo-tenant',
      profile: 'standard',
      ttlMinutes: 60,
      workspaceImage: 'ubuntu-dev:latest',
    },
  });

  const selectedProfile = watch('profile');
  const configuredTTL   = watch('ttlMinutes');
  const configuredImage = watch('workspaceImage');
  const enteredName     = watch('name');

  /* Entrance animation */
  useEffect(() => {
    const els = document.querySelectorAll('.form-anim');
    animate(els, {
      opacity:    [0, 1],
      translateY: [20, 0],
      duration:   500,
      ease:       'out(4)',
      delay:      stagger(60, { start: 80 }),
    });
    if (previewRef.current) {
      animate(previewRef.current, {
        opacity:    [0, 1],
        translateX: [24, 0],
        scale:      [0.97, 1],
        duration:   600,
        ease:       'out(4)',
        delay:      300,
      });
    }
  }, []);

  const onSubmit = async (data: CreateSessionFormValues) => {
    try {
      await createSession(data);
      onSessionCreated(data.name);
    } catch (err) {
      console.error(err);
    }
  };

  const getProfileSpecs = (p: string) => ({
    starter:  { cpu: '1 Core',   ram: '2 GiB',  storage: '10 GiB SSD' },
    standard: { cpu: '2 Cores',  ram: '4 GiB',  storage: '20 GiB SSD' },
    advanced: { cpu: '4 Cores',  ram: '8 GiB',  storage: '50 GiB SSD' },
  }[p] ?? { cpu: 'N/A', ram: 'N/A', storage: 'N/A' });

  const formatTTL = (mins: number) => {
    if (mins < 60) return `${mins} minutes`;
    const hrs = Math.floor(mins / 60);
    return `${hrs}h${mins % 60 > 0 ? ` ${mins % 60}m` : ''}`;
  };

  const profiles = [
    { id: 'starter',  color: 'success',   specs: getProfileSpecs('starter') },
    { id: 'standard', color: 'accent',    specs: getProfileSpecs('standard') },
    { id: 'advanced', color: 'secondary', specs: getProfileSpecs('advanced') },
  ];

  const colorVariants: Record<string, { border: string; bg: string; ring: string; dot: string }> = {
    success:   { border: 'border-success/30',   bg: 'bg-success/8',   ring: 'border-success',   dot: 'bg-success'   },
    accent:    { border: 'border-accent/30',    bg: 'bg-accent/8',    ring: 'border-accent',    dot: 'bg-accent'    },
    secondary: { border: 'border-secondary/30', bg: 'bg-secondary/8', ring: 'border-secondary', dot: 'bg-secondary' },
  };

  return (
    <div className="flex flex-col gap-7 w-full animate-fade-in font-mono text-[12px]">

      {/* Back button */}
      <div className="form-anim" style={{ opacity: 0 }}>
        <button
          onClick={() => onNavigate('dashboard')}
          className="px-4 py-2 bg-white/5 hover:bg-white/10 text-foreground rounded-xl border border-white/10 hover:border-white/20 flex items-center gap-1.5 text-[12px] font-semibold transition-all"
        >
          <ArrowLeft className="w-4 h-4" /> /back
        </button>
      </div>

      {/* Page title */}
      <div className="form-anim flex flex-col items-start gap-1" style={{ opacity: 0 }}>
        <h1 className="text-xl font-bold tracking-tight text-foreground flex items-center gap-2">
          <Layers className="w-5 h-5 text-primary" /> Create Sandbox Session
        </h1>
        <p className="text-[12px] text-muted-foreground">Configure specification parameters for your on-demand Kubernetes cluster instance.</p>
      </div>

      {/* Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-7 items-start">

        {/* ── Form ── */}
        <form ref={formRef} onSubmit={handleSubmit(onSubmit)} className="lg:col-span-2 glass-panel p-6 border-white/6 flex flex-col gap-6 relative">

          {/* Name */}
          <div className="form-anim flex flex-col gap-2" style={{ opacity: 0 }}>
            <label className="text-[11px] font-bold text-foreground tracking-wider flex items-center gap-1.5 uppercase">
              SANDBOX_IDENTIFIER
              <span title="DNS-1035 valid: lowercase letters, numbers, dashes — starts with a letter">
                <HelpCircle className="w-3.5 h-3.5 text-muted-foreground cursor-help" />
              </span>
            </label>
            <input
              type="text"
              placeholder="e.g. dev-sandbox"
              {...register('name')}
              className={`px-4 py-3 bg-black/40 border rounded-xl focus:outline-none focus:border-primary/60 transition-all text-foreground placeholder-muted font-mono text-[12px] ${
                errors.name ? 'border-error/50' : 'border-white/10'
              }`}
            />
            {errors.name && (
              <span className="text-[10px] text-error font-medium">{errors.name.message}</span>
            )}
          </div>

          {/* Tenant ref */}
          <div className="form-anim flex flex-col gap-2" style={{ opacity: 0 }}>
            <label className="text-[11px] font-bold text-foreground tracking-wider uppercase">TENANT_REFERENCE</label>
            <input
              type="text"
              readOnly
              {...register('tenantRef')}
              className="px-4 py-3 bg-white/4 border border-white/8 rounded-xl text-muted-foreground font-mono cursor-not-allowed text-[12px]"
            />
          </div>

          {/* Profile selector */}
          <div className="form-anim flex flex-col gap-3" style={{ opacity: 0 }}>
            <label className="text-[11px] font-bold text-foreground tracking-wider uppercase">HARDWARE_PROFILE</label>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              {profiles.map(p => {
                const c       = colorVariants[p.color];
                const active  = selectedProfile === p.id;
                return (
                  <div
                    key={p.id}
                    onClick={() => setValue('profile', p.id as any, { shouldValidate: true })}
                    className={`glass-panel p-4 flex flex-col gap-2.5 cursor-pointer select-none transition-all duration-200 border hover:-translate-y-0.5 ${
                      active ? `${c.border} ${c.bg}` : 'border-white/6 hover:border-white/12'
                    }`}
                  >
                    <div className="flex justify-between items-center">
                      <span className="font-bold text-[11px] uppercase tracking-wider text-foreground capitalize">{p.id}</span>
                      <div className={`w-4 h-4 rounded-full border-2 flex items-center justify-center transition-all ${active ? c.ring : 'border-white/20'}`}>
                        {active && <span className={`w-2 h-2 rounded-full ${c.dot}`} />}
                      </div>
                    </div>
                    <div className="text-[10px] text-muted-foreground flex flex-col gap-0.5">
                      <span className="flex items-center gap-1"><Cpu className="w-3 h-3" /> {p.specs.cpu}</span>
                      <span className="flex items-center gap-1"><Server className="w-3 h-3" /> {p.specs.ram}</span>
                      <span className="flex items-center gap-1"><HardDrive className="w-3 h-3" /> {p.specs.storage}</span>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* TTL slider */}
          <div className="form-anim flex flex-col gap-3" style={{ opacity: 0 }}>
            <div className="flex justify-between items-center">
              <label className="text-[11px] font-bold text-foreground tracking-wider uppercase flex items-center gap-1.5">
                <Clock className="w-3.5 h-3.5" /> SESSION_LIFESPAN
              </label>
              <span className="text-[12px] text-primary font-bold text-neon">{formatTTL(configuredTTL)}</span>
            </div>
            <input
              type="range"
              min="15"
              max="1440"
              step="15"
              value={configuredTTL}
              onChange={e => setValue('ttlMinutes', parseInt(e.target.value), { shouldValidate: true })}
              className="w-full h-2 rounded-full cursor-pointer bg-black/40 border border-white/8"
              style={{ accentColor: 'var(--primary)' }}
            />
            <div className="flex justify-between text-[10px] text-muted-foreground font-mono">
              <span>15m</span>
              <span>1h</span>
              <span>6h</span>
              <span>12h</span>
              <span>24h</span>
            </div>
          </div>

          {/* Workspace image */}
          <div className="form-anim flex flex-col gap-2" style={{ opacity: 0 }}>
            <label className="text-[11px] font-bold text-foreground tracking-wider uppercase flex items-center gap-1.5">
              <Server className="w-3.5 h-3.5" /> CONTAINER_IMAGE
            </label>
            <input
              type="text"
              placeholder="e.g. ubuntu-dev:latest"
              {...register('workspaceImage')}
              className={`px-4 py-3 bg-black/40 border rounded-xl focus:outline-none focus:border-primary/60 transition-all text-foreground placeholder-muted font-mono text-[12px] ${
                errors.workspaceImage ? 'border-error/50' : 'border-white/10'
              }`}
            />
            {errors.workspaceImage && (
              <span className="text-[10px] text-error font-medium">{errors.workspaceImage.message}</span>
            )}
          </div>

          {/* Submit */}
          <button
            type="submit"
            disabled={!isValid || isSubmitting}
            className={`form-anim w-full py-4 rounded-xl font-semibold flex items-center justify-center gap-2 transition-all font-mono text-[13px] ${
              isValid && !isSubmitting
                ? 'bg-primary hover:bg-primary/90 text-primary-foreground hover:-translate-y-0.5 hover:shadow-neon-sm'
                : 'bg-white/5 border border-white/6 text-muted-foreground cursor-not-allowed'
            }`}
            style={{ opacity: 0 }}
          >
            {isSubmitting ? (
              <>
                <svg className="animate-spin h-4 w-4 text-primary-foreground" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
                PROVISIONING_STACK...
              </>
            ) : (
              <><PlusCircle className="w-4 h-4" /> TRIGGER_SANDBOX_CREATION</>
            )}
          </button>

        </form>

        {/* ── Preview Panel ── */}
        <div
          ref={previewRef}
          className="glass-panel p-6 border-white/6 flex flex-col gap-5 sticky top-24 relative overflow-hidden"
          style={{ opacity: 0 }}
        >
          <div className="absolute top-0 right-0 w-32 h-32 pointer-events-none"
            style={{ background: 'radial-gradient(circle at top right, rgba(164,255,79,0.05), transparent 70%)' }}
          />

          <div className="border-b border-white/8 pb-2">
            <h3 className="font-bold text-foreground uppercase tracking-wider flex items-center gap-1.5 text-[11px]">
              <FolderLock className="w-4 h-4 text-primary" /> LIVE_RESOURCE_PREVIEW
            </h3>
          </div>

          <div className="space-y-4">
            <div className="flex flex-col gap-1.5 bg-black/40 p-3 rounded-xl border border-white/8">
              <span className="text-[9px] text-muted-foreground uppercase tracking-wider">Target ID</span>
              <span className="text-foreground font-bold tracking-wide truncate">
                {enteredName ? `kubesandbox-${enteredName}` : <span className="text-muted">PENDING_INPUT</span>}
              </span>
            </div>

            <div className="space-y-2.5">
              {[
                ['Profile',         selectedProfile, 'text-foreground capitalize'],
                ['CPU',             getProfileSpecs(selectedProfile).cpu, 'text-foreground'],
                ['RAM',             getProfileSpecs(selectedProfile).ram, 'text-foreground'],
                ['Storage',         getProfileSpecs(selectedProfile).storage, 'text-foreground'],
                ['Orchestrator',    'Crossplane compose', 'text-accent'],
                ['Active TTL',      formatTTL(configuredTTL), 'text-primary text-neon'],
              ].map(([k, v, cls]) => (
                <div key={k as string} className="flex justify-between items-center text-[11px]">
                  <span className="text-muted-foreground">{k}:</span>
                  <span className={`font-bold ${cls}`}>{v}</span>
                </div>
              ))}
            </div>

            <div className="border-t border-white/8 pt-4 flex flex-col gap-1.5">
              <span className="text-[9px] text-muted-foreground uppercase tracking-wider">Container Image</span>
              <span className="text-foreground/90 font-mono break-all bg-black/30 px-2.5 py-2 rounded-lg border border-white/6 text-[10px]">
                {configuredImage || 'ubuntu-dev:latest'}
              </span>
            </div>
          </div>
        </div>

      </div>
    </div>
  );
};
