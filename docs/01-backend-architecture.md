# KubeSandbox — Architecture

**Status:** living design (updated 2026-07-01 — G1 + G2 live on prod-k3s)
**Audience:** platform engineers building/operating KubeSandbox
**Related:** [`02-auth-design.md`](./02-auth-design.md) · [`03-implementation-plan.md`](./03-implementation-plan.md)

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

- **Self-serve & fast** — request to ready terminal in under a minute (vcluster cold-boot ~4–5 min is the dominant wait; provisioning itself is fast).
- **Strong isolation** — one tenant can never reach another's cluster or pods.
- **Ephemeral by default** — every session has a TTL and is reaped safely.
- **Declarative provisioning** — backend writes one claim; Crossplane does the rest.
- **Browser-only access** — users get a terminal via ttyd, never a kubeconfig or a direct route to the vcluster API.

---

## 2. Components (as built)

| Component | Where | Role | Status |
|---|---|---|---|
| **Frontend SPA** | chart `kubesandbox-charts/frontend` | Sign-in (Authentik OIDC), session dashboard, "open terminal." | Scaffold; not yet built (G5). |
| **Backend service** | Go source `backend/`, chart `kubesandbox-charts/kubesandbox-backend` | Creates/lists/deletes claims (via `client-go` dynamic client), enforces the one-sandbox-per-user rule + TTL loop, owns the full OIDC/PKCE auth flow for session access, answers ext-authz ownership checks. | **Built and live on prod-k3s (G1 + G2).** |
| **XRD / claim** | `crossplane/.../kubesandbox-session-xrd.yaml` | `platform.kubesandbox.com` `KubeSandboxSession` API. | **Built.** |
| **Composition** | `crossplane/.../kubesandbox-session-composition.yaml` | Fans one claim into 8 managed resources per session. | **Built.** |
| **Authentik OIDC clients** | `crossplane/.../authentik-oidc-app.yaml`, `kubesandbox-{frontend,backend}-auth.yaml` | OIDC clients for frontend and backend provisioned by Crossplane Terraform Workspaces. | **Backend client built + live.** Frontend client built (waiting for G5). |
| **Envoy Gateway** | `envoy-gateway/.../kubesandbox-gateway.yaml`, `-proxy-config.yaml` | Shared `kubesandbox` Gateway (TLS termination), routing + session ownership authz. | **Built and live.** |
| **Session SecurityPolicy** | `securitypolicy-session.yaml` (chart) | Ext-authz to backend `/authz` for all session routes (`session-route: "true"` label). | **Built and live (prod, rev 11).** |
| **OIDC callback route** | `httproute-callback.yaml` (chart) | Unauthenticated route for `/oauth2/callback` — login completion endpoint. | **Built and live.** |
| **Backend NetworkPolicy** | `networkpolicy.yaml` (chart) | Restricts backend ingress to `envoy-gateway-system` only (anti-spoofing). | **Built and live.** |
| **TTL cleanup loop** | `backend/internal/kubernetes/cleanup.go` | In-process loop that deletes expired claims. `status.expiresAt` fallback to `creationTimestamp + ttlMinutes`. | **Built; not yet live-tested end-to-end (G3).** |
| **Sweep CronJob** | `sweep-cronjob.yaml` (chart) | Backstop: deletes orphaned session namespaces older than `maxAgeHours` with no surviving claim. | **Built, `dryRun: true` on prod.** |

---

## 3. The two control loops

A **synchronous API plane** (backend) and an **asynchronous control plane**
(Crossplane + the cluster). The API never blocks on infrastructure.

