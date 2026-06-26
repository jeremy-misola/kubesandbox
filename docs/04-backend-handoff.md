# KubeSandbox — Backend Handoff

**Status:** active handoff
**Audience:** whoever picks up the backend next (incl. future me)
**Last updated:** 2026-06-26 (rev 2 — G4 Authentik client + JWT policy added)
**Related:** [`01-backend-architecture.md`](./01-backend-architecture.md) · [`02-auth-design.md`](./02-auth-design.md) · [`03-implementation-plan.md`](./03-implementation-plan.md)

---

## 1. Where we are

We are in **Phase 1 (G1 — backend control service)** of the implementation plan,
with two adjacent fixes pulled forward from Phase 2 (G2 routing/auth groundwork).

The backend Go service now exists end-to-end as source, the session routing has
been converted to path-based, and the identity plumbing (JWT → headers) has been
scaffolded but is **not yet enabled**. Nothing in this batch has been deployed —
it is all code/manifests in the repo awaiting `go mod tidy`, image build, and an
ArgoCD sync.

**One-line status (rev 2):** G1 code + its identity source are now both in place
in Git — the Authentik backend OIDC client is defined (Crossplane Workspace) and
the JWT `SecurityPolicy` is enabled with real issuer/JWKS in the prod/dev
overrides. What remains before G1 runs is purely *apply + verify*: build/push the
image, let ArgoCD sync the prereqs (client secret) and the chart, then confirm
`X-User-*` headers actually arrive (§3).

### Gap scorecard (vs. plan §1)

| Gap | Plan status | Now | Notes |
|---|---|---|---|
| G1 Backend control service | P0, not built | **Code complete, not deployed** | Needs `go mod tidy`, image, deploy. |
| G2 Per-session ownership authz | P0, not built | **Not started** | `/authz` endpoint not written (scope was G1-only). |
| G2b Path-based routing | "done" in doc, but composition wasn't | **Actually done now** | Composition + ttyd switched this session. |
| G3 TTL enforcement | P0, not built | **Not started** | Config var wired, loop not implemented. |
| G4 Backend Authentik client + token validation | P1, not built | **Defined in Git, not yet applied** | Authentik backend client (Workspace) created; JWT policy enabled w/ issuer+JWKS in prd/dev overrides. Needs ArgoCD sync + header verify. |
| G5 Frontend SPA | P1, scaffold | **Unchanged** | Must attach bearer tokens to `/api` (see §4). |
| G6 Tenant/quota model | P1, partial | **Partial** | Per-user cap implemented in backend; profiles→resources implemented. |
| G7 Observability | P2 | **Not started** | |
| G8 Starter labs | P2 | **Not started** | |

---

## 2. What was done this session

### 2.1 Backend control service (G1) — new, at `backend/`

A Go module (`github.com/jeremy-misola/kubesandbox/backend`, Go 1.25, Gin +
client-go dynamic client) implementing the session control API. Layout:

```
backend/
├── Dockerfile                      # distroless, non-root; CI builds this
├── go.mod
├── cmd/server/main.go              # entrypoint, graceful shutdown
└── internal/
    ├── config/config.go            # env config (chart-injected)
    ├── models/session.go           # DTOs, GVR, profile→resources map
    ├── kubernetes/
    │   ├── client.go               # in-cluster + kubeconfig fallback
    │   └── sessions.go             # claim CRUD, opaque naming, watch
    └── api/
        ├── router.go               # routes (/health root, /api group)
        ├── middleware/identity.go  # X-User-* → Identity
        └── handlers/               # sessions, health, sse
```

Implemented behavior:

- **Endpoints:** `POST /api/sessions`, `GET /api/sessions`,
  `GET /api/sessions/{id}`, `DELETE /api/sessions/{id}`,
  `GET /api/sessions/{id}/events` (SSE), and `GET /health` + `/healthz` at root
  (served at root so the kubelet reaches them directly, bypassing the gateway).
- **Identity (G1):** reads `X-User-Id` (preferred) / `X-User-Email` from headers;
  rejects requests with no identity (401). `tenantRef = ownerRef = subject`.
