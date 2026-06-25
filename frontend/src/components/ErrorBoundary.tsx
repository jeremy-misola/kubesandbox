import React from 'react';
import { AlertOctagon, RefreshCw } from 'lucide-react';

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

interface ErrorBoundaryProps {
  children: React.ReactNode;
  fallbackPage?: string;
}

export class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('[KubeSandbox] Uncaught render error:', error, info);
  }

  render() {
    if (!this.state.hasError) return this.props.children;

    return (
      <div className="max-w-md mx-auto py-12 flex flex-col gap-6 text-center animate-fade-in font-mono text-[12px]">
        <div className="glass-panel p-8 border-error/15 flex flex-col items-center gap-5">
          <div className="w-14 h-14 rounded-2xl bg-error/10 border border-error/25 flex items-center justify-center text-error">
            <AlertOctagon className="w-7 h-7" />
          </div>
          <div className="flex flex-col gap-2">
            <h3 className="text-base font-bold text-foreground tracking-wide uppercase">RENDER_ERROR</h3>
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              {this.state.error?.message ?? 'An unexpected error occurred rendering this page.'}
            </p>
          </div>
          <button
            onClick={() => {
              this.setState({ hasError: false, error: null });
              window.location.hash = '';
            }}
            className="px-5 py-2.5 bg-white/5 hover:bg-white/10 text-foreground rounded-xl border border-white/10 hover:border-white/20 flex items-center gap-2 text-[12px] transition-all"
          >
            <RefreshCw className="w-3.5 h-3.5" /> Try again
          </button>
        </div>
      </div>
    );
  }
}