```
                         ┌────────────────────────────────────────────────────┐
  Browser                │              Envoy Gateway (kubesandbox)             │
 ┌─────────┐   TLS       │   kubesandbox.com            → frontend SPA (G5)    │
 │  SPA +  │◀───────────▶│   kubesandbox.com/api        → backend /api         │
 │  ttyd   │             │   kubesandbox.com/s/{id}     → session ttyd         │
 │         │             │   kubesandbox.com/oauth2/callback → backend (login) │
 └─────────┘             └───────────┬──────────┬───────────────────────────────┘
                                     │          │ ext-authz (every /s/{id} req)
                                     │          ▼
                                     │  backend /authz: cookie → owns? → 200/302/403
                                     │
                                     │ create/read claim (/api)
                                     ▼
        ┌───────────────── Management Cluster (Kubernetes API) ─────────────────┐
        │  KubeSandboxSession claim  ──▶ XKubeSandboxSession (composite)         │
        │        │                                                               │
        │        ▼  Crossplane Composition (patch-and-transform + function-auto-ready)
        │   provisions 8 resources per session:                                 │
        │     • Namespace  {claim-ns}-{claim-name}                              │
        │     • ResourceQuota (profile-shaped)                                  │
        │     • vcluster (Helm Release — cold-boot ~4–5 min)                    │
        │     • NetworkPolicy (allow envoy-gateway-system + kube-system DNS)    │
        │     • Pod  "shell"  (ttyd, jurassicjey/ttyd-k8s:ttyd, kubeconfig mnt) │
        │     • Service "shell" :80→8080                                        │
        │     • HTTPRoute  kubesandbox.com/s/{id} in kubesandbox namespace      │
        │     • ReferenceGrant in session namespace (cross-ns backendRef)       │
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
| `tenantRef` | string (req) | Logical tenant. Currently `tenantRef == ownerRef == sub` (1-tenant-per-user). |
| `ownerRef` | string (req) | Authentik `sub` claim (from `hashed_user_id` mode — a 64-char hex hash, **not** email or UUID). |
| `profile` | enum (req) | `starter` \| `standard` \| `advanced` — maps to resource shapes. |
| `ttlMinutes` | int | 15–1440, default 60. Backend TTL loop deletes expired claims. |
| `workspaceImage` | string | Default `jurassicjey/ttyd-k8s:ttyd`. Must bundle ttyd ≥ 1.6 (needs `-b` base-path flag). |
| `starterLabRef` | string | Optional starter-lab/template id (field exists, unused). |
| `resources.cpu` / `.memory` | string | Shell pod request/limit. Set by backend from `profile`: starter `250m/256Mi`, standard `500m/512Mi`, advanced `1/1Gi`. |

### Status

`phase`, `message`, `expiresAt`, `sessionNamespace`, `vclusterRelease`,
`workspacePod`, `workspaceReady`. The backend reads these to surface progress and
the session URL; readiness is computed by the `function-auto-ready` pipeline step.

> **Note:** `status.expiresAt` is not currently populated by the Composition.
> The TTL loop falls back to `creationTimestamp + spec.ttlMinutes`.

---

## 5. Provisioning (what the Composition does)

The `patch-and-transform` pipeline produces **8 managed resources** per session.
Resources 1–6 land in the per-session namespace `{claim-ns}-{claim-name}`;
resources 7–8 require special namespace placement for the auth model to work:

1. **Namespace** — the session sandbox, labels `kubesandbox.com/managed: "true"` +
   `profile`; owner/tenant stored as **annotations** (not labels — values may
   contain `@`).
2. **ResourceQuota** — caps CPU/memory/pods/services (hard: 4 CPU, 8 Gi, 10 pods,
   20 services/configmaps/secrets).
3. **vcluster `Release`** — user's private cluster. Cold-boot ~4–5 min while
   coredns comes up and the kubeconfig Secret is published. The shell pod blocks
   on the Secret; no action required.
4. **NetworkPolicy** — allows ingress from `envoy-gateway-system` (gateway → ttyd)
   and egress to the vcluster service endpoint + `kube-system` DNS; otherwise
   isolated between sessions.
5. **Shell Pod (ttyd)** — copies the vcluster kubeconfig to `/tmp`, sets
   `KUBECONFIG`, and runs `ttyd -W -b "$BASE_PATH" -p 8080 sh` where
   `BASE_PATH=/s/{ns}-{name}`. Hardened: `automountServiceAccountToken: false`,
   `enableServiceLinks: false`, `readOnlyRootFilesystem: true`, non-root, only the
   vcluster kubeconfig mounted (via Secret volume).
6. **Service `shell`** — ClusterIP, `:80 → 8080`.
7. **HTTPRoute** — placed in the **`kubesandbox` namespace** (not the session
   namespace). Host `kubesandbox.com`, path prefix `/s/{ns}-{name}`, cross-namespace
   backendRef to the session-namespace `shell` Service. The `kubesandbox.com/session-route: "true"` label lets the shared SecurityPolicy attach via `targetSelectors`.
8. **ReferenceGrant** — placed in the **session namespace**. Permits the
   `kubesandbox`-ns HTTPRoute to reference the per-session `shell` Service
   (required by Gateway API for cross-namespace backendRefs).

> **Routing is path-based**: a single `kubesandbox.com` TLS cert covers all
> sessions — no wildcard `*.kubesandbox.com` required. The HTTPRoute matches host
> `kubesandbox.com` + path prefix `/s/{id}` and forwards the prefix unchanged;
> ttyd serves under `BASE_PATH` with the `-b` flag (no Envoy URL rewrite needed).

All children are sync-wave-aligned at wave 18 and owned by the composite so
deletion cascades cleanly.

---

## 6. The browser terminal (ttyd, not a backend proxy)

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
                                     └─▶ ttyd :8080 ──▶ sh + kubectl (private vcluster)
```

