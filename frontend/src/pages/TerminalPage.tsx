import { useAuth } from '../auth';
import { UserInfo } from '../components/UserInfo';

const pods = [
  ['api-gateway-v1-90f7', '1/1', 'Running', '0', '12m', 'success'],
  ['auth-service-77b2', '1/1', 'Running', '0', '12m', 'success'],
  ['payment-worker-b4g1', '1/1', 'Running', '2', '8m', 'success'],
  ['search-index-rebuild-x2', '0/1', 'Pending', '0', '14s', 'warning'],
  ['stats-aggregator-d821', '1/1', 'Running', '0', '12m', 'success'],
  ['ui-frontend-deployment-7e', '0/1', 'CrashLoop', '14', '1m', 'error'],
];

function TerminalPage() {
  const { user, isAuthenticated } = useAuth();
  return (
    <div className="obsidian-page">
      <header className="obsidian-topbar">
        <div className="obsidian-topbar__left">
          <a className="obsidian-wordmark" href="/terminal">
            <span />
            OBSIDIAN_ENGINE
          </a>

          <nav className="obsidian-topnav" aria-label="Primary">
            <a className="is-active" href="/terminal">
              Cluster Status
            </a>
            <a href="/">Metric Stream</a>
            <a href="/">API Explorer</a>
          </nav>
        </div>

        <div className="obsidian-topbar__right">
          <div className="obsidian-timer">
            <span className="material-symbols-outlined">timer</span>
            <span>02:45:12 REMAINING</span>
          </div>
          <div className="obsidian-clock">LOC: 09:59:00</div>
        </div>
      </header>

      <div className="obsidian-layout">
        <aside className="obsidian-sidebar">
          <div className="obsidian-cluster">
            <div className="obsidian-cluster__dot" />
            <div>
              <h1>K8S-ALPHA</h1>
              <p>us-east-1 (prod)</p>
            </div>
          </div>

          <nav className="obsidian-sidenav" aria-label="Workspace">
            <a className="is-active" href="/terminal">
              <span className="material-symbols-outlined">rocket_launch</span>
              Provision
            </a>
            <a href="/">
              <span className="material-symbols-outlined">restart_alt</span>
              Reset
            </a>
            <a href="/">
              <span className="material-symbols-outlined">terminal</span>
              Kubeconfig
            </a>
          </nav>

          <div className="obsidian-sidebar__footer">
            <a className="obsidian-settings" href="/">
              <span className="material-symbols-outlined">settings</span>
              Settings
            </a>

            {isAuthenticated && user ? (
              <div className="obsidian-user">
                <div className="obsidian-user__icon">
                  <span className="material-symbols-outlined">shield_person</span>
                </div>
                <div>
                  <strong>{user.name ?? user.email ?? 'User'}</strong>
                  <p>{user.groups?.length ? user.groups.join(', ') : 'Authenticated'}</p>
                </div>
              </div>
            ) : (
              <div className="obsidian-user">
                <div className="obsidian-user__icon">
                  <span className="material-symbols-outlined">shield_person</span>
                </div>
                <div>
                  <strong>Cluster Admin</strong>
                  <p>S-rank permissions</p>
                </div>
              </div>
            )}
          </div>
        </aside>

        <main className="obsidian-main">
          <section className="obsidian-statusbar">
            <div className="obsidian-statusbar__group">
              <div className="obsidian-stat">
                <p>Node Integrity</p>
                <div>
                  <strong className="ok">3/3</strong>
                  <span className="pill pill--green">Healthy</span>
                </div>
              </div>

              <div className="obsidian-stat">
                <p>Pod Lifecycle</p>
                <div>
                  <strong>7/8</strong>
                  <span className="pill pill--blue">Syncing</span>
                </div>
              </div>
            </div>

            <div className="obsidian-meters">
              <div className="obsidian-meter">
                <div>
                  <span>Memory</span>
                  <strong>42%</strong>
                </div>
                <div className="meter">
                  <span className="meter__fill meter__fill--blue" style={{ width: '42%' }} />
                </div>
              </div>

              <div className="obsidian-meter">
                <div>
                  <span>CPU Load</span>
                  <strong>18%</strong>
                </div>
                <div className="meter">
                  <span className="meter__fill meter__fill--green" style={{ width: '18%' }} />
                </div>
              </div>
            </div>
          </section>

          <section className="obsidian-terminal">
            <div className="obsidian-terminal__toolbar">
              <div className="traffic-lights" aria-hidden="true">
                <span />
                <span />
                <span />
              </div>

              <div className="obsidian-toolbar-meta">
                <div className="obsidian-live">
                  <span />
                  <span>SSH: OBS-3949-ALPHA</span>
                </div>
                <span className="material-symbols-outlined">fullscreen</span>
              </div>
            </div>

            <div className="obsidian-terminal__body">
              <div className="obsidian-command-chip">
                <span className="material-symbols-outlined">terminal</span>
                <span>k8s-alpha kubectl get pods</span>
              </div>

              <div className="obsidian-table">
                <div className="obsidian-table__head">
                  <span>Pod Identifier</span>
                  <span>Ready</span>
                  <span>Current Status</span>
                  <span>Restarts</span>
                  <span>Uptime</span>
                </div>

                {pods.map((pod) => (
                  <div className="obsidian-table__row" key={pod[0]}>
                    <span>{pod[0]}</span>
                    <span>{pod[1]}</span>
                    <span>
                      <em className={`pill pill--${pod[5]}`}>{pod[2]}</em>
                    </span>
                    <span>{pod[3]}</span>
                    <span>{pod[4]}</span>
                  </div>
                ))}
              </div>

              <div className="obsidian-terminal__prompt">
                <span className="obsidian-terminal__prompt-user">k8s-alpha</span>
                <span className="obsidian-terminal__cursor" />
              </div>

              <button className="obsidian-launch-button" type="button">
                <span className="material-symbols-outlined">prompt_suggestion</span>
              </button>
            </div>
          </section>
        </main>
      </div>
    </div>
  );
}

export default TerminalPage;
