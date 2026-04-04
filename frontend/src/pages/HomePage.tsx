import { LinkButton } from '../ui/LinkButton';

const features = [
  {
    icon: 'bolt',
    label: 'Instant',
    value: 'Ready in seconds',
  },
  {
    icon: 'cloud_sync',
    label: 'Ephemeral',
    value: 'Auto-cleanup',
  },
  {
    icon: 'terminal',
    label: 'Full Access',
    value: 'kubectl & API',
  },
];

function HomePage() {
  return (
    <div className="landing-shell">
      <header className="landing-topbar">
        <a className="landing-brand" href="/">
          KubeSandbox
        </a>

        <nav className="landing-nav" aria-label="Primary">
          <a href="#features">Features</a>
          <LinkButton href="/terminal" variant="primary">
            Start Free
          </LinkButton>
        </nav>
      </header>

      <main className="landing-main">
        <section className="landing-hero">
          <h1>Instant Kubernetes<br />Playgrounds</h1>
          <p className="landing-kicker">
            Spin up ephemeral clusters in seconds. No setup, no config.
          </p>
        </section>

        <section className="landing-terminal-preview">
          <div className="terminal-window">
            <div className="terminal-header">
              <span className="terminal-dot" />
              <span className="terminal-dot" />
              <span className="terminal-dot" />
            </div>
            <pre className="terminal-content">
{`$ kubectl get pods -A
NAMESPACE     NAME                        READY   STATUS
kube-system   coredns-78fcdf6894-x        1/1     Running
kube-system   kube-proxy-abc12            1/1     Running
default       nginx-deployment-5c4        1/1     Running

$ kubectl get nodes
NAME       STATUS   ROLES           AGE
sandbox    Ready    control-plane  2m`}
            </pre>
          </div>
        </section>

        <section className="landing-feature-grid" id="features">
          {features.map((feature) => (
            <article className="landing-feature-card" key={feature.label}>
              <span className="material-symbols-outlined landing-feature-icon">
                {feature.icon}
              </span>
              <div className="landing-feature-copy">
                <h2>{feature.value}</h2>
                <p>{feature.label}</p>
              </div>
            </article>
          ))}
        </section>

        <section className="landing-cta">
          <h2>Start experimenting</h2>
          <p className="landing-cta-sub">No credit card required.</p>
          <LinkButton href="/terminal" variant="primary" size="large">
            Launch Playground
          </LinkButton>
        </section>
      </main>

      <footer className="landing-footer">
        <span>© 2024 KubeSandbox</span>
        <nav>
          <a href="#">Status</a>
          <a href="#">Privacy</a>
        </nav>
      </footer>
    </div>
  );
}

export default HomePage;