The user never holds cluster credentials and never gets a route to the vcluster
API — only to ttyd, which is `kubectl`-scoped to *their* vcluster via the mounted
kubeconfig.

---

## 7. Session authentication & authorization (G2, live)

The backend owns the **complete OIDC/PKCE login flow** for session access. There
is no edge OIDC filter — the `SecurityPolicy` does ext-authz only, and the backend
`/authz` handler drives the auth:

**Request flow (unauthenticated):**
1. Browser hits `kubesandbox.com/s/{id}` — Envoy calls `GET /authz`.
2. No session cookie → backend generates PKCE verifier + S256 challenge, signs a
   state JWT (HMAC-SHA256, 5-min TTL, carries `code_verifier` + `originalURL`),
   returns `302` → Authentik `/application/o/authorize/`.
3. User logs in to Authentik; Authentik redirects to
   `kubesandbox.com/oauth2/callback?code=...&state=...`.
4. Backend (via the unauthenticated `/oauth2/callback` HTTPRoute) verifies the
   state JWT, exchanges the code at Authentik's token endpoint
   (`/application/o/token/`), parses the ID token claims (`sub`, `email`, `name`),
   sets an `HttpOnly Secure SameSite=Lax` session cookie (HMAC-SHA256 signed,
   payload: `sub`/`email`/`name`/`exp`), and redirects back to the original URL.
5. Browser retries `kubesandbox.com/s/{id}` — Envoy calls `/authz` again; this
   time the cookie is present and valid → ownership check → 200 → ttyd.

**Ownership check:** backend resolves `{id}` from `X-Forwarded-Uri`, looks up the
`KubeSandboxSession` claim, compares `ownerRef == cookie.sub`. Owner → 200;
non-owner / unknown / malformed → 403 (no existence leak); backend error → 503
(fail closed).

**Key implementation detail:** Authentik's `sub_mode: hashed_user_id` means the
`sub` claim is the user's `uid` field — a 64-char hex hash, **not** the email or
UUID. The backend stores this as `ownerRef`; the frontend (G5) must use the `sub`
from the JWT when creating sessions via `/api`.

**Authentik endpoint note:** auth/token endpoints are at `/application/o/authorize/`
and `/application/o/token/` — shared across providers. The per-provider issuer URL
(e.g. `https://auth.jeremymr.dev/application/o/kubesandbox-backend/`) does **not**
prefix these endpoints; `authEndpoint`/`tokenEndpoint` must be set explicitly in
the chart values.

---

## 8. Isolation & security

