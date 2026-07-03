# KubeSandbox — Hot Warm-Pool: Implemented Design

**Status:** implemented (2026-07-02) — closes the brief in [`hot-pool-implementation-brief.md`](../history/hot-pool-implementation-brief.md)
**Audience:** Jeremy + future maintainers
**Related:** [`provisioning-latency-approach.md`](../history/provisioning-latency-approach.md) · [`pause-resume-spike.md`](../history/pause-resume-spike.md) · [`backend-architecture.md`](./backend-architecture.md)

---

> **TL;DR:** The backend now maintains a pool of **hot, unclaimed sandboxes**
> (default **2**, Helm-configurable) provisioned in the background through the
> existing composition. `POST /api/sessions` no longer builds anything: it
> **assigns** an already-Ready pool member by mutating its claim (owner, pool
> label, `spec.expiresAt = now + ttl`) under optimistic concurrency — a
> metadata-only change, measured sub-second against prod-k3s. The
> one-sandbox-per-user guarantee moved from the owner-derived claim name to an
> atomically-created **per-owner marker ConfigMap**. When the pool is empty the
> request **queues** (202 + SSE progress on `/api/queue/events`); it is never
> put on a synchronous cold build. Profiles are **removed** (single sandbox
> type, per sign-off). The vcluster control-plane CPU limit was raised to a
> burstable **200m request / 2000m limit** (the old flat 200m limit inflated
> boot ~6×, docs/history/pause-resume-spike.md).

---

## 1. Decisions taken (with Jeremy, 2026-07-02)

| Decision | Value |
|---|---|
| Control-plane CPU shape | request **200m**, limit **2000m** (burstable; spike-tested) |
| Default warm target | **2** (docs/history/provisioning-latency-approach.md: drain probability negligible at ~5/hour) |
| Concurrent ceiling | **60** (warm + live, from sign-off) |
| Max warm age | **24 h** — older unclaimed members are recycled, never handed out |
| Profiles | **Removed entirely** (XRD, models, API). Single uniform shape: 500m/512Mi shell pod |

All of these are Helm values (`pool.*`, `vcluster.controlPlane.*` in
`kubesandbox-backend/values.yaml`) → env vars (`POOL_*`) → `config.Config`.

## 2. How the pieces fit

```
                    POST /api/sessions
                          │
                 ┌────────▼─────────┐   AlreadyExists → 409 (one per user)
                 │ marker ConfigMap │   (atomic create, name = sbxowner-{hash(owner)})
                 └────────┬─────────┘
                          │ created
                 ┌────────▼─────────┐   none Ready → roll back marker,
                 │ claim pool member│   202 queued + /api/queue/events (SSE)
                 │ (CAS on rv,      │
                 │  oldest first)   │
                 └────────┬─────────┘
                          │ success: owner/tenant set, pool=claimed,
                          │ spec.expiresAt = now+ttl  → 201 + URL
                          ▼
              PoolManager (background, watch-driven + 30s resync)
              • refills to targetWarm within maxTotal ceiling
              • admits queue head as members become Ready
              • recycles members older than maxWarmAge
              • trims overshoot (youngest, not-Ready first)
              • GCs orphaned markers (crash between marker & claim)
```

### Warm members

`KubeSandboxSession` claims named `s-pool-<rand>` with empty
`ownerRef`/`tenantRef` and label `kubesandbox.com/pool: available`. They
provision through the **unchanged** composition path (namespace, quota,
vcluster, netpol, shell pod, service, HTTPRoute, ReferenceGrant — one vcluster
per namespace, as required). A member is assignable only when
`status.workspaceReady == true`, not deleting, and younger than maxWarmAge.

Verified live: the deployed XRD accepts empty owner/tenant on create and
accepts owner mutation afterwards (no immutability constraints), and the
composition re-patches the owner annotations onto the session namespace on
reconcile without churning the vcluster/pod (see §4).

### Assignment (Phase C)

`SessionService.Assign` in `backend/internal/kubernetes/sessions.go`:
marker create → legacy-holder guard (`List(owner)`) → CAS-update the oldest
Ready member (`Update` with the listed `resourceVersion`; `Conflict` → next
member). TTL starts at hand-over: `spec.expiresAt = now + ttl`. The claim's
`spec.ownerRef` is what `/authz` reads (direct claim GET), so ownership is
visible to the gateway immediately, before any Crossplane reconcile.

