# KubeSandbox — Implementation Plan

**Status:** largely delivered — historical plan, kept for context
**Audience:** Jeremy (platform owner)
**Last updated:** 2026-07-02

> **Update (2026-07-02) — what actually shipped.** G1–G5 are built and live on
> prod-k3s: the backend control service, session ownership authz (G2, delivered
> as **Options A+B** — backend-owned OIDC + shared-namespace routes, not the plain
> Option A locked below), server-side TTL enforcement (G3), and the frontend SPA
> (G5). Two decisions below were **superseded**:
>
> - **Session `{id}` / one-per-user.** No longer the owner-derived claim name.
>   The hot warm-pool provisions members before an owner exists, so member names
>   are generated (`s-pool-…`) and one-sandbox-per-user is enforced by an
>   atomically-created **per-owner marker ConfigMap** (`sbxowner-…`). The
>   owner-derived name (`s-{sha256(owner)[:16]}`) survives only on the
>   pool-disabled direct-create path.
> - **Profiles.** The `starter/standard/advanced` model was dropped: every sandbox
>   is identical so warm members stay fungible. `ttlMinutes` is the only knob.
>
> TTL cleanup graduated as an in-process loop (`cleanup.go`), not a
> `controller-runtime` controller. See [`hot-pool-design.md`](../reference/hot-pool-design.md)
> and [`backend-architecture.md`](../reference/backend-architecture.md) for the
> as-built system. The phased plan below is retained as a sequencing record.

### Locked decisions (2026-06-26)

| # | Decision |
|---|---|
| Authz model | **Option A** — Envoy ext-authz / ForwardAuth → backend `/authz`. |
| Tenant model | **1 tenant = 1 user** for now (`tenantRef == ownerRef == Authentik sub`). |
| Backend source | **Go source at repo-root `backend/`**, built into the existing `kubesandbox-charts/kubesandbox-backend` Helm chart image (`jurassicjey/kubesandbox-backend`). *(The earlier `operators-helm/operators/...` path does not exist in this repo; the chart lives under `kubesandbox-charts/`.)* |
| Backend identity (G1) | **Trust Envoy-forwarded identity headers** (`X-User-Email` / `X-User-Name` / `X-User-Groups`, optional `X-User-Id`). Edge does OIDC; the backend reads `sub`/email from headers. In-backend OIDC/JWT validation is **deferred to G4**. Defense in depth: lock the gateway→backend path (NetworkPolicy / mTLS). |
| Kubernetes client | **`client-go` dynamic client** (`unstructured`) for `KubeSandboxSession` claim CRUD in G1 — no codegen needed for the CRD. Graduate to **`controller-runtime`** (informers + leader election) for the G3 TTL controller, reusing the same scheme. |
| Session URL | **Path-based:** `kubesandbox.com/s/{id}` (single cert, no wildcard). Replaces host-based `{ns}-{name}.kubesandbox.com`. |
| Session `{id}` | **Deterministic per-owner claim name** `s-{sha256(owner)[:16]}`; public id `= {namespace}-{name}` (no PII — hashed — *and* the routing path). One owner ⇒ one name, which is how **one sandbox per user** is enforced (API-server uniqueness, race-free). Supersedes the earlier opaque-random-name decision. |
| TTL / cleanup | **In the backend** (no separate controller for now). |

This plan is written against the **actual repo state**, not a greenfield. A large
part of the platform already exists in GitOps:

- **Crossplane session composition** (`crossplane/.../kubesandbox-session-composition.yaml`)
  already provisions, per session: namespace, ResourceQuota, vcluster (Helm
  `Release`), NetworkPolicy, the **ttyd shell Pod** (`jurassicjey/ttyd-k8s:ttyd`),
  a `shell` Service, and a per-session **HTTPRoute** on host
  `{claim-ns}-{claim-name}.kubesandbox.com`.
- **XRD / claim** `platform.kubesandbox.com` (`KubeSandboxSession`) with
  `tenantRef`, `ownerRef`, `profile` (starter/standard/advanced), `ttlMinutes`,
  `workspaceImage`, `resources`, and a rich `status` (phase, expiresAt,
  sessionNamespace, vclusterRelease, workspacePod, workspaceReady).
- **Authentik OIDC provisioning** via Crossplane Terraform — a reusable module
  ConfigMap plus a frontend `Workspace` that creates the `kubesandbox-frontend`
  OIDC client and writes `kubesandbox-frontend-client-secret` into the
  `kubesandbox` namespace.
- **Envoy Gateway** with the shared `kubesandbox` Gateway, EnvoyProxy config, and
  TLS cert. SecurityPolicy/JWT patterns already exist for other apps
  (adguard, code-server, longhorn, stirling-pdf).
