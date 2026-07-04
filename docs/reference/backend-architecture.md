# KubeSandbox — Architecture

**Status:** living design (updated 2026-07-02 — G1–G5 live on prod-k3s; hot warm-pool shipped)
**Audience:** platform engineers building/operating KubeSandbox
**Related:** [`auth-design.md`](./auth-design.md) · [`hot-pool-design.md`](./hot-pool-design.md) · [`provisioning-latency-approach.md`](../history/provisioning-latency-approach.md)

---

## 1. What it is

KubeSandbox is a self-serve platform for **ephemeral Kubernetes sandboxes**. A
user signs in, creates a session, and gets a browser terminal wired to a private,
throwaway [vcluster](https://www.vcluster.com/) preloaded with `kubectl`. Every
session has a TTL and is garbage-collected.

The platform is **declarative end-to-end**: every sandbox is one Crossplane claim
(`KubeSandboxSession`) and a Composition provisions everything else. What makes it
**fast** is that the backend does not build a sandbox on request — it keeps a
**hot pool** of identical, pre-provisioned sandboxes and *assigns* one on create,
which is a metadata-only change. The browser terminal is **ttyd** running in the
session pod, exposed through the shared Envoy Gateway on a per-session path
(`kubesandbox.com/s/{id}`).

### Design goals

- **Self-serve & instant** — create returns an already-`Ready` sandbox in seconds (assignment, not a cold build). Cold vcluster boot happens in the background, off the request path.
- **Strong isolation** — one tenant can never reach another's cluster or pods.
- **Ephemeral by default** — every session has a TTL and is reaped safely.
- **Declarative provisioning** — the backend writes/mutates one claim; Crossplane does the rest.
- **Uniform & fungible** — every sandbox has the same shape, so any warm member can be handed to any user.
- **Browser-only access** — users get a terminal via ttyd, never a kubeconfig or a direct route to the vcluster API.

---

## 2. Components (as built)

| Component | Where | Role | Status |
|---|---|---|---|
| **Frontend SPA** | `frontend/`, chart `kubesandbox-charts/frontend` | Sign-in (Authentik OIDC), sandbox dashboard, queue view, terminal hand-off. | **Built and live (G5).** |
| **Backend service** | `backend/`, chart `kubesandbox-charts/kubesandbox-backend` | Assigns/lists/deletes claims (client-go dynamic client), runs the hot pool + assignment queue + TTL loop, enforces one-sandbox-per-user via markers, owns the full OIDC/PKCE flow for session access, answers ext-authz ownership checks. | **Built and live (G1–G3).** |
| **Hot pool manager** | `backend/internal/kubernetes/pool.go` | Keeps N unclaimed sandboxes `Ready`, admits queued users, recycles/trims/refills, GCs orphaned markers. | **Built and live.** |
| **Assignment queue** | `backend/internal/kubernetes/queue.go` | In-memory FIFO for when the pool is momentarily empty; SSE progress. | **Built and live.** |
| **TTL controller** | `backend/internal/kubernetes/cleanup.go` | In-process loop that deletes expired claims (`spec.expiresAt` first). | **Built and live (G3).** |
| **XRD / claim** | `kubesandbox-session-xrd.yaml` (chart) | `platform.kubesandbox.com` `KubeSandboxSession` API. `ownerRef`/`tenantRef` optional (empty for warm members). | **Built.** |
| **Composition** | `kubesandbox-session-composition.yaml` (chart) | Fans one claim into the per-session resource set. | **Built.** |
| **Authentik OIDC clients** | Crossplane Terraform Workspaces | Backend + frontend OIDC clients. | **Both built + live.** |
| **Envoy Gateway** | shared `kubesandbox` Gateway | TLS termination, routing, JWT policy (`/api`), ext-authz (`/s/{id}`). | **Built and live.** |
| **Session SecurityPolicy** | `securitypolicy-session.yaml` (chart) | Ext-authz to backend `/authz` for all session routes (`session-route: "true"` label). | **Built and live.** |
| **OIDC callback route** | `httproute-callback.yaml` (chart) | Unauthenticated route for `/oauth2/callback`. | **Built and live.** |
| **Backend NetworkPolicy** | `networkpolicy.yaml` (chart) | Restricts backend ingress to `envoy-gateway-system` (anti-spoofing). | **Built and live.** |
| **Sweep CronJob** | `sweep-cronjob.yaml` (chart) | Backstop: deletes orphaned session namespaces older than `maxAgeHours` with no surviving claim. | **Built, `dryRun: true` on prod.** |

---

## 3. The two control loops

A **synchronous API plane** (backend) and an **asynchronous control plane**
(Crossplane + the cluster). The API never blocks on infrastructure — and with the
hot pool, it never even blocks on a cold build, because sandboxes are provisioned
ahead of demand.

```
                         ┌────────────────────────────────────────────────────┐
  Browser                │              Envoy Gateway (kubesandbox)             │
 ┌─────────┐   TLS       │   kubesandbox.com            → frontend SPA          │
 │  SPA +  │◀───────────▶│   kubesandbox.com/api        → backend /api (JWT)    │
 │  ttyd   │             │   kubesandbox.com/s/{id}     → session ttyd          │
 │         │             │   kubesandbox.com/oauth2/callback → backend (login)  │
 └─────────┘             └───────────┬──────────┬───────────────────────────────┘
                                     │          │ ext-authz (every /s/{id} req)
                                     │          ▼
                                     │  backend /authz: cookie → owns? → 200/302/403
                                     │
                                     │ POST /api/sessions → ASSIGN a warm member
                                     ▼
        ┌───────────────── Management Cluster (Kubernetes API) ─────────────────┐
        │  Pool manager keeps N warm KubeSandboxSession claims (no owner)        │
        │        │ assignment stamps owner + expiresAt onto a warm claim         │
        │        ▼                                                               │
        │  KubeSandboxSession claim  ──▶ XKubeSandboxSession (composite)         │
        │        │                                                               │
        │        ▼  Crossplane Composition (patch-and-transform + auto-ready)    │
        │   provisions per session:                                             │
        │     • Namespace  {claim-ns}-{claim-name}                              │
        │     • ResourceQuota                                                    │
        │     • vcluster (Helm Release — burstable control plane)               │
        │     • NetworkPolicy (allow envoy-gateway-system + kube-system DNS)    │
        │     • Pod  "shell"  (ttyd, jurassicjey/ttyd-k8s:1.0.1, kubeconfig mnt)│
        │     • Service "shell"  :80→8080                                       │
        │     • HTTPRoute  kubesandbox.com/s/{id} in kubesandbox namespace      │
        │     • ReferenceGrant in session namespace (cross-ns backendRef)       │
        └───────────────────────────────────────────────────────────────────────┘
```

**Why split it.** `POST /sessions` returns as soon as a warm member is assigned
(or the request is queued). The heavy lifting (vcluster boot, pod scheduling)
happens ahead of time in the pool, out of band. The frontend streams
`GET /sessions/{id}/events` to render live status. Claims are the source of truth,
Crossplane reconciliation is level-triggered and self-heals.

---

## 4. Data model — the `KubeSandboxSession` claim

API group `platform.kubesandbox.com/v1alpha1`, claim kind `KubeSandboxSession`
(composite `XKubeSandboxSession`). This is the **source of truth** for a sandbox.

### Spec

| Field | Type | Notes |
|---|---|---|
| `tenantRef` | string | Logical tenant. **Empty on unclaimed warm members**; set to the owner subject at assignment. `tenantRef == ownerRef == sub`. |
| `ownerRef` | string | Authentik `sub` claim (hashed uid — a 64-char hex hash, **not** email or UUID). Empty until assignment. |
| `expiresAt` | string (date-time) | Absolute expiry, **set by the backend at assignment** (TTL starts at hand-over). Authoritative for cleanup. |
| `ttlMinutes` | int | 15–1440, default 60. |
| `workspaceImage` | string | Default `jurassicjey/ttyd-k8s:1.0.1`. Single source: the chart's `.Values.workspaceImage`, which also feeds the XRD default and the backend's `WORKSPACE_IMAGE` env. Must bundle ttyd with base-path support. |
| `starterLabRef` | string | Optional starter-lab/template id (field exists, unused). |
| `resources.cpu` / `.memory` | string | Shell pod request/limit. **Uniform for every sandbox**: `500m` / `512Mi`. |

> **No profiles.** Earlier revisions had `starter`/`standard`/`advanced` profiles.
> They were removed when the hot pool shipped: warm members are provisioned before
> an owner (and therefore any profile choice) exists, so every sandbox must be
> identical to remain fungible. The only knob left on create is `ttlMinutes`.

### Status

`phase`, `message`, `expiresAt`, `sessionNamespace`, `vclusterRelease`,
`workspacePod`, `workspaceReady`. The backend reads these to surface progress and
the session URL; readiness is computed by the `function-auto-ready` pipeline step.

> **`expiresAt` round-trip.** The backend sets `spec.expiresAt` at assignment; the
> Composition stamps it as a namespace annotation and reads it back into
> `status.expiresAt`. Cleanup prefers `spec.expiresAt`, then `status.expiresAt`,
> then `creationTimestamp + ttlMinutes` (legacy fallback).

### Labels & markers the backend relies on

- `app.kubernetes.io/managed-by: kubesandbox-backend` — every claim and marker the backend owns.
- `kubesandbox.com/pool: available|claimed` — warm (unclaimed) vs assigned pool members. Absent on legacy direct-create claims.
- `kubesandbox.com/owner: <sha256(owner)[:32]>` — DNS-safe owner hash for ownership queries (owner may contain `@`).
- Per-owner marker `ConfigMap` `sbxowner-<sha256(owner)[:16]>` — the atomic one-sandbox-per-user reservation (see §10).

---

## 5. The hot warm-pool & assignment

The pool is the reason create is fast. Full design in
[`hot-pool-design.md`](./hot-pool-design.md); the essentials:

- **Warm members** (`warm.go`) are claims created with no owner, labelled
  `pool=available`, on a uniform shape. The pool manager keeps `POOL_TARGET_WARM`
  of them `Ready` within a `POOL_MAX_TOTAL` concurrent ceiling.
- **Pool manager** (`pool.go`) is watch-driven with a periodic resync backstop.
  Each level-based pass: (1) admits queued users onto `Ready` members, (2) recycles
  members older than `POOL_MAX_WARM_AGE`, (3) trims overshoot, (4) refills to
  target within the ceiling, (5) GCs orphaned owner markers.
- **Assignment** (`assign.go`) is the request path for `POST /sessions`. It creates
  the owner marker first (atomic one-per-user), then claims a `Ready`, fresh
  member by stamping `ownerRef`/`tenantRef`/`ttlMinutes`/`expiresAt` and flipping
  `pool` to `claimed` with an optimistic-concurrency `Update`. A conflict means
  another assignment won that member; the loser retries the next one. If no member
  can be claimed, the marker is rolled back and `ErrPoolEmpty` is returned so the
  handler queues the request.
- **Queue** (`queue.go`) is an in-memory FIFO. `POST /sessions` on an empty pool
  returns `202` with a position; the caller follows `GET /api/queue/events` (SSE)
  and is admitted by the pool manager as members become `Ready`. The queue is
  per-replica state; losing it on restart is safe because the one-per-user
  invariant lives in the marker, not the queue.

**Legacy path.** With `POOL_ENABLED=false`, `POST /sessions` calls `Create`
directly: a claim named `s-{sha256(owner)[:16]}` built cold on request. This is a
fallback only; the pool path is the default.

---

## 6. Provisioning (what the Composition does)

The `patch-and-transform` pipeline produces the per-session resource set. Most
land in the per-session namespace `{claim-ns}-{claim-name}`; the HTTPRoute and
ReferenceGrant require special placement for the auth model:

1. **Namespace** — the session sandbox. Owner/tenant stored as **annotations**
   (not labels — values may contain `@`); empty for warm members and re-patched
   after assignment.
2. **ResourceQuota** — caps CPU/memory/pods/services for the session.
3. **vcluster `Release`** — the user's private cluster. The control plane is
   **burstable** (modest request, generous CPU limit) because it is CPU-hungry
   only during boot; a flat low limit previously inflated cold boot ~6x and
   throttled pool refill (see [`pause-resume-spike.md`](../history/pause-resume-spike.md)).
4. **NetworkPolicy** — allows ingress from `envoy-gateway-system` (gateway → ttyd)
   and egress to the vcluster service + `kube-system` DNS; otherwise isolated.
5. **Shell Pod (ttyd)** — copies the vcluster kubeconfig to `/tmp`, sets
   `KUBECONFIG`, and runs `ttyd` under `BASE_PATH=/s/{ns}-{name}`. Hardened:
   `automountServiceAccountToken: false`, `enableServiceLinks: false`,
   `readOnlyRootFilesystem: true`, non-root, only the vcluster kubeconfig mounted.
   The pod is **watched** (not polled) by provider-kubernetes so
   `status.workspaceReady` tracks the pod's actual state promptly — a poll-only
   observe once lagged readiness by up to ~10 min (see [`hot-pool-design.md`](./hot-pool-design.md) §3).
   The image (`ttyd/Dockerfile`, `jurassicjey/ttyd-k8s:1.0.1`) bakes in a
   pinned `kubecolor` binary with `kubectl`/`k` aliased to it, plus a
   KubeSandbox welcome banner. Both live in `/etc/kubesandbox/shellrc.sh` and
   load via an explicit `bash --rcfile /etc/kubesandbox/shellrc.sh` — the pod
   runs the shell as UID 1000 with `HOME=/tmp` and a read-only root, so
   `~/.bashrc` / `/root/.bashrc` are **not** usable (an earlier `sh`-based
   command sourced nothing, so the banner and aliases silently never loaded).
   Both are purely cosmetic, no effect on the hardening above (baked into image
   layers at build time, not written at runtime).
6. **Service `shell`** — ClusterIP, `:80 → 8080`.
7. **HTTPRoute** — placed in the **`kubesandbox` namespace** (not the session
   namespace). Host `kubesandbox.com`, path prefix `/s/{ns}-{name}`, cross-namespace
   backendRef to the session `shell` Service. The `kubesandbox.com/session-route: "true"`
   label lets the shared SecurityPolicy attach via `targetSelectors`.
8. **ReferenceGrant** — in the **session namespace**. Permits the `kubesandbox`-ns
   HTTPRoute to reference the per-session `shell` Service.

> **Routing is path-based**: a single `kubesandbox.com` TLS cert covers all
> sessions — no wildcard required. The HTTPRoute forwards the `/s/{id}` prefix
> unchanged; ttyd serves under `BASE_PATH` (no Envoy URL rewrite needed).

Children are owned by the composite so deletion cascades cleanly.

---

## 7. The browser terminal (ttyd, not a backend proxy)

The terminal is **ttyd inside the session pod**, exposed through Envoy with
per-session ownership enforcement:

```
Browser ──TLS, WS──▶ Envoy Gateway
                         │
                         ├─▶ SecurityPolicy (ext-authz → backend /authz)
                         │     cookie missing/invalid → 302 to Authentik (PKCE login)
                         │     cookie valid, owner → 200 (allow)
                         │     cookie valid, not owner / unknown → 403
                         │
                         └─▶ HTTPRoute kubesandbox.com/s/{id} (in kubesandbox ns)
                               │
                               └─▶ shell Service (cross-ns via ReferenceGrant)
                                     └─▶ ttyd :8080 ──▶ bash + kubectl (private vcluster)
```

The user never holds cluster credentials and never gets a route to the vcluster
API — only to ttyd, which is `kubectl`-scoped to *their* vcluster via the mounted
kubeconfig.

---

## 8. Session authentication & authorization (G2, live)

The backend owns the **complete OIDC/PKCE login flow** for session access. There
is no edge OIDC filter — the `SecurityPolicy` does ext-authz only, and the backend
`/authz` handler drives the auth. Full detail in
[`auth-design.md`](./auth-design.md); in short:

1. Browser hits `kubesandbox.com/s/{id}` — Envoy calls `GET /authz`.
2. No session cookie → backend generates PKCE verifier + S256 challenge, signs a
   state token (HMAC-SHA256, 5-min TTL, carries `code_verifier` + `originalURL` +
   a **nonce**), sets a matching short-lived nonce cookie, and returns `302` →
   Authentik.
3. User logs in; Authentik redirects to `/oauth2/callback?code=&state=`.
4. Backend verifies the state token **and** the nonce cookie (OAuth login-CSRF
   protection), exchanges the code over server-to-server TLS, parses the ID token
   (`sub`/`email`/`name`), and sets an `HttpOnly Secure SameSite=Lax` HMAC-signed
   session cookie, then redirects to the original URL.
5. Browser retries `/s/{id}` — the cookie is now present → ownership check → 200.

**Ownership check:** resolve `{id}` from the forwarded URI, look up the claim,
compare `ownerRef == cookie.sub`. Owner → 200; non-owner / unknown / malformed →
403 (no existence leak); backend error → 503 (fail closed).

`/api` uses a different surface: an Envoy JWT `SecurityPolicy` validates Authentik
bearer tokens and injects `X-User-*` headers the backend trusts.

---

## 9. Isolation & security

- **vcluster** — each user is `cluster-admin` only *inside* their throwaway cluster; the management-plane API is not exposed to users.
- **Per-session Namespace** with **ResourceQuota** (CPU/memory/pods/services capped).
- **Session NetworkPolicy** — only `envoy-gateway-system` may reach the shell pod on 8080; egress limited to the vcluster service + kube-system DNS. Sessions are isolated from each other.
- **Backend NetworkPolicy** — only `envoy-gateway-system` may reach the backend on 8080. Prevents in-cluster pods from spoofing `X-User-*` identity headers on `/api` and `/authz`.
- **ttyd pod hardening** — non-root, `readOnlyRootFilesystem: true`, no host SA token, no service links; writable area confined to `/tmp` emptyDir. Only credential is the mounted vcluster kubeconfig.
- **Session ownership** — the shared `SecurityPolicy` (ext-authz → backend `/authz`) enforces that only the session owner can reach their ttyd. Negative test (user B → 403) verified live.
- **Backend /api** — JWT `SecurityPolicy` enforces Authentik bearer tokens; rejects missing/invalid tokens with `401`. `claimToHeaders` injects `X-User-Id`/`X-User-Email`/`X-User-Name`/`X-User-Groups`.

---

## 10. Lifecycle & garbage collection

Sessions are **ephemeral and disposable**. Layers:

1. **TTL controller** (`cleanup.go`) — runs every `TTL_CLEANUP_INTERVAL` (default
   1 min), deleting claims past expiry. Expiry prefers `spec.expiresAt`, then
   `status.expiresAt`, then `creationTimestamp + ttlMinutes`. **Unclaimed warm
   members are skipped** — their clock starts at assignment, and reaping them would
   drain the pool. Delete uses background propagation so a stuck finalizer can't
   block the loop; the owner's marker is released on delete so they can create again.
2. **Owner-reference cascade** — deleting the claim tears down all composed
   children (Crossplane propagates the delete).
3. **Sweep CronJob** (`dryRun: true` on prod) — deletes orphaned session namespaces
   (managed, older than `maxAgeHours`, no surviving claim). Backstop against
   finalizer deadlocks (see the 2026-06-24 post-mortem).

> **One-sandbox-per-user marker.** The invariant is enforced by a per-owner marker
> `ConfigMap` (`sbxowner-<hash>`), created *before* a member is claimed in
> `Assign`. A duplicate create fails `AlreadyExists` → 409. The marker is deleted
> when the session is deleted or TTL-reaped; the pool manager also GCs markers
> orphaned by a request that crashed between marker create and member claim
> (older than a 2-minute grace window, not currently queued or holding a claim).

---

## 11. Scaling & failure modes

- **Backend** — currently a **single replica** (the assignment queue is in-memory,
  per-replica). Claim CRUD, assignment, and TTL are all safe to run multi-replica
  (assignment CASes on resourceVersion; markers enforce one-per-user at the API
  server); the queue would need shared/rebuildable state before scaling out.
- **Crossplane / controllers** — leader-elected; level-triggered convergence.
- **Backpressure** — **one sandbox per user** (marker), plus a **`POOL_MAX_TOTAL`
  concurrent ceiling** (warm + live) that caps how many sandboxes exist at once.
  The pool sizing and the control-plane capacity ceiling are discussed in
  [`hot-pool-design.md`](./hot-pool-design.md) §7.
- **Crash safety** — claims live in etcd; the TTL loop and pool manager both run an
  immediate reconcile on start, so a restart self-heals. The in-memory queue is the
  only volatile state, and losing it just re-queues users on their next POST.

---

## 12. What's still to build

- **Observability/alerting** — sessions stuck Terminating, vcluster not ready >N min, authz deny rate, pool depth / queue length, control-plane capacity headroom.
- **Multi-replica backend** — durable or rebuildable queue state before running >1 replica.
- **Starter labs** — `starterLabRef` field exists but is unused.
- **Sweep to enforce** — set `sweep.dryRun: false` once the sweep's logged output has been confirmed on prod.
