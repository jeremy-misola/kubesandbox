# KubeSandbox — Observability & Metrics Architecture

**Status:** implemented (2026-07-03) — Phase 0 (collector → Mimir) shipped; Phases 1–2 (app instrumentation + chart wiring + dashboard JSON) implemented, pending image build/deploy
**Audience:** Jeremy + future maintainers
**Scope:** metrics only (traces/logs already flow to Tempo/Loki). Instrumentation via the **OpenTelemetry Go SDK**, exported OTLP → node-local collector → Mimir → Grafana.
**Related:** [`backend-architecture.md`](./backend-architecture.md) · [`hot-pool-design.md`](./hot-pool-design.md) · [`../history/provisioning-latency-approach.md`](../history/provisioning-latency-approach.md)

---

> **TL;DR:** Instrument the Gin backend with the OpenTelemetry Go SDK. HTTP
> server metrics come for free from `otelgin`; the interesting signals —
> warm-pool state, assignment throughput, queue depth/wait, provisioning
> latency, TTL reaping — are a small set of custom instruments hooked into the
> functions that already exist (`Assign`, `reconcileOnce`, the queue, the TTL
> loop). The app pushes OTLP to the **node-local otel-collector agent**
> (DaemonSet, `4318`), whose `metrics` pipeline now exports to **Mimir** (`prod`
> tenant) via `prometheusremotewrite` — so everything is queryable in Grafana
> against the existing **Mimir** datasource. A single dashboard (six rows) covers
> API load, pool state, claim throughput, queueing, internals, and an infra
> cross-check from kube-state-metrics. The only remaining work is the app-side
> instrumentation itself.

---

## 1. What already exists (verified against prod-k3s, 2026-07-02)

| Component | State | Detail |
|---|---|---|
| **kube-prometheus-stack** | running (`monitoring`) | Prometheus Operator v84.5.0; scrapes via ServiceMonitor/PodMonitor |
| Prometheus selector | `serviceMonitorSelector: release=kube-prometheus-helm`, `serviceMonitorNamespaceSelector: {}` | any-namespace, but the object **must** carry label `release: kube-prometheus-helm` |
| Long-term storage | **Mimir** | Prometheus remote-writes to `http://mimir-helm-nginx.mimir.svc.cluster.local/api/v1/push`, header `X-Scope-OrgID: prod`; local retention 10d; `externalLabels: cluster=prod` |
| **Grafana** | `grafana.jeremymr.dev` (Authentik SSO) | datasource **`Mimir`** → `http://mimir-helm-nginx.mimir.svc.cluster.local/prometheus` (tenant `prod`). Also Loki + Tempo datasources |
| **otel-collector** | DaemonSet agent (`opentelemetry-collector`) | OTLP `4317`/`4318`, Service `otel-collector-helm-opentelemetry-collector`; `k8sattributes` + `batch` processors |
| Collector `traces` pipeline | → Tempo | `otlp/tempo` exporter |
| Collector `logs` pipeline | → Loki | `otlphttp/loki`, `X-Scope-OrgID: 1` |
| Collector `metrics` pipeline | → **Mimir** (+ `debug`) | `prometheusremotewrite/mimir`, `X-Scope-OrgID: prod`; receivers `[otlp, prometheus]` |
| kube-state-metrics / node-exporter | running | already in Mimir — free infra signals for the cross-check row |
| **kubesandbox-backend** | 1 replica (`kubesandbox` ns) | Gin, client-go dynamic. **No metrics library today** |

The backend is a single-replica Gin service. Sandboxes are Crossplane
`KubeSandboxSession` claims; assigned/warm members live as namespaces labelled
`kubesandbox.com/managed=true` (`playground-s-pool-*`).

## 2. Target data flow

```
  kubesandbox-backend (Go, OTel SDK)
        │  OTLP/HTTP (cumulative), 15s periodic reader
        │  → http://$(HOST_IP):4318          (node-local agent, downward API)
        ▼
  otel-collector agent (DaemonSet, per node)
        │  receivers: otlp
        │  processors: k8sattributes, memory_limiter, batch
        │  exporters:  prometheusremotewrite/mimir
        ▼
  Mimir  (tenant: prod)   http://mimir-helm-nginx.mimir.svc/api/v1/push
        ▼
  Grafana  →  datasource "Mimir" (/prometheus, X-Scope-OrgID: prod)
        ▼
  Dashboard: "KubeSandbox — Backend & Pool"
```