- **Helm scaffolds** for `kubesandbox-backend` and `kubesandbox-frontend`
  (chart value files exist but are empty / pre-resources only).

So this is **not** "build the platform." It is "build the **backend control
service** and close the remaining gaps so the existing infra becomes a
product." The earlier Go skeleton assumed a backend WebSocket exec-proxy; the
repo instead uses **ttyd reached directly through Envoy**, so the backend's job
is orchestration and authorization, not terminal proxying.

---

## 1. Gap analysis — what's missing

| # | Gap | Status today | Priority |
|---|---|---|---|
| G1 | **Backend control service** (creates/lists/deletes `KubeSandboxSession` claims, enforces one sandbox per user, surfaces session URL). | Helm scaffold only, no app. | **P0** |
| G2 | **Per-session ownership authorization.** Session URLs are routed but not protected by an *ownership-aware* policy — any authenticated Authentik user could reach any session. | **✅ Done (rev 11, 2026-07-01).** Backend-owned OIDC/PKCE flow (Options A+B). Full browser smoke test on prod-k3s: unauthenticated → 302 → Authentik login → cookie → authz 200 → ttyd. Negative test (user B → 403) confirmed. `sessionAuth.enabled: true` on prod. | ~~P0~~ |
| G1h | **Backend NetworkPolicy (anti-spoofing).** Backend trusts `X-User-*`; nothing stopped an in-cluster pod from spoofing identity on `/api` / `/authz`. | **Done rev 5** (`networkpolicy.yaml`, default-on, ingress from `envoy-gateway-system` only). | ~~P0~~ |
| G2b | **Switch routing to path-based** `kubesandbox.com/s/{id}` — composition HTTPRoute + ttyd base-path. | **Done 2026-06-26** (composition updated). | ~~P0~~ |
| G3 | **TTL enforcement / cleanup.** XRD has `ttlMinutes` + `status.expiresAt`, but something must actually delete expired claims — safely (see post-mortem). | **Done rev 5**: in-backend TTL loop (`cleanup.go`) + backstop sweep CronJob (`sweep-cronjob.yaml`, dryRun). Unit-tested; live-test pending. | ~~P0~~ |
| G4 | **Backend Authentik client + token validation.** Frontend client exists; backend needs to validate user identity and map `sub → ownerRef`. | Not implemented. | **P1** |
| G5 | **Frontend SPA** (signup, session dashboard, "open terminal"). | Scaffold only. | **P1** |
| G6 | **Tenant / user model & quotas** (one sandbox per user via deterministic claim naming, profiles → resources). | Partial (profile enum exists). | **P1** |
| G7 | **Observability & alerting** — especially "claim stuck Terminating" and "vcluster not ready," per post-mortem action items. | Not implemented. | **P2** |
| G8 | **Starter labs** (`starterLabRef`) seeding content into the vcluster. | Field exists, unused. | **P2** |

---

## 2. Target end-state architecture (reconciled)

```
 Browser ──TLS──▶ Envoy Gateway (kubesandbox) ──┬─▶ kubesandbox.com         → frontend SPA
                                                ├─▶ api.kubesandbox.com     → backend service
                                                └─▶ kubesandbox.com/s/{id}  → per-session ttyd
                                                       ▲
                                                       │ (ext-authz / ForwardAuth: "does caller own session {id}?")
                                                       └────────────── backend /authz endpoint

 backend ──creates──▶ KubeSandboxSession claim ──Crossplane──▶ ns + quota + vcluster + ttyd + svc + HTTPRoute
 backend ──TTL loop──▶ deletes expired claims (safe cleanup)
```

The decisive design choice is **G2**: a plain OIDC `SecurityPolicy` authenticates
*a* user but does not check that *this* user owns *this* session. **Decision:
Option A.** The session route carries a SecurityPolicy that first does OIDC
(cookie), then calls the backend's `/authz` endpoint with the original request
path (`/s/{id}`); the backend resolves `{id}` to a claim and allows iff
`ownerRef == token.sub`. Ownership logic stays in one place (the backend), with
no per-session secrets and immediate revocation (delete claim → 403/404).

> Option B (backend-minted signed per-session token validated by a JWT
> SecurityPolicy) was considered and **not** chosen — it pushes token issuance,
> lifetime, and revocation onto us for no real benefit here.

---

## 3. Phased delivery

