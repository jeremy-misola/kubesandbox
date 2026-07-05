# KubeSandbox Documentation

Docs are split into two groups: **reference** docs describe the system as it is
today and are kept current with the code; **history** docs are point-in-time
records (plans, spikes, briefs, handoffs) kept for context — they show how the
system got here and are not maintained against the current code.

## Reference — current, maintained

| Doc | What it covers |
|---|---|
| [`reference/backend-architecture.md`](./reference/backend-architecture.md) | The system end-to-end: control loops, the claim data model, hot warm-pool, provisioning, terminal, isolation, lifecycle, scaling. Start here. |
| [`reference/auth-design.md`](./reference/auth-design.md) | Authentication & authorization: `/api` JWT bearer, backend-owned OIDC/PKCE + ext-authz for `/s/{id}`, Authentik specifics, login-CSRF nonce binding. |
| [`reference/frontend-architecture.md`](./reference/frontend-architecture.md) | The SPA: routes, data/auth layer, SSE, the create→assign-or-queue flow, terminal hand-off, runtime config, build & deploy. |
| [`reference/hot-pool-design.md`](./reference/hot-pool-design.md) | The as-built hot warm-pool: warm members, assignment, one-per-user markers, queue, TTL, cold-path measurements, pool sizing & the control-plane ceiling. |
| [`reference/observability-architecture.md`](./reference/observability-architecture.md) | Metrics: OpenTelemetry Go SDK instrumentation, OTLP → node-local collector → Mimir → Grafana, the metrics catalog, and the "KubeSandbox — Backend & Pool" dashboard. |

## History — point-in-time records

| Doc | What it was |
|---|---|
| [`history/implementation-plan.md`](./history/implementation-plan.md) | The original phased delivery plan (G1–G8) and locked decisions. Several were later superseded (markers, no profiles) — see its banner. |
| [`history/backend-handoff.md`](./history/backend-handoff.md) | Backend hand-off snapshot at the end of G2 (session ownership authz). |
| [`history/g2-session-auth-spike.md`](./history/g2-session-auth-spike.md) | Spike that found the original one-policy session-auth design couldn't work on Envoy Gateway v1.7.1, leading to the Options A+B redesign. |
| [`history/frontend-implementation-plan.md`](./history/frontend-implementation-plan.md) | Gap-to-done plan for the SPA (now delivered). |
| [`history/provisioning-latency-approach.md`](./history/provisioning-latency-approach.md) | Rationale and options assessment that led to choosing a hot warm-pool over on-demand builds. |
| [`history/pause-resume-spike.md`](./history/pause-resume-spike.md) | Spike rejecting a *paused* pool (resume too slow) and surfacing the vcluster control-plane CPU-limit fix. |
| [`history/hot-pool-implementation-brief.md`](./history/hot-pool-implementation-brief.md) | The agent handoff brief that specified the hot pool; the as-built result is `reference/hot-pool-design.md`. |
| [`history/challenges-feasibility-spike.md`](./history/challenges-feasibility-spike.md) | Feasibility pass on 64 scraped iximiuz-style challenges against the vcluster model: what works today, what needed the NetworkPolicy-sync fix (shipped), and what's architecturally out of reach (real multi-node, static pods, kubeadm upgrade). Also surfaced a Pod Security Standard gap. |
| [`history/platform-limitations-and-challenges-decision.md`](./history/platform-limitations-and-challenges-decision.md) | Capability matrix vs. a real cluster and vs. iximiuz Labs / KodeKloud / Killercoda, and the decision to build challenge content scoped to workload/policy/troubleshooting skills — not cluster-administration. |
| [`history/challenge-seeding-design-note.md`](./history/challenge-seeding-design-note.md) | Why baking challenge state into hot-pool members breaks pool fungibility, and the resolution: seed challenge manifests into the tenant vcluster at assignment time, not at pool-warm time. Not yet implemented. |

## Conventions

- **Reference docs** should be updated when the code changes. Each carries a
  status line at the top.
- **History docs** are generally left as written, with a dated banner noting what
  shipped since. They preserve the decision trail; don't rewrite them to match the
  current code.