- **vcluster** — each user is `cluster-admin` only *inside* their throwaway cluster; the management-plane API is not exposed to users.
- **Per-session Namespace** with **ResourceQuota** (CPU/memory/pods/services capped).
- **Session NetworkPolicy** — only `envoy-gateway-system` may reach the shell pod on 8080; egress limited to the vcluster service + kube-system DNS. Sessions are isolated from each other.
- **Backend NetworkPolicy** — only `envoy-gateway-system` may reach the backend on 8080. Prevents in-cluster pods from spoofing `X-User-*` identity headers (anti-spoofing for the `/api` and `/authz` endpoints).
- **ttyd pod hardening** — non-root (`runAsUser: 1000`), `readOnlyRootFilesystem: true`, no host SA token (`automountServiceAccountToken: false`), no service links; writable area confined to `/tmp` emptyDir. Only credential is the mounted vcluster kubeconfig.
- **Session ownership** — the shared `SecurityPolicy` (ext-authz → backend `/authz`) enforces that only the session owner can reach their ttyd. Verified live: negative test (user B → 403) passed.
- **Backend /api** — JWT `SecurityPolicy` enforces Authentik bearer tokens; rejects missing/invalid tokens with `401`. `claimToHeaders` injects `X-User-Id`/`X-User-Email`/`X-User-Name`/`X-User-Groups`.

---

## 9. Lifecycle & garbage collection

Sessions are **ephemeral and disposable**. Three layers:

1. **TTL loop** — in-backend `TTLController` (`cleanup.go`) runs every
   `TTL_CLEANUP_INTERVAL` (default 1 min), deleting claims past expiry. Expiry
   prefers `status.expiresAt`; falls back to `creationTimestamp + spec.ttlMinutes`
   (since the Composition does not populate `expiresAt`). Background delete
   propagation — a stuck finalizer can't block the loop.
2. **Owner-reference cascade** — deleting the claim tears down all 8 composed
   children (Crossplane propagates the delete).
3. **Sweep CronJob** (`sweep-cronjob.yaml`, `dryRun: true` on prod) — deletes
   orphaned session namespaces (label `kubesandbox.com/managed: "true"`, older
   than `maxAgeHours`, with no surviving claim). Backstop against finalizer
   deadlocks (see 2026-06-24 post-mortem).

> **Cleanup safety.** Per the 2026-06-24 post-mortem, a single failing finalizer
> once deadlocked the `workspaces.tf.upbound.io` CRD. Session teardown uses
> background delete propagation so a stuck finalizer can't block the TTL loop.
> Set `sweep.dryRun: false` in the prod chart once a full TTL-expiry cycle has
> been live-tested (G3 next step).

---

## 10. Scaling & failure modes

- **Backend** — stateless; run 2+ replicas behind the gateway. Re-derives everything from claims. Session cookie is HMAC-signed (no server-side store needed) so any replica can verify it.
- **Crossplane / controllers** — leader-elected; level-triggered convergence.
- **Backpressure** — **one sandbox per user**, enforced atomically by the deterministic per-owner claim name (`s-{sha256(owner)[:16]}`): a second create for the same user fails `AlreadyExists` at the API server → 409. No list-then-create race, no configurable cap.
- **Crash safety** — kill any process; state is in the claims (etcd). The TTL loop re-starts clean with no loss.

---

## 11. What's still to build

See [`03-implementation-plan.md`](./03-implementation-plan.md). In short:

- **G3 TTL live-test** — create a session, let it expire, confirm the backend reaps it; then set `sweep.dryRun: false`.
- **G4 valid-bearer verification** — the JWT `SecurityPolicy` on `/api` is active and rejecting invalid tokens, but a real Authentik bearer token has never been confirmed end-to-end. Needed before the frontend ships.
- **G5 Frontend SPA** — sign-in, session dashboard, "open terminal." Must use JWT bearer tokens on `/api` (not cookies) and use `sub` from the JWT (not email) as the identity key.
- **G7 Observability/alerting** — sessions stuck Terminating, vcluster not ready >N min, authz deny rate.
- **G8 Starter labs** — `starterLabRef` field exists but is unused.
