import React, { useState, useEffect, useRef } from 'react';
import { useKubeSandbox } from '../context/KubeSandboxContext';
import { ArrowLeft, Terminal, ShieldAlert, ExternalLink, Loader2 } from 'lucide-react';
import { animate } from 'animejs';

interface WorkspacePageProps {
  sessionName: string;
  onNavigate: (page: string) => void;
}

// ── Mock terminal (used only when mockMode is active) ────────────────────────

interface CommandHistory {
  command: string;
  output: string;
  id: number;
}

let cmdId = 0;

const MockTerminal: React.FC<{ sessionName: string; workspacePod: string }> = ({
  sessionName,
  workspacePod,
}) => {
  const [history, setHistory] = useState<CommandHistory[]>([{
    command: 'system-init',
    output:
      `Initializing secure terminal connection to sandbox environment...\nConnected successfully to namespace playground-${sessionName}\nWelcome to KubeSandbox Secure Terminal v1.0.\n\nType "help" to see available commands.`,
    id: ++cmdId,
  }]);
  const [input, setInput] = useState('');
  const terminalEndRef    = useRef<HTMLDivElement>(null);

  useEffect(() => {
    terminalEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [history]);

  const handleCommand = (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim()) return;

    const cmd = input.trim().toLowerCase();
    let out = '';

    if (cmd === 'help') {
      out = `Available commands:
  help                     Show this help block
  clear                    Clear terminal output
  kubectl cluster-info     Retrieve target cluster details
  kubectl get namespaces   List active tenant namespaces
  kubectl get pods         List workloads in default namespace
  helm list                List installed Helm packages`;
    } else if (cmd === 'clear') {
      setHistory([]);
      setInput('');
      return;
    } else if (cmd === 'kubectl cluster-info') {
      out = `Kubernetes control plane is running at https://vcluster-${sessionName}.playground.kubesandbox.com
CoreDNS is running at https://vcluster-${sessionName}.playground.kubesandbox.com/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy

To further debug cluster problems, use 'kubectl cluster-info dump'.`;
    } else if (cmd === 'kubectl get namespaces' || cmd === 'kubectl get ns') {
      out = `NAME              STATUS   AGE
default           Active   5m
kube-system       Active   5m
kube-public       Active   5m
kube-node-lease   Active   5m`;
    } else if (cmd === 'kubectl get pods' || cmd === 'kubectl get po') {
      out = `NAME                                READY   STATUS    RESTARTS   AGE
workspace-dev-pod-6f59b6f84-xklp2   1/1     Running   0          4m20s
postgresql-0                        1/1     Running   0          3m45s`;
    } else if (cmd === 'helm list') {
      out = `NAME        NAMESPACE   REVISION    UPDATED                                 STATUS      CHART               APP VERSION
postgresql  default     1           2026-06-01 00:05:12.384729 -0400 EDT    deployed    postgresql-12.1.0   15.2.0`;
    } else {
      out = `bash: command not found: ${input}\nType "help" to see available options.`;
    }

    setHistory(prev => [...prev, { command: input, output: out, id: ++cmdId }]);
    setInput('');
  };

  return (
    <>
      <div className="flex-1 p-6 overflow-y-auto flex flex-col gap-4 text-[11px] leading-relaxed" style={{ color: '#86efac' }}>
        {history.map(h => (
          <div key={h.id} className="flex flex-col gap-1">
            {h.command !== 'system-init' && (
              <div className="flex gap-2 items-center" style={{ color: '#f0f4fc' }}>
                <span className="text-primary font-bold text-neon">{workspacePod || 'shell'}@kubesandbox:~#</span>
                <span>{h.command}</span>
              </div>
            )}
            <pre className="whitespace-pre-wrap leading-relaxed opacity-85">{h.output}</pre>
          </div>
        ))}
        <div ref={terminalEndRef} />
      </div>

      <form
        onSubmit={handleCommand}
        className="border-t border-white/8 px-6 py-4 flex items-center gap-2"
        style={{ background: 'rgba(4, 6, 14, 0.9)', color: '#86efac' }}
      >
        <span className="text-foreground font-bold select-none">{workspacePod || 'shell'}@kubesandbox:~#</span>
        <input
          type="text"
          value={input}
          onChange={e => setInput(e.target.value)}
          className="flex-1 bg-transparent focus:outline-none placeholder-opacity-30 border-none"
          style={{ color: '#86efac', caretColor: '#a4ff4f' }}
          placeholder="Type 'help' or a kubectl command..."
          autoFocus
        />
      </form>
    </>
  );
};

// ── Provisioning placeholder ─────────────────────────────────────────────────

const ProvisioningState: React.FC<{ message: string }> = ({ message }) => (
  <div className="flex-1 flex flex-col items-center justify-center gap-4 text-[11px]" style={{ color: '#86efac' }}>
    <Loader2 className="w-8 h-8 animate-spin opacity-60" style={{ color: '#22d3ee' }} />
    <div className="flex flex-col items-center gap-1 text-center">
      <span className="text-foreground font-bold tracking-wider uppercase text-[10px]">Workspace provisioning</span>
      <span className="text-muted-foreground max-w-xs">{message || 'Waiting for cluster reconciliation...'}</span>
    </div>
  </div>
);

// ── Main page ────────────────────────────────────────────────────────────────