### One-per-user (Phase D)

The old guarantee (claim name = `s-{sha256(owner)[:16]}`) is impossible for
pre-named pool members. Replacement: a **ConfigMap marker**
`sbxowner-{sha256(owner)[:16]}` in the backend namespace, created *before*
claiming a member. Concurrent duplicates fail `AlreadyExists` at the API
server — same atomicity as before, different object. Marker lifecycle:

- released on `DELETE /api/sessions/{id}` and on TTL reap;
- rolled back when assignment finds no member (so queueing works);
- orphans (crash between marker create and member claim) GC'd by the pool
  manager after a 2-minute grace, unless the owner holds a claim or is queued.

Concurrency is unit-tested with a resourceVersion-enforcing fake (the stock
fake client ignores rv — tests would pass racy code silently otherwise):
8 concurrent creates for one user → exactly 1 success; 4 users racing over 4
members → 4 distinct members.

### Queue + freshness (Phase E)

Empty pool → `202 {status: queued, position}`; progress via
`GET /api/queue/events` (SSE: `queued` position updates, then terminal
`assigned` with the session, or `error`). The pool manager admits the queue
head as members become Ready (FIFO). The queue is in-memory per replica
(backend runs 1 replica); a restart just means the user re-POSTs. Correctness
never depends on the queue — only the marker + CAS enforce invariants.
Freshness: `maxWarmAgeHours` (24h) — stale members are recycled by the pool
manager and skipped by assignment.

### TTL (Phase A)

`spec.expiresAt` was added to the XRD; the backend sets it at assignment. The
composition round-trips it onto the session namespace annotation
`kubesandbox.com/expires-at` and back into `status.expiresAt`. The TTL sweep
(`cleanup.go`) prefers `spec.expiresAt` → `status.expiresAt` →
`creationTimestamp + ttlMinutes` (legacy), **skips available pool members**
(their clock hasn't started; the pool manager owns their lifecycle), and
releases the owner's marker after reaping.

## 3. Cold-path measurement (the open gate in docs/history/provisioning-latency-approach.md §6)

Measured 2026-07-02 on prod-k3s, one cold provision of a warm-style claim
(`s-pool-t01` in throwaway namespace `kubesandbox-hotpool-test`), deployed
(pre-change, 200m-limited) composition. Timestamps are cluster-clock.

| Milestone | clock | t+ |
|---|---|---|
| Claim created | 17:48:41 | 0 s |
| Composite + ALL 8 composed resources created (incl. shell pod object) | 17:48:44 | 3 s |
| vcluster pod created / Helm release `deployed` | 17:48:50 | 9 s |
| kubeconfig Secret published (vcluster booted @200m) | 17:52:58 | 4 m 17 s |
| Shell pod Running | 17:54:58 | 6 m 17 s |
| Claim `status.workspaceReady: true` | 17:58:14 | **9 m 33 s** |

**Where the production "10+ minutes" goes** — three terms, none of them the
orchestration itself (Crossplane fan-out + Helm install take ~10 s total):

1. **vcluster boot under the 200m CPU limit: ~4 m 10 s.** Fixed in this
   change (burstable 200m/2000m); the spike showed ~77 s at 2000m.
2. **kubelet mount-retry backoff: exactly 2 m 00 s.** The shell pod is
   created at t+3 s and immediately starts failing to mount the
   not-yet-existing kubeconfig Secret; the kubelet backs off up to 2 min
   between retries. The Secret landed 4 s after a failed attempt
   (`FailedMount` at 17:52:54, Secret at 17:52:58), so the pod sat idle until
   the next retry at 17:54:54+. Residual after the CPU fix: ~1–2 min worst
   case. Affects refill speed only (never user-facing with the pool); a
   future optimization could gate shell-pod creation on the Secret.
3. **provider-kubernetes observation lag: 3 m 16 s (up to 10 m).** The
   shell-pod `Object` had `watch: false` — the provider only re-observes the
   pod on its poll interval, so `workspaceReady` (what the pool manager and
   the old polling frontend key on) lagged the pod's actual Running state.
   Fixed in this change: `watch: true` on the shell-pod Object.

**Refill estimate after fixes:** ~10 s orchestration + ~80 s vcluster boot +
≤2 min mount backoff + near-zero observation lag ≈ **2–4 minutes** per member.
At ~5 arrivals/hour against target 2, drain probability stays negligible
(docs/history/provisioning-latency-approach.md §4).

**Post-deploy addendum (2026-07-02, first production fill).** The first
create after deploy (3:04 PM) was queued and admitted at 3:12 — the user
raced the pool's very first fill. Member #1's pod was Running in 2.5 min
(CPU fix + `watch: true` both confirmed live), but its claim only flipped
`workspaceReady` at creation + 9 m 32 s: Crossplane core's realtime watch
circuit for the composite type wasn't established yet after the XRD/
composition update (XR condition `Responsive: WatchCircuitClosed` appeared
only at 19:12:23), so XRs advanced on core's slow poll (~9.5 min observed —
the same constant as the pre-fix measurement in §3, which was
poll-dominated too). Once the circuit closed, the replacement member went
created → Ready in **90 seconds**, confirming the ~1.5 min steady-state
refill. Mitigation for the degraded-to-poll window (Crossplane restarts /
XRD changes): set `--poll-interval=1m` on Crossplane core (crossplane-helm
app values, outside this repo).