Design choices:

- **Push, not scrape.** The OTel SDK path means the app never exposes a
  `/metrics` endpoint; it pushes OTLP to the node-local agent. No ServiceMonitor
  is needed for the app. (A Prometheus `/metrics` + ServiceMonitor remains a
  viable fallback — see §9 — but is not the chosen path.)
- **Node-local agent via downward API.** Send to `$(HOST_IP):4318` so each pod
  talks to the agent on its own node (standard DaemonSet pattern), not through a
  single Service hop.
- **Cumulative temporality.** Prometheus/Mimir require cumulative sums; the Go
  SDK default is cumulative — keep it. Do **not** switch to delta.
- **Consistency with the existing tenant.** Metrics land in the same `prod`
  Mimir tenant and are queried through the same Grafana `Mimir` datasource as
  everything else, so the new dashboard sits alongside the infra dashboards.

## 3. Collector metrics → Mimir (done)

The collector's `metrics` pipeline now has a Mimir exporter, so OTLP metrics are
persisted rather than dropped. The change lives in the **`otel-collector`**
GitOps values (`operators-helm/operators/otel-collector/values/chart/values-prd.yaml`
in `GitOps-Homelab`), rendered by ArgoCD into the
`otel-collector-helm-opentelemetry-collector-agent` ConfigMap — not the app repo.

```yaml
# otel-collector values — config.exporters
exporters:
  prometheusremotewrite/mimir:
    endpoint: http://mimir-helm-nginx.mimir.svc.cluster.local/api/v1/push
    headers:
      X-Scope-OrgID: prod
    external_labels:
      cluster: prod          # match Prometheus' externalLabels for a unified view
    resource_to_telemetry_conversion:
      enabled: true          # keep k8s.* resource attrs as labels where useful

# config.service.pipelines.metrics — only exporters is overridden; the chart's
# default receivers [otlp, prometheus] and processors [k8sattributes,
# memory_limiter, batch] are preserved via deep-merge. "debug" is kept so it
# stays a referenced component (the collector errors on any defined-but-unused
# component).
metrics:
  exporters: [debug, prometheusremotewrite/mimir]
```

Notes:
- `resource_to_telemetry_conversion` + the `k8sattributes` processor mean each
  series carries `k8s_namespace_name`, `k8s_pod_name`, `k8s_node_name`, etc.,
  and a `target_info` series is emitted for the rest.
- With the default `prometheusremotewrite` behaviour, OTel `service.name`
  becomes the Prometheus **`job`** label (→ `job="kubesandbox-backend"`), and
  metric/unit suffixes are added (see §5). Alternative: Mimir also accepts OTLP
  natively at `/otlp/v1/metrics`; `prometheusremotewrite` is chosen for parity
  with the existing remote-write path.
- **Verified** by rendering the chart (v0.159.0) with the patched values: the
  merged metrics pipeline is exactly as above, and the DaemonSet pod template
  carries a `checksum/config` annotation, so an ArgoCD sync rolls the agents to
  pick up the new config. Collector self-metrics (`otelcol_*`) reach Mimir
  immediately; `count({__name__=~"otelcol_.*"})` in Grafana Explore confirms the
  path even before the app is instrumented. Once the app ships,
  `count({__name__=~"kubesandbox_.*"})` confirms app metrics end-to-end.

## 4. Instrumentation in the Go app

### 4.1 Bootstrap (new `internal/telemetry` package)

A single init that builds a `MeterProvider` with an OTLP/HTTP exporter and a
periodic reader, returns a `Meter` + shutdown func. Driven entirely by env so
Helm owns the wiring:

| Env | Value |
|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://$(HOST_IP):4318` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` |
| `OTEL_SERVICE_NAME` | `kubesandbox-backend` |
| `OTEL_RESOURCE_ATTRIBUTES` | `service.namespace=kubesandbox,service.version=<chart appVersion>,deployment.environment=prod` |
| `OTEL_METRIC_EXPORT_INTERVAL` | `15000` (ms) |
| `HOST_IP` | from `status.hostIP` (downward API) |
| `OTEL_SDK_DISABLED` | `true` for local dev / tests (SDK becomes a no-op). NB: the Go SDK does not implement this env itself — `internal/telemetry.Setup` honours it explicitly, and also no-ops when no OTLP endpoint is set |

Packages: `go.opentelemetry.io/otel`, `otel/sdk/metric`,
`otel/exporters/otlp/otlpmetric/otlpmetrichttp`,
`contrib/.../gin-gonic/gin/otelgin`, `contrib/instrumentation/runtime`.

`main.go`: build the provider first, `defer shutdown(ctx)`, then pass the meter
into the service/pool/queue constructors (or expose the instruments from the
`telemetry` package as an injected struct — keep it out of package globals so
tests stay hermetic).

### 4.2 Where each signal hooks in

| File / func | Instrument(s) added |
|---|---|
| `api/router.go` | `otelgin.Middleware("kubesandbox-backend")` → all HTTP server metrics |
| `kubernetes/pool.go` `reconcileOnce` | set pool-state observable gauges from the values it already computes (`fresh`, `notReady`, claimed, `total`); `provisioned_total`, `recycled_total`; observe `reconcile.duration`; detect first-Ready transition → `provision.duration` |
| `kubernetes/assign.go` `Assign` | `assign.attempts_total{result=...}` |
| `api/handlers/sessions.go` `Create` | `sandbox.claimed_total{source="request"}` on success — recorded at the call site, not inside `Assign`, because the queue-admit path calls the same `Assign` and would otherwise be mislabelled |
| `kubernetes/pool.go` (queue admit) | `sandbox.claimed_total{source="queue"}` |
| `kubernetes/queue.go` | `queue.depth` (from `Len()`); `queue.enqueued_total`; `queue.resolved_total{outcome}`; `queue.wait.duration` (enqueue→Resolve — add an `enqueuedAt time.Time` to `waiter`) |
| `kubernetes/cleanup.go` `reconcileOnce` | `sandbox.expired_total` (TTL reaped) |
| `api/handlers/sse.go` | `sse.active_streams` UpDownCounter (session + queue streams) |
| `main.go` | runtime metrics via `runtime.Start(...)` |

Pool-state gauges are best modelled as **asynchronous (observable) gauges**: the
callback reads atomics that `reconcileOnce` writes at the end of each pass — the
level-based reconcile is already the source of truth for these numbers, so
there's no double counting and no drift.

## 5. Metrics catalog

Prometheus names below are the **post-export** names (dots→underscores, counter
`_total`, unit suffixes, histogram `_bucket/_sum/_count`) — copy-paste ready for
PromQL. Every series carries `job="kubesandbox-backend"` plus the k8s resource
labels from the collector.

### 5.1 HTTP (automatic, from `otelgin`)

| Prometheus series | Type | Key labels |
|---|---|---|
| `http_server_request_duration_seconds_{bucket,sum,count}` | histogram (s) | `http_request_method`, `http_route`, `http_response_status_code` |
| `http_server_active_requests` | up/down gauge | `http_request_method` |

`http_route` is the templated path (`/api/sessions/:id`), so cardinality is
bounded by the route table, not by IDs.

> **Version caveat — RESOLVED (2026-07-03).** The pinned contrib version
> (otelgin v0.69.0) emits the stable semconv `http.server.request.duration` in
> **seconds** unconditionally — the legacy `http.server.duration` path is gone
> and no `OTEL_SEMCONV_STABILITY_OPT_IN` is needed. The names in this doc are
> correct as written; still worth a one-time confirmation in Grafana Explore
> after the first deploy.

### 5.2 Custom domain metrics