export const WorkspacePage: React.FC<WorkspacePageProps> = ({ sessionName, onNavigate }) => {
  const { selectedSession, fetchSessionDetail, mockMode } = useKubeSandbox();
  const terminalRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    fetchSessionDetail(sessionName);
  }, [sessionName, fetchSessionDetail]);

  // Poll for readiness every 5 s while the workspace isn't ready yet
  useEffect(() => {
    if (selectedSession?.workspaceReady) return;
    const id = setInterval(() => fetchSessionDetail(sessionName), 5000);
    return () => clearInterval(id);
  }, [selectedSession?.workspaceReady, sessionName, fetchSessionDetail]);

  useEffect(() => {
    if (terminalRef.current) {
      animate(terminalRef.current, {
        opacity:    [0, 1],
        translateY: [24, 0],
        duration:   600,
        ease:       'out(4)',
      });
    }
  }, []);

  if (!selectedSession) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px] gap-4 animate-fade-in font-mono text-[12px]">
        <Loader2 className="w-8 h-8 animate-spin text-accent opacity-60" />
        <div className="flex flex-col items-center gap-1 text-center">
          <span className="text-foreground font-bold tracking-wider uppercase text-[10px]">Loading workspace</span>
          <span className="text-muted-foreground">Fetching session details for <span className="text-primary">{sessionName}</span>...</span>
        </div>
        <button
          onClick={() => onNavigate('dashboard')}
          className="mt-2 px-4 py-2 bg-white/5 hover:bg-white/10 text-foreground rounded-xl border border-white/10 hover:border-white/20 flex items-center gap-1.5 text-[12px] font-semibold transition-all"
        >
          <ArrowLeft className="w-4 h-4" /> Back to dashboard
        </button>
      </div>
    );
  }

  // URL the Crossplane composition routes to:
  // HTTPRoute hostname: {claim-namespace}-{claim-name}.kubesandbox.com
  const ttydUrl = `https://${selectedSession.namespace}-${selectedSession.name}.kubesandbox.com`;
  const isReady = selectedSession.workspaceReady;

  const renderTerminalBody = () => {
    if (!isReady) {
      return <ProvisioningState message={selectedSession.message} />;
    }

    if (mockMode) {
      return (
        <MockTerminal
          sessionName={sessionName}
          workspacePod={selectedSession.workspacePod}
        />
      );
    }

    // Real ttyd iframe — fills the shell chrome
    return (
      <iframe
        src={ttydUrl}
        className="flex-1 w-full border-none"
        title={`KubeSandbox terminal — ${sessionName}`}
        allow="clipboard-read; clipboard-write"
      />
    );
  };

  return (
    <div className="flex flex-col gap-6 w-full animate-fade-in font-mono text-[12px]">

      {/* Nav row */}
      <div className="flex justify-between items-center">
        <button
          onClick={() => onNavigate(`sessions/${sessionName}`)}
          className="px-4 py-2 bg-white/5 hover:bg-white/10 text-foreground rounded-xl border border-white/10 hover:border-white/20 flex items-center gap-1.5 text-[12px] font-semibold transition-all"
        >
          <ArrowLeft className="w-4 h-4" /> /back
        </button>

        <div className="flex items-center gap-4 text-muted-foreground text-[11px]">
          <span>Sandbox: <strong className="text-foreground">{sessionName}</strong></span>
          {isReady && selectedSession.workspacePod && (
            <>
              <span className="text-white/15">|</span>
              <span>Pod: <strong className="text-accent">{selectedSession.workspacePod}</strong></span>
            </>
          )}
          {isReady && !mockMode && (
            <>
              <span className="text-white/15">|</span>
              <a
                href={ttydUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1 hover:text-foreground transition-colors"
              >
                <ExternalLink className="w-3 h-3" /> open in new tab
              </a>
            </>
          )}
        </div>
      </div>

      {/* Terminal shell */}
      <div
        ref={terminalRef}
        className="w-full rounded-2xl border border-white/8 shadow-2xl relative overflow-hidden flex flex-col h-[600px]"
        style={{ background: 'rgba(4, 6, 14, 0.95)', opacity: 0 }}
      >
        {/* Chrome bar */}
        <div
          className="w-full border-b border-white/8 px-4 py-3 flex items-center justify-between select-none flex-shrink-0"
          style={{ background: 'rgba(12, 16, 28, 0.8)' }}
        >
          <div className="flex items-center gap-2">
            <span className="w-3 h-3 rounded-full bg-error/70 hover:bg-error transition-colors cursor-pointer" />
            <span className="w-3 h-3 rounded-full bg-warning/70 hover:bg-warning transition-colors cursor-pointer" />
            <span className="w-3 h-3 rounded-full bg-success/70 hover:bg-success transition-colors cursor-pointer" />
          </div>
          <span className="text-[10px] text-muted-foreground uppercase tracking-widest font-bold flex items-center gap-1.5">
            <Terminal className="w-3.5 h-3.5 text-primary" /> SECURE_SHELL_CONSOLE
          </span>
          <div className="w-14" />
        </div>

        {/* Scanline overlay — sits above everything but doesn't intercept pointer events */}
        <div
          className="absolute inset-0 pointer-events-none z-10"
          style={{
            backgroundImage: 'repeating-linear-gradient(0deg, transparent, transparent 3px, rgba(255,255,255,0.008) 3px, rgba(255,255,255,0.008) 4px)',
            mixBlendMode: 'overlay',
          }}
        />

        {renderTerminalBody()}
      </div>

      {/* Footer note */}
      {mockMode && (
        <div className="flex items-center gap-2 bg-warning/5 border border-warning/15 text-warning/70 rounded-xl p-3 max-w-xl mx-auto text-[11px] font-mono">
          <ShieldAlert className="w-4 h-4 flex-shrink-0" />
          <span>Mock mode active — commands are simulated. Disable in Settings to connect to the real vcluster.</span>
        </div>
      )}
    </div>
  );
};