- **Claim CRUD** via the dynamic client against
  `platform.kubesandbox.com/v1alpha1`. Mints opaque names `s-xxxxxxxx`; public
  id = `{namespace}-{name}`; session URL = `{PublicBaseURL}/s/{id}`.
- **Ownership scoping:** list/get/delete/events only return the caller's
  sessions; unknown/unowned/malformed ids all 404 (no existence leak). Owner is
  selected via a hashed label `kubesandbox.com/owner` and double-checked against
  `spec.ownerRef`.
- **Per-user concurrency cap** (`MAX_SESSIONS_PER_USER`, default 3 → 429).
- **Profiles → resources:** starter `250m/256Mi`, standard `500m/512Mi`,
  advanced `1/1Gi`; TTL clamped to 15–1440 (default 60).

> **Not built (deliberately, scope was G1-only):** the `/authz` ext-authz
> endpoint (G2) and the in-process TTL cleanup loop (G3). `TTL_CLEANUP_INTERVAL`
> is read into config but no loop runs.

### 2.2 Path-based routing (G2b) — fixed in the composition

The docs claimed G2b was done, but the composition still emitted a **host-based**
route. Fixed in
`kubesandbox-charts/kubesandbox-backend/templates/kubesandbox-session-composition.yaml`:

- **HTTPRoute** now matches host `kubesandbox.com` + **path prefix
  `/s/{ns}-{name}`** (was `{ns}-{name}.kubesandbox.com`).
- **ttyd** now runs with a `BASE_PATH` env (`/s/{ns}-{name}`) and
  `ttyd -W -b "$BASE_PATH" -p 8080 sh`, so it serves under the route prefix with
  no rewrite. Requires ttyd ≥ 1.6 (the `-b` flag).

This means the single apex `kubesandbox.com` cert covers sessions; the
`*.kubesandbox.com` wildcard is no longer needed.

### 2.3 Owner/tenant label-safety — fixed in the composition

The composition stamped raw `spec.ownerRef` / `spec.tenantRef` into namespace
**labels**. Label values can't contain `@`, so an email owner would fail
validation and break session creation. These are now namespace **annotations**
(`kubesandbox.com/owner-ref`, `kubesandbox.com/tenant-ref`); `profile` stays a
label (safe enum). The backend's own claim label was already hashed, so it was
fine.

### 2.4 JWT identity source (G4 groundwork) — scaffolded, default OFF

New `templates/securitypolicy-api.yaml` + `authentication.*` values: a JWT
`SecurityPolicy` targeting the backend `/api` HTTPRoute that validates Authentik
tokens and maps claims to headers via `claimToHeaders`
(`sub→X-User-Id`, `email→X-User-Email`, `name→X-User-Name`,
`groups→X-User-Groups`). Deployment env extended with `USER_ID_HEADER`,
`PUBLIC_BASE_URL`, `MAX_SESSIONS_PER_USER`.

**Disabled by default** (`authentication.enabled: false`) so an ArgoCD sync
won't touch the live gateway until `issuer`/`jwksUri` are filled in.

### 2.5 Authentik backend client + JWT policy enablement (G4) — done rev 2

Closes next-steps items 2 and 3. All changes live in the **`GitOps-Homelab`**
repo (the chart default in `kubesandbox-charts` stays `enabled: false`; prod/dev
turn it on via overrides).

- **Authentik backend OIDC client** — new Crossplane Terraform `Workspace`
  `operators-helm/operators/kubesandbox-backend/pre-resources/templates/kubesandbox-backend-auth.yaml`,
  mirroring `kubesandbox-frontend-auth.yaml`. Creates an `authentik_provider_oauth2`
  + `authentik_application` with `client_id`/slug `kubesandbox-backend` and writes
  **`kubesandbox-backend-client-secret`** to the `kubesandbox` namespace. New
  pre-resources `Chart.yaml` + `values/pre-resources/values-{prd,dev}.yaml`
  (`envSuffix`, `signingKeyName`).