| Instrument (SDK name) | Prometheus series | Type | Labels | Meaning |
|---|---|---|---|---|
| `kubesandbox.pool.warm.available` | `kubesandbox_pool_warm_available` | gauge | — | Ready, unclaimed members (hand-out ready) |
| `kubesandbox.pool.warm.pending` | `kubesandbox_pool_warm_pending` | gauge | — | provisioned but not yet Ready |
| `kubesandbox.pool.claimed` | `kubesandbox_pool_claimed` | gauge | — | live claimed sessions |
| `kubesandbox.pool.total` | `kubesandbox_pool_total` | gauge | — | all managed claims (vs ceiling) |
| `kubesandbox.pool.target` | `kubesandbox_pool_target` | gauge | — | configured warm target |
| `kubesandbox.pool.capacity.max` | `kubesandbox_pool_capacity_max` | gauge | — | `MaxTotal` ceiling |
| `kubesandbox.sandbox.provisioned` | `kubesandbox_sandbox_provisioned_total` | counter | — | warm members created |
| `kubesandbox.sandbox.claimed` | `kubesandbox_sandbox_claimed_total` | counter | `source=request\|queue` | successful hand-outs |
| `kubesandbox.sandbox.recycled` | `kubesandbox_sandbox_recycled_total` | counter | `reason=stale\|trim` | members deleted by the manager |
| `kubesandbox.sandbox.expired` | `kubesandbox_sandbox_expired_total` | counter | — | TTL-reaped sessions |
| `kubesandbox.sandbox.provision.duration` | `kubesandbox_sandbox_provision_duration_seconds_*` | histogram (s) | — | warm create → `workspaceReady` |
| `kubesandbox.assign.attempts` | `kubesandbox_assign_attempts_total` | counter | `result=success\|pool_empty\|already_exists\|conflict_retry\|error` | assignment outcomes |
| `kubesandbox.marker.orphan_gc` | `kubesandbox_marker_orphan_gc_total` | counter | — | orphaned owner markers reaped |
| `kubesandbox.queue.depth` | `kubesandbox_queue_depth` | gauge | — | waiters in the FIFO |
| `kubesandbox.queue.enqueued` | `kubesandbox_queue_enqueued_total` | counter | — | requests queued (pool empty) |
| `kubesandbox.queue.resolved` | `kubesandbox_queue_resolved_total` | counter | `outcome=assigned\|error` | queue exits |
| `kubesandbox.queue.wait.duration` | `kubesandbox_queue_wait_duration_seconds_*` | histogram (s) | — | enqueue → terminal event |
| `kubesandbox.pool.reconcile.duration` | `kubesandbox_pool_reconcile_duration_seconds_*` | histogram (s) | — | one reconcile pass |
| `kubesandbox.pool.reconcile.errors` | `kubesandbox_pool_reconcile_errors_total` | counter | `stage=reconcile\|provision\|recycle\|trim\|marker_gc` | reconcile failures — `reconcile` is a whole-pass LIST failure, the rest are per-item op failures within a pass |
| `kubesandbox.sse.active_streams` | `kubesandbox_sse_active_streams` | up/down gauge | `kind=session\|queue` | open SSE connections |

**Cardinality rule (important):** never put `ownerRef`/email/session-id on a
metric label — that is unbounded and will blow up Mimir cardinality. Per-user
and per-session investigation belongs in **traces (Tempo)** and **logs (Loki)**,
not metrics. The label sets above are all small and static.

Histogram buckets: use explicit second-scale buckets tuned to the SLOs — e.g.
HTTP `[.005,.01,.025,.05,.1,.25,.5,1,2.5,5]`; provision duration
`[1,2,5,10,20,30,60,120]`; queue wait `[.5,1,2,5,10,30,60,120]`.

## 6. Grafana dashboard — "KubeSandbox — Backend & Pool"

Datasource: **Mimir**. `$job = kubesandbox-backend`. Panels/queries:

**Row 1 — API & load** (answers "api requests / how it handles load")
- Request rate: `sum(rate(http_server_request_duration_seconds_count{job="$job"}[5m]))`
- By route: `sum by (http_route) (rate(http_server_request_duration_seconds_count{job="$job"}[5m]))`
- Error ratio (5xx): `sum(rate(http_server_request_duration_seconds_count{job="$job",http_response_status_code=~"5.."}[5m])) / sum(rate(http_server_request_duration_seconds_count{job="$job"}[5m]))`
- Latency p50/p95/p99: `histogram_quantile(0.95, sum by (le) (rate(http_server_request_duration_seconds_bucket{job="$job"}[5m])))`
- Create outcomes (201 assign / 202 queued / 409 dup): `sum by (http_response_status_code) (rate(http_server_request_duration_seconds_count{job="$job",http_route="/api/sessions",http_request_method="POST"}[5m]))`
- In-flight: `http_server_active_requests{job="$job"}`