## 4. Live verification summary (2026-07-02, prod-k3s)

All testing in throwaway namespace `kubesandbox-hotpool-test`; everything
created was deleted (cascade verified: 8 composed resources, session
namespace, session HTTPRoute in `kubesandbox` ns all gone).

- **Warm claim under current schema:** the deployed XRD accepted a claim with
  empty `ownerRef`/`tenantRef` (required-but-empty), pool label included.
- **Owner mutation accepted:** `spec.ownerRef/tenantRef` updated `"" →
  test-user-sub-hash` on the live claim — no XRD/webhook rejection. The
  mutation is a single API-server write (plus one marker create in the real
  path): **sub-second, orders of magnitude inside the 15 s ceiling**. The
  claim read that `/authz` performs returns the new owner immediately.
- **Composition re-patch without churn:** within seconds of the mutation the
  session namespace annotations showed the new owner/tenant, while the
  vcluster pod and shell pod kept their UIDs, IPs and 0 restarts — assignment
  does not disturb composed resources.
- **Authz:** verified at the service level (unit tests: owner → allow,
  non-owner/unknown/empty-owner → deny; unclaimed pool members unreachable
  through every owner-scoped path). A full gateway-level 200/403 smoke test
  needs a real Authentik session cookie and should be repeated once the new
  backend image is deployed (see §6 deploy checklist).
- **Observed orphan (pre-existing, untouched):** an HTTPRoute named `shell`
  (17 h old, `session-route=true` label) sits in the `kubesandbox` namespace —
  looks like a leftover from the pre-fix route-name collision era. Worth
  deleting manually.

## 5. What changed, by file

**Backend (Go)** — `internal/kubernetes/sessions.go` (Assign, CreateWarm,
markers, pool helpers; legacy Create kept for `pool.enabled=false`),
`pool.go` (PoolManager, new), `queue.go` (AssignQueue, new), `cleanup.go`
(expiry preference, pool skip, marker release), `models/session.go` (profiles
removed, QueueStatus), `config/config.go` (`POOL_*`), `api/router.go`
(`/api/queue`, `/api/queue/events`), `api/handlers/sessions.go` (assign/queue
flow), `api/handlers/sse.go` (queue SSE), `cmd/server/main.go` (pool manager
startup).

**Chart** — composition: burstable control-plane CPU
(`vcluster.controlPlane.cpuRequest/cpuLimit`), profile label removed,
`expiresAt` round-trip patches; XRD: profile removed, owner/tenant optional
(default `""`), `spec.expiresAt` added; `values.yaml`: `pool.*`, `vcluster.*`;
`deployment.yaml`: `POOL_*` env; `clusterrole.yaml` (a namespaced Role):
ConfigMap verbs for markers.

**Tests** — `assign_test.go`, `pool_test.go` (new), `cleanup_test.go`
(extended). `go test ./...` and `-race` on the concurrency tests pass.

