import React, { useEffect, useState, useRef } from 'react';
import { useKubeSandbox } from '../context/KubeSandboxContext';
import {
  ArrowLeft,
  Terminal,
  Trash2,
  ExternalLink,
  Download,
  Copy,
  Check,
  Database,
  ChevronDown,
  ChevronUp,
  RefreshCw,
  Play,
} from 'lucide-react';
import { animate, stagger } from 'animejs';

interface SessionDetailPageProps {
  sessionName: string;
  onNavigate: (page: string) => void;
}

interface LogLine {
  timestamp: string;
  message: string;
  type: 'info' | 'success' | 'warn' | 'error';
  id: number;
}

let logIdCounter = 0;

export const SessionDetailPage: React.FC<SessionDetailPageProps> = ({ sessionName, onNavigate }) => {
  const {
    selectedSession,
    fetchSessionDetail,
    deleteSession,
    getKubeconfig,
    loading
  } = useKubeSandbox();

  const [copied, setCopied]           = useState(false);
  const [kubeconfigData, setKubeconfigData] = useState<string | null>(null);
  const [logs, setLogs]               = useState<LogLine[]>([]);
  const [showLogs, setShowLogs]       = useState(true);
  const [activeStep, setActiveStep]   = useState(0);
  const logTerminalEndRef             = useRef<HTMLDivElement>(null);
  const stepsRef                      = useRef<HTMLDivElement>(null);
  const actionsRef                    = useRef<HTMLDivElement>(null);
  const headerRef                     = useRef<HTMLDivElement>(null);

  useEffect(() => {
    fetchSessionDetail(sessionName);
  }, [sessionName, fetchSessionDetail]);

  /* Animate header details on load */
  useEffect(() => {
    if (!headerRef.current || !selectedSession) return;
    animate(headerRef.current.querySelectorAll('.detail-anim'), {
      opacity:    [0, 1],
      translateY: [16, 0],
      duration:   500,
      ease:       'out(4)',
      delay:      stagger(70),
    });
  }, [selectedSession]);

  /* Animate action panel */
  useEffect(() => {
    if (!actionsRef.current || !selectedSession) return;
    animate(actionsRef.current.querySelectorAll('.action-card'), {
      opacity:    [0, 1],
      translateX: [24, 0],
      scale:      [0.97, 1],
      duration:   500,
      ease:       'out(4)',
      delay:      stagger(80, { start: 200 }),
    });
  }, [selectedSession]);

  /* Update step progress */
  useEffect(() => {
    if (!selectedSession) return;
    const msg = (selectedSession.message ?? '').toLowerCase();
    let step = 0;
    if (selectedSession.phase === 'Ready')                                     step = 4;
    else if (msg.includes('network') || msg.includes('routing'))               step = 3;
    else if (msg.includes('workspace') || msg.includes('pod'))                 step = 2;
    else if (msg.includes('vcluster') || msg.includes('control plane'))        step = 1;

    setActiveStep(step);

    setLogs(prev => {
      if (prev.some(l => l.message === selectedSession.message)) return prev;
      const type: LogLine['type'] = selectedSession.phase === 'Error'
        ? 'error'
        : selectedSession.phase === 'Ready'
        ? 'success'
        : 'info';
      return [...prev, { timestamp: new Date().toLocaleTimeString(), message: selectedSession.message ?? 'Waiting for status...', type, id: ++logIdCounter }];
    });
  }, [selectedSession]);

  /* Animate new log lines */
  useEffect(() => {
    if (!logTerminalEndRef.current) return;
    logTerminalEndRef.current.scrollIntoView({ behavior: 'smooth' });
    const lines = document.querySelectorAll('.log-line-new');
    if (lines.length > 0) {
      animate(lines, {
        opacity:    [0, 1],
        translateX: [-8, 0],
        duration:   250,
        ease:       'out(3)',
      });
      lines.forEach(el => el.classList.remove('log-line-new'));
    }
  }, [logs, showLogs]);

  useEffect(() => {
    if (selectedSession?.phase === 'Ready' && !kubeconfigData) {
      getKubeconfig(sessionName).then(res => {
        setKubeconfigData(res.kubeconfig);
      }).catch(console.error);
    }
  }, [selectedSession, kubeconfigData, getKubeconfig, sessionName]);

  const handleCopyKubeconfig = () => {
    if (!kubeconfigData) return;
    navigator.clipboard.writeText(kubeconfigData);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleDownloadKubeconfig = () => {
    if (!kubeconfigData) return;
    const blob = new Blob([kubeconfigData], { type: 'text/yaml' });
    const url = URL.createObjectURL(blob);
    const a = Object.assign(document.createElement('a'), { href: url, download: `kubeconfig-${sessionName}.yaml` });
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const handleDelete = async () => {
    if (window.confirm('Are you sure you want to completely tear down this sandbox? All data will be deleted.')) {
      await deleteSession(sessionName);
      onNavigate('dashboard');
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

  /* ── Loading skeleton ── */
  if (loading && !selectedSession) {
    return (
      <div className="max-w-3xl mx-auto py-12 animate-fade-in">
        <div className="glass-panel p-8 border-white/5 relative overflow-hidden w-full h-72">
          <div className="absolute inset-0 skeleton-shimmer rounded-xl" />
        </div>
      </div>
    );
  }

  /* ── Not Found ── */
  if (!selectedSession) {
    return (
      <div className="max-w-md mx-auto py-12 flex flex-col gap-6 text-center font-mono text-[12px] animate-fade-in">
        <div className="glass-panel p-8 border-white/5 flex flex-col items-center gap-4">
          <h3 className="text-base font-bold text-foreground uppercase tracking-wider">SANDBOX_NOT_FOUND</h3>
          <p className="text-muted-foreground">The specified session is either unavailable or has expired.</p>
          <button
            onClick={() => onNavigate('dashboard')}
            className="px-5 py-2.5 bg-primary text-primary-foreground font-semibold rounded-xl text-[12px] hover:bg-primary/90 transition-all"
          >
            Return to Dashboard
          </button>
        </div>
      </div>
    );
  }

  const steps = [
    { label: 'Tenant Allocating',   desc: 'Setting up sandboxed Kubernetes namespace boundaries' },
    { label: 'Control Plane Deploy', desc: 'Deploying vCluster control pods' },
    { label: 'Workspace Launch',    desc: 'Initializing developer tools container' },
    { label: 'Network Bindings',    desc: 'Routing internal ports via Envoy HTTPRoute policies' },
    { label: 'Operational',         desc: 'Session ready. Console workspace and kubeconfig are live.' },
  ];

  const isReady = selectedSession.phase === 'Ready';

  return (
    <div className="flex flex-col gap-8 w-full animate-fade-in font-mono text-[12px]">

      {/* ── Nav row ── */}
      <div className="flex justify-between items-center">
        <button
          onClick={() => onNavigate('dashboard')}
          className="px-4 py-2 bg-white/5 hover:bg-white/10 text-foreground rounded-xl border border-white/10 hover:border-white/20 flex items-center gap-1.5 text-[12px] font-semibold transition-all"
        >
          <ArrowLeft className="w-4 h-4" /> /sandboxes
        </button>

        <div className="flex items-center gap-2.5">
          <button
            onClick={() => fetchSessionDetail(sessionName)}
            className="p-2.5 bg-white/5 hover:bg-white/10 text-foreground rounded-xl border border-white/10 hover:border-white/20 transition-all"
            title="Sync Details"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={handleDelete}
            className="px-4 py-2.5 bg-error/10 hover:bg-error/20 text-error rounded-xl border border-error/20 flex items-center gap-1.5 text-[12px] transition-all"
          >
            <Trash2 className="w-3.5 h-3.5" /> Tear Down
          </button>
        </div>
      </div>

      {/* ── Main Grid ── */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-7 items-start">

        {/* Left: metadata + stepper + logs */}
        <div className="lg:col-span-2 flex flex-col gap-6">

          {/* Metadata panel */}
          <div ref={headerRef} className="glass-panel p-6 border-white/6 flex flex-col gap-4 relative overflow-hidden">
            <div className="absolute top-0 right-0 w-48 h-48 pointer-events-none"
              style={{ background: 'radial-gradient(circle at top right, rgba(164,255,79,0.04), transparent 65%)' }}
            />

            <div className="detail-anim flex justify-between items-start border-b border-white/8 pb-4" style={{ opacity: 0 }}>
              <div className="flex flex-col gap-1">
                <span className="text-xl font-bold text-foreground tracking-tight flex items-center gap-2">
                  <Terminal className="w-5 h-5 text-primary" /> {selectedSession.name}
                </span>
                <span className="text-[10px] text-muted-foreground">
                  Created: {new Date(selectedSession.createdAt).toLocaleString()}
                </span>
              </div>

              <div className={`border px-3 py-1 rounded-lg text-[10px] uppercase font-bold tracking-wider flex items-center gap-1.5 ${
                isReady ? 'bg-success/10 text-success border-success/25' : 'bg-accent/10 text-accent border-accent/25 animate-pulse'
              }`}>
                <span className={`w-1.5 h-1.5 rounded-full ${isReady ? 'bg-success' : 'bg-accent'}`} />
                {selectedSession.phase}
              </div>
            </div>

            <div className="detail-anim grid grid-cols-2 sm:grid-cols-4 gap-3 text-[10px]" style={{ opacity: 0 }}>
              {[
                { label: 'Lifespan TTL', value: getRemainingTime(selectedSession.expiresAt), cls: 'text-primary text-neon' },
                { label: 'Virtual CPU',  value: selectedSession.resources?.cpu || '2 Cores', cls: 'text-foreground' },
                { label: 'Memory RAM',   value: selectedSession.resources?.memory || '4 GiB', cls: 'text-foreground' },
                { label: 'vCluster',     value: selectedSession.vclusterRelease || 'Pending', cls: 'text-foreground/80 truncate max-w-[80px]' },
              ].map(stat => (
                <div key={stat.label} className="flex flex-col gap-1 bg-white/4 p-2.5 rounded-lg border border-white/6">
                  <span className="text-muted-foreground uppercase tracking-wider text-[9px]">{stat.label}</span>
                  <span className={`font-bold ${stat.cls}`} title={stat.value}>{stat.value}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Stepper */}
          <div ref={stepsRef} className="glass-panel p-6 border-white/6 flex flex-col gap-6">
            <h3 className="text-[10px] font-bold text-foreground tracking-widest uppercase border-b border-white/8 pb-2">
              PROVISIONING_LIFECYCLE
            </h3>

            <div className="relative pl-7 border-l border-white/10 ml-3 flex flex-col gap-7">
              {steps.map((step, idx) => {
                const done    = idx < activeStep || isReady;
                const current = idx === activeStep && !isReady;

                return (
                  <div key={idx} className="relative flex flex-col gap-0.5">
                    {/* Step node */}
                    <div className={`absolute -left-[35px] top-0 w-5 h-5 rounded-full border-2 flex items-center justify-center transition-all duration-500 ${
                      done
                        ? 'bg-primary border-primary'
                        : current
                        ? 'bg-accent/10 border-accent shadow-cyan-sm'
                        : 'bg-surface border-white/20'
                    }`}>
                      {done ? (
                        <Check className="w-3 h-3 text-primary-foreground stroke-[3]" />
                      ) : current ? (
                        <span className="w-2 h-2 rounded-full bg-accent animate-pulse" />
                      ) : null}
                    </div>

                    <span className={`font-bold tracking-tight text-[11px] ${
                      done ? 'text-foreground' : current ? 'text-accent' : 'text-muted'
                    }`}>
                      {idx + 1}. {step.label}
                    </span>
                    <span className="text-[10px] text-muted-foreground leading-relaxed">{step.desc}</span>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Log stream */}
          <div className="glass-panel p-4 border-white/6 flex flex-col gap-3">
            <button
              onClick={() => setShowLogs(!showLogs)}
              className="flex justify-between items-center font-bold text-foreground uppercase tracking-wider font-mono hover:text-primary transition-colors text-left text-[11px]"
            >
              <span className="flex items-center gap-1.5">
                <Terminal className="w-4 h-4 text-primary" /> REALTIME_EVENT_LOG
              </span>
              {showLogs ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </button>

            {showLogs && (
              <div className="rounded-xl border border-white/8 p-4 h-[180px] overflow-y-auto font-mono text-[10px] leading-relaxed flex flex-col gap-1.5 relative"
                style={{ background: 'rgba(4,6,14,0.7)' }}
              >
                {logs.map(log => (
                  <div key={log.id} className="flex gap-2.5 items-start log-line-new">
                    <span className="text-muted-foreground flex-shrink-0 select-none">[{log.timestamp}]</span>
                    <span className={
                      log.type === 'error'   ? 'text-error font-semibold' :
                      log.type === 'success' ? 'text-primary font-semibold text-neon' :
                      log.type === 'warn'    ? 'text-warning' : 'text-foreground/70'
                    }>
                      {log.message}
                    </span>
                  </div>
                ))}

                {selectedSession.phase === 'Provisioning' && (
                  <div className="flex gap-2.5 items-center text-accent/60 animate-pulse mt-0.5">
                    <span className="text-muted-foreground select-none">[{new Date().toLocaleTimeString()}]</span>
                    <span>Waiting for cluster reconciliation events...</span>
                  </div>
                )}

                <div ref={logTerminalEndRef} />
              </div>
            )}
          </div>
        </div>

        {/* Right: Actions */}
        <div ref={actionsRef} className="flex flex-col gap-5">

          {/* Workspace launch */}
          <div className="action-card glass-panel-accent p-6 border-accent/15 flex flex-col gap-4 relative overflow-hidden" style={{ opacity: 0 }}>
            <div className="absolute top-0 right-0 w-28 h-28 pointer-events-none"
              style={{ background: 'radial-gradient(circle at top right, rgba(34,211,238,0.06), transparent 70%)' }}
            />

            <div className="border-b border-white/8 pb-2">
              <h3 className="font-bold text-foreground uppercase tracking-wider flex items-center gap-1.5 text-[11px]">
                <Play className="w-4 h-4 text-accent" /> DEV_WORKSPACE
              </h3>
            </div>

            <p className="text-muted-foreground leading-relaxed text-[11px]">
              Once provisioned, click below to open your secure web-terminal. Connects via Envoy HTTPRoute rules.
            </p>

            <button
              disabled={!isReady}
              onClick={() => onNavigate(`sessions/${sessionName}/workspace`)}
              className={`w-full py-3.5 rounded-xl font-bold flex items-center justify-center gap-2 transition-all text-[11px] ${
                isReady
                  ? 'bg-accent hover:bg-accent/90 text-accent-foreground shadow-md shadow-accent/10 hover:-translate-y-0.5 hover:shadow-cyan-sm'
                  : 'bg-white/5 border border-white/8 text-muted-foreground cursor-not-allowed'
              }`}
            >
              <ExternalLink className="w-4 h-4" /> LAUNCH_WORKSPACE_SHELL
            </button>
          </div>

          {/* Kubeconfig */}
          <div className="action-card glass-panel p-6 border-white/6 flex flex-col gap-4 relative overflow-hidden" style={{ opacity: 0 }}>
            <div className="border-b border-white/8 pb-2">
              <h3 className="font-bold text-foreground uppercase tracking-wider flex items-center gap-1.5 text-[11px]">
                <Database className="w-4 h-4 text-primary" /> KUBECONFIG_CREDENTIALS
              </h3>
            </div>

            <p className="text-muted-foreground leading-relaxed text-[11px]">
              Retrieve admin cluster credentials for your local CLI, Lens, or k9s.
            </p>

            <div className="flex flex-col gap-3 mt-1">
              <button
                disabled={!isReady || !kubeconfigData}
                onClick={handleCopyKubeconfig}
                className={`w-full py-3 rounded-xl font-semibold flex items-center justify-center gap-2 border transition-all text-[11px] ${
                  isReady && kubeconfigData
                    ? 'bg-white/5 border-white/12 hover:bg-white/10 text-foreground'
                    : 'bg-white/5 border-white/6 text-muted-foreground cursor-not-allowed'
                }`}
              >
                {copied
                  ? <><Check className="w-4 h-4 text-primary" /> COPIED_TO_CLIPBOARD</>
                  : <><Copy className="w-4 h-4" /> COPY_KUBECONFIG_DATA</>
                }
              </button>

              <button
                disabled={!isReady || !kubeconfigData}
                onClick={handleDownloadKubeconfig}
                className={`w-full py-3 rounded-xl font-semibold flex items-center justify-center gap-2 border transition-all text-[11px] ${
                  isReady && kubeconfigData
                    ? 'bg-primary/10 border-primary/20 hover:bg-primary/20 text-primary hover:shadow-neon-sm'
                    : 'bg-white/5 border-white/6 text-muted-foreground cursor-not-allowed'
                }`}
              >
                <Download className="w-4 h-4" /> DOWNLOAD_CONFIG_FILE
              </button>
            </div>
          </div>

        </div>
      </div>
    </div>
  );
};
