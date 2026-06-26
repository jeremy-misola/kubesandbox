# KubeSandbox — Architecture

**Status:** living design (reconciled to the repo on 2026-06-26)
**Audience:** platform engineers building/operating KubeSandbox
**Related:** [`02-auth-design.md`](./02-auth-design.md) · [`03-implementation-plan.md`](./03-implementation-plan.md)

> This document describes the system **as actually built in this repo**, plus the
> backend control service that is still to be written. Where an earlier draft
> assumed a backend WebSocket exec-proxy, the repo instead serves the terminal
> with **ttyd reached directly through Envoy Gateway** — this doc reflects that.

---

## 1. What it is

KubeSandbox is a self-serve platform for **ephemeral Kubernetes sandboxes**. A
user signs in, creates a session, and gets a browser terminal wired to a private,
throwaway [vcluster](https://www.vcluster.com/) preloaded with `kubectl`. Every
session has a TTL and is garbage-collected.

The platform is **declarative end-to-end**: the backend writes one Crossplane
claim (`KubeSandboxSession`) and a Composition provisions everything else. The
browser terminal is **ttyd** running in the session pod, exposed through the
shared Envoy Gateway on a per-session path (`kubesandbox.com/s/{id}`).

### Design goals

- **Self-serve & fast** — request to ready terminal in seconds.
- **Strong isolation** — one tenant can never reach another's cluster or pods.
- **Ephemeral by default** — every session has a TTL and is reaped safely.
- **Declarative provisioning** — backend writes one claim; Crossplane does the rest.
- **Browser-only access** — users get a terminal via ttyd, never a kubeconfig or a direct route to the vcluster API.

---

## 2. Components (as built)

| Component | Where | Role |
|---|---|---|
| **Frontend SPA** | chart `kubesandbox-charts/frontend` | Sign-in (Authentik OIDC), session dashboard, "open terminal." *(scaffold; to build)* |
| **Backend service** | Go source `backend/`, chart `kubesandbox-charts/kubesandbox-backend` | Creates/lists/deletes claims (via `client-go` dynamic client), enforces quota + TTL, answers session-ownership authz. Identity from Envoy-forwarded `X-User-*` headers (G1). *(scaffold; to build — see plan)* |
| **XRD / claim** | `crossplane/.../kubesandbox-session-xrd.yaml` | `platform.kubesandbox.com` `KubeSandboxSession` API. |
| **Composition** | `crossplane/.../kubesandbox-session-composition.yaml` | Fans one claim into ns + quota + vcluster + netpol + ttyd pod + svc + HTTPRoute. |
| **Authentik OIDC** | `crossplane/.../authentik-oidc-app.yaml`, `kubesandbox-frontend/.../*-auth.yaml` | OIDC clients provisioned by Crossplane Terraform. |
| **Envoy Gateway** | `envoy-gateway/.../kubesandbox-gateway.yaml`, `-proxy-config.yaml` | Shared `kubesandbox` Gateway (TLS terminate), routing + edge auth. |

---

## 3. The two control loops

A **synchronous API plane** (backend) and an **asynchronous control plane**
(Crossplane + the cluster). The API never blocks on infrastructure.

```
                         ┌────────────────────────────────────────────┐
  Browser                │              Envoy Gateway (kubesandbox)     │
 ┌─────────┐   TLS       │   kubesandbox.com            → frontend SPA  │
 │  SPA +  │◀───────────▶│   api.kubesandbox.com        → backend       │
 │  ttyd   │             │   kubesandbox.com/s/{id}     → session ttyd  │
 └─────────┘             └───────────────┬──────────────────────────────┘
                                         │ create/read claim
                                         ▼
        ┌───────────────── Management Cluster (Kubernetes API) ─────────────────┐
        │  KubeSandboxSession claim  ──▶ XKubeSandboxSession (composite)          │
        │        │                                                               │
        │        ▼  Crossplane Composition (patch-and-transform + function-auto-ready)
        │   provisions per session:                                             │
        │     • Namespace  {claim-ns}-{claim-name}                              │
        │     • ResourceQuota (profile-shaped)                                  │
        │     • vcluster (Helm Release)                                         │
        │     • NetworkPolicy (allow envoy-gateway-system + kube-system DNS)    │
        │     • Pod  "shell"  (ttyd, jurassicjey/ttyd-k8s:ttyd, kubeconfig mnt) │
        │     • Service "shell" :80→8080                                        │
        │     • HTTPRoute  kubesandbox.com/s/{id} → shell svc (path-based)      │
        └───────────────────────────────────────────────────────────────────────┘
```

**Why split it.** `POST /sessions` returns as soon as the claim is written. The
heavy lifting (vcluster boot, pod scheduling) happens out-of-band; the frontend
polls `GET /sessions/{id}` until `status.workspaceReady`. This keeps the backend
**stateless** (state lives in the claims), **horizontally scalable**, and
**crash-safe**; Crossplane reconciliation is level-triggered and self-heals.

---

## 4. Data model — the `KubeSandboxSession` claim

API group `platform.kubesandbox.com/v1alpha1`, claim kind `KubeSandboxSession`
(composite `XKubeSandboxSession`). This is the **source of truth** for a session.

### Spec

| Field | Type | Notes |
|---|---|---|
| `tenantRef` | string (req) | Logical tenant that owns the session. |
| `ownerRef` | string (req) | User id from Authentik (`sub`) or the app DB. |
| `profile` | enum (req) | `starter` \| `standard` \| `advanced` — default resource shape. |
| `ttlMinutes` | int | 15–1440, default 60. Backend must still enforce deletion. |
| `workspaceImage` | string | default `jurassicjey/ttyd-k8s:ttyd` — must bundle a web terminal (ttyd). |
| `starterLabRef` | string | optional starter-lab/template id. |
| `resources.cpu` / `.memory` | string | shell pod request/limit (default `500m` / `512Mi`). |

### Status

`phase`, `message`, `expiresAt`, `sessionNamespace`, `vclusterRelease`,
`workspacePod`, `workspaceReady`. The backend reads these to surface progress and
the session URL; readiness is computed by the `function-auto-ready` pipeline step.

---

## 5. Provisioning (what the Composition does)

One `patch-and-transform` pipeline produces, all in the per-session namespace
`{claim-ns}-{claim-name}`:

1. **Namespace** — the session sandbox.
2. **ResourceQuota** — caps CPU/memory/pods (profile-shaped).
3. **vcluster `Release`** — the user's private cluster; its API is reachable only
   in-namespace at `https://{ns}-{name}-vcluster...:443`.
4. **NetworkPolicy** — allows ingress from `envoy-gateway-system` (so the gateway
   can reach ttyd) and egress to `kube-system` DNS; otherwise isolated.
5. **Shell Pod (ttyd)** — copies the vcluster kubeconfig to `/tmp`, sets
   `KUBECONFIG`, and runs `ttyd -W -p 8080 sh`. Hardened:
   `automountServiceAccountToken: false`, `enableServiceLinks: false`, its only
   credential is the mounted vcluster kubeconfig.
6. **Service `shell`** — `:80 → 8080`.
7. **HTTPRoute** — parent `kubesandbox` Gateway, backend → `shell` Service.

All children are sync-wave-aligned (mostly 25; the HTTPRoute at 18) and owned by
the composite so deletion cascades.

> **Routing scheme (decided 2026-06-26): path-based.** Today the composition's
> HTTPRoute is host-based (`{ns}-{name}.kubesandbox.com`). It is being switched to
> **path-based** `kubesandbox.com/s/{id}` so a single `kubesandbox.com` TLS cert
> covers everything (no wildcard). This requires the HTTPRoute to match host
> `kubesandbox.com` + path prefix `/s/{id}` and ttyd to serve under that base path
> (Envoy `URLRewrite` to strip the prefix, or `ttyd -b /s/{id}`). See the
> implementation plan (G2b / Phase 0).

---

## 6. The browser terminal (ttyd, not a backend proxy)

The terminal is **ttyd inside the session pod**, exposed through Envoy:

```
Browser ──TLS, WS──▶ Envoy Gateway ──HTTPRoute kubesandbox.com/s/{id}──▶ shell Service ──▶ ttyd :8080 ──▶ sh + kubectl (vcluster)
```

The user never holds cluster credentials and never gets a route to the vcluster
API — only to ttyd, which is `kubectl`-scoped to *their* vcluster via the mounted
kubeconfig.

> **Why this differs from the earlier skeleton.** An earlier draft proxied the
> terminal through the backend (`kubectl exec` over a WebSocket). The repo's ttyd
> approach is simpler operationally (no backend in the hot data path) but moves
> the **authorization burden to the edge**: because ttyd is reachable by URL,
> the gateway must ensure only the session **owner** can reach it. That is the
> central problem of [`02-auth-design.md`](./02-auth-design.md) (the "session
> ownership" gap). The backend-exec-proxy remains a viable alternative if you ever
> want auth to live entirely in the backend.

---

## 7. Isolation & security

- **vcluster** — each user is `cluster-admin` only *inside* their throwaway cluster.
- **Per-session Namespace** with **ResourceQuota** (and room for a LimitRange).
- **NetworkPolicy** — only the gateway namespace may reach the pod; egress limited
  (DNS to kube-system). Sessions can't reach each other.
- **ttyd pod hardening** — non-root intent, no host SA token, only the vcluster
  kubeconfig mounted; writable area confined to `/tmp` emptyDir.
- **Edge** does OIDC (authN) + ownership authZ (see auth doc); the backend owns
  fine-grained "owns this session."

---

## 8. Lifecycle & garbage collection

Sessions are **ephemeral and disposable**. Three layers:

1. **TTL** — backend (or a small controller) deletes claims past
   `status.expiresAt`.
2. **Owner-reference cascade** — deleting the claim tears down all children.
3. **Sweep CronJob** — deletes orphaned namespaces older than `maxAge` as backstop.

> **Cleanup must not be able to wedge a CRD.** Per the 2026-06-24 post-mortem
> (`docs/post-mortems/`), a single failing `terraform destroy`/finalizer once
> deadlocked the `workspaces.tf.upbound.io` CRD and caused a cluster-wide
> create/delete loop. Session teardown should favor delete policies that can't
> deadlock (orphan-then-sweep for any data-bearing child), and bound/idempotent
> deletes. See the implementation plan, Phase 3.

---

## 9. Scaling & failure modes

- **Backend** — stateless; run 2+ replicas behind the gateway. Re-derives
  everything from claims.
- **Crossplane / controllers** — leader-elected; level-triggered convergence.
- **Backpressure** — cap total concurrent sessions; excess claims sit pending.
- **Crash safety** — kill any process; state is in the claims (etcd).

---

## 10. What's still to build

See [`03-implementation-plan.md`](./03-implementation-plan.md). In short: the
**backend control service** (G1), **session-ownership authz** (G2), **TTL/cleanup**
(G3), backend Authentik client (G4), the **frontend SPA** (G5), tenant/quota model
(G6), observability/alerting (G7), and starter labs (G8).