**Frontend (SPA)** — adapted to the new API contract: `profile` removed from
schemas/request/UI (`ProfilePicker` deleted; the create dialog's one knob is
the lifetime, with the uniform spec stated up front); `POST /sessions` handled
as a discriminated result (`201 created` / `202 queued`); new queue plumbing
(`api.getQueueStatus`, `streamQueueEvents` fetch-stream SSE, `useQueueStatus`
/ `useQueueWatcher` hooks) with polling fallback; the dashboard renders a
live "you're #N in line" card (the position is the determinate progress
indicator per design-principles §1) that swaps to the sandbox card on
hand-over, survives page reloads (mount-time `GET /api/queue`), and surfaces
a terminal queue error inline. Copy updated everywhere the old
minutes-of-provisioning reality leaked through (landing hero, create dialog,
detail-page banner). `tsc --noEmit` and `vite build` pass.

## 6. Operational notes & known limits

- **Deploy checklist:** (1) build/push the backend image; (2) ship the chart
  (XRD + composition + RBAC + env changes together); (3) watch the pool
  manager provision 2 warm members and reach `workspaceReady`; (4) smoke-test
  through the gateway: create → 201 with URL, second create → 409, non-owner
  `/s/{id}` → 403, delete → member replaced; (5) live-test one TTL expiry
  (also closes G3) and confirm the marker is released.
- **Existing claims keep working** (`profile` is pruned on read; TTL falls
  back to creation+ttl). Existing users without markers are handled by the
  legacy-holder guard in Assign.
- **Multi-replica:** refill could briefly overshoot with >1 replica (each
  reconciles independently); the trim step self-heals. The queue is
  per-replica. Correctness (marker + CAS) is replica-safe. If the backend ever
  scales out, add leader election for the pool manager.
- **Custom `workspaceImage`/`starterLabRef` on create are ignored** on the
  pool path (members are pre-provisioned and uniform). The XRD fields remain
  for the legacy path.
- **Pre-existing display bug (not fixed here):** `status.sessionNamespace`
  reports the Crossplane Object wrapper name (`…-namespace` suffix), not the
  actual namespace name. Cosmetic; the URL/id derivation doesn't use it.
- **Init-container CPU** (~30–50 s of every vcluster boot at 100m) is the next
  refill lever (docs/history/pause-resume-spike.md §4); not in scope here.

## 7. Pool sizing & the control-plane capacity ceiling (2026-07-02 incident)

**TL;DR:** the binding constraint on this cluster is **control-plane CPU, not
worker capacity**. Each warm member is a full vcluster whose API server is a
persistent client of the host etcd/apiserver, and the control plane is three
**2-vCPU** k3s server/etcd nodes (**~6 vCPU total** for etcd + apiserver +
scheduler + controller-manager). Raising `pool.targetWarm` from 2 to 50
(`maxTotal` 60→100) took the whole cluster down. Even the recovered steady
state of **10 warm members keeps the control plane saturated and flapping**.
Keep `targetWarm` small (single digits) until the control plane is scaled up.

### What happened

`pool.targetWarm: 50` was set in the values override. The pool manager did
exactly what it is designed to do — refill to target within `maxTotal` — and
began provisioning ~50 vclusters. Two properties turned that into an outage:

1. **No concurrency cap on refill.** The manager refills "to target within
   `maxTotal`" but nothing limits *how many boot simultaneously*. At target 2
   this never mattered; at target 50 it launches ~50 concurrent vcluster boots.
2. **Boot is the CPU-hungry phase, and the control-plane CPU shape is
   burstable 200m/2000m** (§1, raised deliberately in this change to speed
   boot). ~50 API servers each bursting toward 2000m at once demanded roughly
   an order of magnitude more CPU than the cluster has.

Observed failure cascade (prod-k3s):

- All three control-plane nodes pinned at **97–100% CPU** (allocatable is
  `cpu: "2"` each — confirmed on the Node objects).