- **RS256 signing key (important deviation from the frontend client).** The
  backend provider sets `signing_key` (+ `issuer_mode = "per_provider"`). The JWT
  policy validates against the provider's **JWKS** endpoint, which Authentik only
  populates when an asymmetric signing key is configured — without it, tokens are
  opaque/HS256 and JWKS is empty, so validation would fail. The key is referenced
  by name via `signingKeyName` (default `"authentik Self-signed Certificate"`);
  **confirm that cert name in your Authentik or override it.**
- **ArgoCD wiring** — set `preResources: {enabled: true, templated: true}` on the
  `kubesandbox-backend` operator entry in `operators-helm/values/values-{prd,dev}.yaml`,
  so Argo emits a `kubesandbox-backend-prereqs` Application that applies the
  Workspace (verified via `helm template` of the operators ApplicationSet).
- **JWT policy enabled with real values** — `operators-helm/operators/kubesandbox-backend/values/chart/values-{prd,dev}.yaml`
  now set `authentication.enabled: true` with per-provider issuer/JWKS:
  - prd `issuer` `https://auth.jeremymr.dev/application/o/kubesandbox-backend/`,
    `jwksUri` `…/jwks/`
  - dev `https://auth-dev.jeremymr.dev/application/o/kubesandbox-backend/` (+ `/jwks/`)
- **Verified** with `helm template`/`helm lint`: the Workspace, the rendered
  `SecurityPolicy` (issuer/JWKS/`claimToHeaders`), the dev variant, and the
  ApplicationSet prereqs Application all render cleanly.
- **BackendTLSPolicy** — *not* added. `auth.jeremymr.dev` appears publicly
  issued; add one only if Envoy can't trust the cert when fetching JWKS.

---

## 3. Key finding: identity headers are NOT being injected today

Verified against the live `prod-k3s` cluster: every OIDC `SecurityPolicy`
(`kubesandbox-frontend-helm-auth`, `code-server`, `backstage`, …) is **OIDC-only**
and emits **no `X-User-*` headers**. Envoy Gateway's OIDC (`oauth2`) filter has
no claim→header feature; `claimToHeaders` exists only under `spec.jwt`.

**Consequence:** the G1 backend, which trusts `X-User-*`, would 401 every request
until an identity source is enabled. That is what §2.4 (and the Authentik backend
client below) is for. Re-verify after enabling using either:

- echo service behind the gateway, then read forwarded headers; or
- `kubectl port-forward -n envoy-gateway-system <kubesandbox-envoy-pod> 19000:19000`
  then `curl -s localhost:19000/config_dump | grep -i x-user`.

---

## 4. Known caveats / decisions still open

1. **Bearer vs. cookie.** The JWT policy requires `Authorization: Bearer <token>`.
   The SPA currently authenticates by OIDC **cookie**, which JWT ignores. The
   frontend (G5) must attach a token to `/api` calls. Alternative: have the
   backend validate the OIDC session itself (fuller G4) — cleaner for browsers.
2. **SSE + headers.** Browser `EventSource` cannot set headers, so the SSE route
   can't carry a bearer via the native API. Consume SSE with a fetch-stream
   client that sets `Authorization`, or keep that one route cookie-based.
3. ~~**Authentik backend client doesn't exist.**~~ **Resolved (rev 2):** the
   `kubesandbox-backend` Authentik app + provider are now defined (Workspace,
   §2.5) and the JWT policy points at its real issuer/JWKS. Still needs an
   ArgoCD apply and a live header check. Open sub-item: confirm the Authentik
   **signing-key cert name** (`signingKeyName`) matches your instance.
4. **No go.sum committed.** The Dockerfile runs `go mod tidy` at build time so CI
   works, but committing `go.sum` locally enables dependency layer caching.
5. **Couldn't compile in this environment** (no Go toolchain + restricted
   network). Run `go build ./... && go vet ./...` locally before merge.

---

## 5. Next steps (in order)

### Immediate — make G1 actually run
1. ~~**Build & verify the Go module locally**~~ — **done** (`go mod tidy /
   build / vet`, `go.sum` committed).