### Phase 0 — Foundations (0.5 wk)
Decisions are locked (see top of doc). Remaining setup:
- [ ] Backend = **Go**, source at repo-root `backend/`, built into the existing
      `kubesandbox-charts/kubesandbox-backend` chart image. Kubernetes access via
      the **`client-go` dynamic client** (decided). HTTP layer still open —
      stdlib `net/http` ServeMux (zero deps) vs. Gin (nicer ergonomics, extra
      deps); both fine for this small API surface.
- [ ] **Identity (G1) via forwarded headers:** the backend trusts the
      `X-User-*` identity headers Envoy injects after edge OIDC and maps them to
      `ownerRef`/`tenantRef`. The dedicated backend Authentik OIDC client (for
      validating `api.kubesandbox.com` JWTs directly) is **deferred to G4** — it
      is not needed to ship G1.
- [ ] **Tenant = user:** the backend sets `tenantRef = ownerRef = sub` (from the
      forwarded identity) on every claim. Claims are the source of truth for
      sessions; no app DB yet.
- [x] **Switch session routing to path-based** (G2b, done 2026-06-26): the
      composition's `shell-httproute` now matches host `kubesandbox.com` + path
      prefix `/s/{ns}-{name}`, and ttyd runs with `-b /s/{ns}-{name}` (base path,
      no rewrite — forwards the prefix unchanged). `{id}` = `{claim-ns}-{claim-name}`.
      The `kubesandbox-cert` already covers the apex `kubesandbox.com`, so the
      `*.kubesandbox.com` wildcard is no longer required for sessions.

### Phase 1 — Backend control service (G1) (1.5–2 wk)
- [ ] HTTP API: `POST/GET/DELETE /sessions`, `GET /sessions/{id}`, `GET /health`
      (chart probes hit `/health`; keep a `/healthz` alias if convenient).
- [ ] **Identity from forwarded headers** (G1 decision): read `sub`/email from
      the `X-User-*` headers and map to `ownerRef`. *(Direct Authentik OIDC/JWT
      validation in the backend is G4 — see below.)*
- [ ] Claim CRUD via the **`client-go` dynamic client** against
      `platform.kubesandbox.com/v1alpha1` `kubesandboxsessions` in the configured
      namespace. Derive a **deterministic per-owner name**
      `s-{sha256(owner)[:16]}`; the public id is `{namespace}-{name}`.
- [ ] Create `KubeSandboxSession` claims with `tenantRef = ownerRef = sub`,
      `profile`, `ttlMinutes`; map `profile → resources` (e.g. starter
      `250m/256Mi`, standard `500m/512Mi`, advanced `1/1Gi`). The
      **one-sandbox-per-user** rule needs no explicit check: the deterministic
      claim name makes a duplicate create fail `AlreadyExists` → `409`.
- [ ] Read claim `status` and return the session URL
      (`kubesandbox.com/s/{id}`) + phase to the frontend (poll until
      `workspaceReady`).
- [ ] Backend RBAC: the chart **already ships** a backend `ClusterRole`
      (`kubesandbox-charts/kubesandbox-backend/templates/clusterrole.yaml`)
      granting `kubesandboxsessions[/status]` CRUD plus `secrets`/`namespaces`
      get/list, bound to the backend ServiceAccount. Verify/scope it to the
      session namespace; it is **separate** from the Crossplane-side
      `crossplane-kubesandbox-session-access` ClusterRole.
- [ ] Package into the existing `kubesandbox-backend` Helm chart; deploy via
      ArgoCD with a sync wave after the XRD is established.

> **G4 (deferred from this phase, P1):** add a `kubesandbox-backend` Authentik
> OIDC client (mirror `kubesandbox-frontend-auth.yaml`, writing
> `kubesandbox-backend-client-secret`) and validate Authentik JWTs **directly**
> for programmatic `api.kubesandbox.com` callers. G1 ships without this by
> trusting Envoy-forwarded identity headers; G4 hardens the API for non-browser
> clients.

### Phase 2 — Session ownership authz (G2) ✅ Done (rev 11, 2026-07-01)
- [x] Backend `GET /authz` endpoint — cookie-based auth; missing/invalid →
      PKCE redirect to Authentik; valid cookie → ownership check (200/403/503).
- [x] SecurityPolicy (ext-authz only) via `targetSelectors` on `session-route: "true"` label.
      Session HTTPRoutes moved to `kubesandbox` namespace (Option A) so the policy attaches.
      ReferenceGrant added in session namespace for cross-namespace backendRef.
- [x] WebSocket upgrade verified — ttyd WS works end-to-end through ext-authz.
- [x] Negative test: user B → 403 (confirmed on prod-k3s).

### Phase 3 — TTL enforcement & safe cleanup (G3) (1 wk)
- [ ] A controller loop (in the backend, or a small separate controller) that
      deletes claims past `status.expiresAt`.