- Control-plane nodes **flapped `NodeNotReady`** in repeated waves (kubelet
  can't heartbeat when the k3s server process is CPU-starved); the apiserver
  intermittently returned `apiserver not ready` / `connection refused`.
- **Crossplane providers melted:** floods of `CannotConnectToProvider`,
  `CannotCreateExternalResource`, `CannotObserveExternalResource`, and
  `ProviderConfigUsage … already exists` as ~50 compositions reconciled
  against a starved control plane.
- Boot chain broke: `FailedMount` for `vcluster-config` / `vcluster-kubeconfig`
  ("secret not found"), `ClusterIPNotAllocated`, `FailedScheduling`,
  `ProvisioningFailed`. Members never reached Ready.
- Collateral damage across the homelab: argocd, immich, cloudflared, otel,
  metallb, mimir, prometheus, etc. threw `Unhealthy` / `NodeNotReady` because
  they share the same control plane.

### Why the control plane is the ceiling (and workers are not)

On k3s, **etcd and the apiserver run inside the k3s *server* process on the
control-plane nodes** — not as scheduled pods with resource requests. So a
"pod requests vs allocatable" analysis *understates* the problem: the killer
load (watches, lists, and etcd writes from N vcluster control planes) never
appears as a pod request, and it lands entirely on the 6-vCPU control plane.
Every warm vcluster is its own mini-apiserver continuously syncing against the
host — the cost scales with **member count**, and it is paid on the control
plane regardless of how idle the member is. Workers only carry the vcluster
syncer + shell pods (~220m CPU / ~300Mi each when idle) and had ample headroom
(19–52% CPU) throughout.

### Post-recovery measurement (10 idle warm members)

After the control-plane VMs were restarted (which cleared all prior state —
verified: composite count == namespace count, no orphaned `XKubeSandboxSession`
or stuck-Terminating namespaces), the pool refilled to a stable **10 members,
all `SYNCED=True / READY=True`, `pool=available`**. That is a *clean* steady
state, yet the control plane was still unhealthy:

- Control-plane nodes flapped `NodeNotReady` in recent waves; CP-hosted system
  pods (metrics-server, coredns, local-path-provisioner) had restarted.
- `metrics-server` scrapes the **workers** fine but **cannot scrape any of the
  three CP kubelets** (they read `<unknown>` in `nodes top`) — i.e. the CP
  kubelets are too starved to answer, which is real distress, not a metrics
  bug.

Conclusion: **10 warm vcluster control planes, on top of the existing homelab
workload, already saturate the 6-vCPU control plane.** The realistic ceiling
for *idle* warm members on the current hardware is well under 10; the ceiling
for *active* sessions (which add in-vcluster workloads and much higher API
churn) is lower still — likely low single digits to low-teens, not 50.

### Guidance

- **Keep `pool.targetWarm` in the low single digits** on the current
  3× 2-vCPU control plane. The design default (2, with a 60 concurrent ceiling)
  reflects the real capacity; treat that as the safe envelope, not a starting
  point to scale up from without hardware changes.
- **Do not size the pool from worker headroom.** Workers being half-idle does
  not mean there is room — the constraint is control-plane CPU / etcd.
- **To actually raise the ceiling (highest leverage first):**
  1. **Bigger control-plane nodes** (2 → 4/8 vCPU). Single highest-impact
     change; the limit is etcd/apiserver, so this is what moves it.
  2. **Cheaper per-sandbox isolation** — k3s/k0s-in-a-pod, or namespace-only
     isolation — so each session is not a full extra apiserver against host
     etcd.
  3. Only then does worker capacity matter.

### Follow-up work (not yet implemented)

- **Add a concurrency cap on pool refill** — provision at most *N* members at a
  time (e.g. 2–3), not "all the way to target at once." A burstable 2000m
  control-plane CPU limit and a large target are fundamentally unsafe together
  without this throttle; it is the direct fix for the stampede in §7.
- **Consider a hard, hardware-aware upper bound** on `targetWarm`/`maxTotal`
  (or a guard that refuses to fill beyond a control-plane-CPU budget) so a
  values override cannot request a target the control plane cannot sustain.
- **Recovery runbook:** if a large target has wedged the control plane, lower
  `targetWarm`, then delete a batch of warm members **by their
  `KubeSandboxSession` claims** (not the namespaces — the namespace is a
  Crossplane-composed resource and will be recreated; deleting the claim
  cascades to all composed resources). Once the apiserver is responsive, the
  pool manager's trim step takes the rest down to target on its own.