**Row 2 — Sandbox pool state** (answers "active sandboxes provisioned & claimed")
- Stats: `kubesandbox_pool_warm_available`, `kubesandbox_pool_warm_pending`, `kubesandbox_pool_claimed`, `kubesandbox_pool_total`
- Stacked timeseries: available / pending / claimed over time
- Capacity headroom: `kubesandbox_pool_total / kubesandbox_pool_capacity_max`
- Warm health (alert cue): `kubesandbox_pool_warm_available < on() kubesandbox_pool_target`

**Row 3 — Provisioning & claim throughput**
- Provisioned/min: `sum(rate(kubesandbox_sandbox_provisioned_total[5m])) * 60`
- Claimed by source: `sum by (source) (rate(kubesandbox_sandbox_claimed_total[5m]))`
- Recycled / expired: `sum by (reason) (rate(kubesandbox_sandbox_recycled_total[5m]))`, `sum(rate(kubesandbox_sandbox_expired_total[5m]))`
- Provision latency p95: `histogram_quantile(0.95, sum by (le) (rate(kubesandbox_sandbox_provision_duration_seconds_bucket[5m])))`

**Row 4 — Queue & backpressure**
- Queue depth: `kubesandbox_queue_depth`
- Wait p50/p95: `histogram_quantile(0.95, sum by (le) (rate(kubesandbox_queue_wait_duration_seconds_bucket[5m])))`
- Pool-empty rate (demand > warm supply): `sum(rate(kubesandbox_assign_attempts_total{result="pool_empty"}[5m]))`
- Enqueue vs assigned: `sum(rate(kubesandbox_queue_enqueued_total[5m]))` vs `sum(rate(kubesandbox_queue_resolved_total{outcome="assigned"}[5m]))`

**Row 5 — Internals**
- Reconcile p95: `histogram_quantile(0.95, sum by (le) (rate(kubesandbox_pool_reconcile_duration_seconds_bucket[5m])))`
- Reconcile errors: `sum(rate(kubesandbox_pool_reconcile_errors_total[5m]))`
- Assign conflict retries (optimistic-concurrency contention): `sum(rate(kubesandbox_assign_attempts_total{result="conflict_retry"}[5m]))`
- SSE streams: `sum by (kind) (kubesandbox_sse_active_streams)`
- Runtime: `go_memory_used_bytes{job="$job"}`, `go_goroutine_count{job="$job"}`
  (the contrib runtime package emits the new `go.*` semconv names; the old
  `process_runtime_go_*` series are gone)

**Row 6 — Infra cross-check** (free, from kube-state-metrics / node-exporter already in Mimir)
- Managed sandbox namespaces (independent truth vs app gauges): `count(kube_namespace_labels{label_kubesandbox_com_managed="true"})`
- Backend CPU: `sum(rate(container_cpu_usage_seconds_total{namespace="kubesandbox",pod=~"kubesandbox-backend-helm-.*",container!=""}[5m]))`
- Backend memory: `sum(container_memory_working_set_bytes{namespace="kubesandbox",pod=~"kubesandbox-backend-helm-.*",container!=""})`
- Backend restarts: `max(kube_pod_container_status_restarts_total{namespace="kubesandbox",pod=~"kubesandbox-backend-helm-.*"})`
- All sandbox pods CPU: `sum(rate(container_cpu_usage_seconds_total{namespace=~"playground-s-pool-.*"}[5m]))`

The Row-2 app gauges vs the Row-6 `kube_namespace` count are deliberately
redundant: a divergence between "what the backend thinks it has" and "what
actually exists in the cluster" is itself a strong signal of a leak or stuck
finalizer.

## 7. Alerting (optional, phase 3)

Ship as a `PrometheusRule` in the `monitoring` stack — it must carry label
`release: kube-prometheus-helm` to be picked up by the operator's `ruleSelector`.
Candidate alerts:

- **BackendDown** — `up{job="kubesandbox-backend"} == 0` for 2m (or absence of
  `kubesandbox_pool_total`).
- **WarmPoolStarved** — `kubesandbox_pool_warm_available == 0 and kubesandbox_pool_target > 0` for 5m.
- **HighErrorRate** — 5xx ratio > 5% for 10m.
- **QueueBacklog** — `kubesandbox_queue_depth > 5` for 10m.
- **SlowProvisioning** — provision p95 > 60s for 15m.
- **CapacityCeiling** — `kubesandbox_pool_total / kubesandbox_pool_capacity_max > 0.9` for 10m.
- **SandboxLeak** — app claimed gauge and `kube_namespace` managed count diverge
  by > N for 15m.

## 8. Helm / GitOps changes (summary, for when this is implemented)

- **otel-collector values** — `prometheusremotewrite/mimir` exporter added to the
  `metrics` pipeline (§3). ✅ **Done** — this was the prerequisite; app metrics
  can now land in Mimir.
- **kubesandbox-backend chart** — ✅ **Done** — OTEL_* env block + `HOST_IP` /
  `POD_NAME` downward API in `deployment.yaml` (incl. `service.instance.id` =
  pod name); `telemetry.*` values in `values.yaml`. Telemetry off
  (`telemetry.enabled: false`) renders `OTEL_SDK_DISABLED=true`. No
  Service/ServiceMonitor changes (push model).
- **Grafana dashboard** — ✅ **JSON done** — lives at
  `kubesandbox-charts/kubesandbox-backend/dashboards/kubesandbox-backend-pool.json`;
  an optional ConfigMap (label `grafana_dashboard: "1"`, gated by
  `telemetry.dashboard.enabled`, default off) provisions it via the sidecar —
  only enable if the sidecar watches all namespaces; otherwise import manually.
- **PrometheusRule** (phase 3) — label `release: kube-prometheus-helm`.

## 9. Decisions & open questions

- **OTel SDK vs Prometheus `/metrics`.** Chosen: OTel SDK (unifies with the
  existing collector, and later lets metrics share resource attributes with the
  Tempo traces / Loki logs, enabling exemplars). Cost: OTel→Prom name
  translation (the collector metrics-pipeline export is already handled, §3). A Prometheus `/metrics` +
  ServiceMonitor (label `release: kube-prometheus-helm`) remains a low-risk
  fallback if the collector path proves fiddly — worth keeping in the back
  pocket.
- **`service.instance.id`.** Set it to the pod name (downward API) before
  scaling the backend past one replica, so per-replica series don't collide.
  Harmless to add now.
- **Provision-latency source.** Approximate (warm-create timestamp → first
  reconcile observing `workspaceReady`) is cheap and lives entirely in
  `reconcileOnce`; a precise figure would read composition/status timestamps.
  Start approximate — see `../history/provisioning-latency-approach.md`.
- **Export interval.** 15s balances freshness against series churn; the Grafana
  panels use `[5m]` windows so this is comfortably oversampled.
- **Exemplars (future).** With OTLP metrics + Tempo traces sharing resource
  attributes, histogram exemplars can deep-link a slow `POST /api/sessions` bar
  straight to its trace. Out of scope for v1 but the SDK choice keeps the door
  open.

## 10. Suggested rollout order

1. **Phase 0 — collector → Mimir:** ✅ **done** — Mimir exporter added to the
   collector metrics pipeline; `otelcol_*` self-metrics confirm the path.
2. **Phase 1 — HTTP + runtime:** ✅ **implemented 2026-07-03** —
   `internal/telemetry` bootstrap, `otelgin` middleware, runtime metrics.
3. **Phase 2 — domain metrics:** ✅ **implemented 2026-07-03** — pool gauges,
   assign/claim/provision/recycle/expire counters, queue depth/wait, reconcile
   histograms; full six-row dashboard JSON. Remaining: build/push the image,
   deploy the chart, verify `count({__name__=~"kubesandbox_.*"})` in Explore,
   import the dashboard.
4. **Phase 3 — alerts & polish:** `PrometheusRule`, exemplars, dashboard
   template variables.

Each phase is independently shippable and adds value on its own.