- [ ] **Cleanup safety (per post-mortem 2026-06-24):** a failing `terraform
      destroy`/finalizer must never wedge the CRD. For data-bearing or
      delete-prone children, prefer `deletionPolicy: Orphan` or
      `managementPolicies` without `Delete`; ensure session teardown is
      idempotent and bounded. Sessions are *ephemeral and disposable*, so favor
      orphan-then-sweep over blocking deletes.
- [ ] Backstop **sweep CronJob**: delete orphaned session namespaces older than
      `maxAge`.
- [ ] Idle detection (optional): shorten TTL when no terminal traffic for N min.

### Phase 4 — Frontend SPA (G5, G6) (1.5 wk)
- [ ] OIDC login (Authentik `kubesandbox-frontend` client, redirect
      `https://kubesandbox.com/auth/callback`).
- [ ] Dashboard: list sessions, create (profile picker), open terminal (link to
      session URL), delete. Show phase/expiry.
- [ ] Profiles → `resources` mapping surfaced in UI (starter/standard/advanced).

### Phase 5 — Observability & hardening (G7) (1 wk)
- [ ] Metrics: sessions created/active/expired, provisioning latency, vcluster
      ready time, authz allow/deny.
- [ ] **Alerts (post-mortem action items):** any CRD stuck `Terminating`; any
      managed resource `Terminating` > 1h; vcluster/release not ready > N min.
- [ ] Rate limiting on `POST /sessions` (Envoy BackendTrafficPolicy or backend).
- [ ] Backstage catalog entries for the backend/frontend components + API.

### Phase 6 — Starter labs (G8) (stretch)
- [ ] Define `starterLabRef` templates; seed manifests/content into the vcluster
      on provision (init job or post-ready hook).

---

## 4. Cross-cutting concerns

**GitOps ordering / sync waves.** Respect existing waves: Authentik module (15),
frontend auth (17), session HTTPRoute (18), XRD/composition/RBAC (25). Backend
Deployment should land after the XRD is established. Use Argo sync waves +
Crossplane readiness so the backend doesn't start creating claims before the
composition is ready.

**Security posture.** Edge does authN (OIDC) and coarse authZ; the backend owns
fine-grained "owns this session" authZ. Keep the gateway→backend ext-authz path
on the cluster network with NetworkPolicy. The shell pod already runs
`automountServiceAccountToken: false` and mounts only the vcluster kubeconfig —
preserve that.

**Don't regress the post-mortem fix.** The `garage-loki-buckets` orphan
configuration is currently live-but-not-in-Git (open action item). Session
cleanup design should adopt the same lesson by default: ephemeral children use
delete policies that can't deadlock a CRD.

---

## 5. Definition of done (MVP)

A user signs in at `kubesandbox.com`, creates a `standard` session, watches it
go `Ready`, clicks **Open terminal**, and gets a working `kubectl`-enabled ttyd
against their private vcluster — and **cannot** reach anyone else's session.
Sessions auto-expire at their TTL and clean up without wedging any CRD.

---

## 6. Resolved decisions

All prior open questions are resolved (see **Locked decisions** at the top):
Option A ext-authz, 1 tenant = 1 user, backend Go source in repo-root `backend/`
built into the `kubesandbox-charts/kubesandbox-backend` chart, `client-go`
dynamic client for claim CRUD, G1 identity via Envoy-forwarded headers (backend
JWT deferred to G4), path-based `/s/{id}` routing with opaque `{id}`, and
TTL/cleanup in the backend.

### Follow-on questions surfaced by these choices
1. ~~**`{id}` format**~~ — **resolved (revised):** the backend derives a
   **deterministic per-owner claim name** `s-{sha256(owner)[:16]}`; the public
   id is `{namespace}-{name}`, so it is PII-free (hashed) *and* exactly the
   path the Crossplane HTTPRoute / ttyd base path already use. Determinism is
   deliberate: it makes the API server enforce **one sandbox per user**
   atomically (duplicate create → `AlreadyExists`). No separate id field
   needed.
2. ~~**ttyd base-path mechanism**~~ — **resolved:** chose `ttyd -b /s/{id}`
   (no Envoy rewrite). The base-path flag avoids trailing-slash / relative-URL
   fragility and keeps the gateway config simple (the HTTPRoute forwards the
   prefix unchanged). Requires the ttyd build to support `-b` (ttyd ≥ 1.6).
3. **When 1-tenant-per-user changes** — if tenants later become teams, `tenantRef`
   becomes an Authentik group and the `/authz` check widens from `ownerRef == sub`
   to "sub ∈ tenant." Keep `tenantRef` distinct in code now to ease that.