2. ~~**Create the Authentik backend client (G4)**~~ — **done rev 2** (§2.5):
   `kubesandbox-backend-auth.yaml` Workspace, writes
   `kubesandbox-backend-client-secret`. Issuer
   `https://auth.jeremymr.dev/application/o/kubesandbox-backend/`, JWKS `…/jwks/`.
3. ~~**Fill in & enable the JWT policy**~~ — **done rev 2** (§2.5):
   `authentication.enabled: true` + issuer/jwksUri in the prd/dev chart
   overrides. `BackendTLSPolicy` not needed (public cert); revisit if JWKS fetch
   fails TLS.
4. **← NEXT. Deploy & verify.** Build/push the image (`backend.yml`), let ArgoCD
   sync `kubesandbox-backend-prereqs` (creates the client secret) then the chart
   (after the XRD is established). Confirm `/health` is green and that `X-User-*`
   headers now arrive (§3). **Pre-flight:** verify the Authentik signing-key cert
   name matches `signingKeyName` (default `"authentik Self-signed Certificate"`),
   else the JWKS will be empty and every `/api` call 401s.
5. **Smoke test:** create a `standard` session, poll `GET /api/sessions/{id}`
   until `workspaceReady`, open `kubesandbox.com/s/{id}`, confirm ttyd + kubectl.

### Phase 2 — Session ownership authz (G2)
6. Implement the backend **`GET /authz`** endpoint: from forwarded path `/s/{id}`
   + identity, 200 iff claim `ownerRef == sub`, else 403 (404 for unknown).
7. Add the **shared session SecurityPolicy** (host `kubesandbox.com`, path `/s/`):
   OIDC (cookie) then ext-authz to the backend. Verify it runs on the **WebSocket
   upgrade** for ttyd.
8. **Negative test:** user B cannot open user A's `/s/{id}`.

### Phase 3 — TTL & safe cleanup (G3)
9. Implement the TTL loop (in-backend) deleting claims past `status.expiresAt`,
   using delete policies that can't wedge a CRD (orphan-then-sweep per the
   2026-06-24 post-mortem). Add the backstop sweep CronJob.

### Later
10. Frontend SPA (G5) incl. token attachment + fetch-based SSE.
11. Observability/alerts (G7), rate limiting; starter labs (G8).

---

## 6. Definition of done (unchanged, from plan §5)

A user signs in at `kubesandbox.com`, creates a `standard` session, watches it go
`Ready`, clicks **Open terminal**, gets a working `kubectl`-enabled ttyd against
their private vcluster — and **cannot** reach anyone else's session. Sessions
auto-expire at TTL and clean up without wedging any CRD.

---

## 7. File index (changed/added this session)

- `backend/**` — new Go service (G1).
- `kubesandbox-charts/kubesandbox-backend/templates/kubesandbox-session-composition.yaml`
  — path-based route + ttyd base path; owner/tenant → annotations.
- `kubesandbox-charts/kubesandbox-backend/templates/securitypolicy-api.yaml` — new JWT policy (off).
- `kubesandbox-charts/kubesandbox-backend/templates/deployment.yaml` — new identity/env vars.
- `kubesandbox-charts/kubesandbox-backend/values.yaml` — `authentication.*` + config additions.

**Rev 2 (G4 client + JWT enablement) — in the `GitOps-Homelab` repo:**

- `operators-helm/operators/kubesandbox-backend/pre-resources/Chart.yaml` — new pre-resources chart.
- `operators-helm/operators/kubesandbox-backend/pre-resources/templates/kubesandbox-backend-auth.yaml` — new Authentik backend client Workspace (RS256 signing key).
- `operators-helm/operators/kubesandbox-backend/values/pre-resources/values-{prd,dev}.yaml` — new (`envSuffix`, `signingKeyName`).
- `operators-helm/operators/kubesandbox-backend/values/chart/values-{prd,dev}.yaml` — enable JWT policy + issuer/JWKS.
- `operators-helm/values/values-{prd,dev}.yaml` — `preResources` enabled on the `kubesandbox-backend` operator entry.
