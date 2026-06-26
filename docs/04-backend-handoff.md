# KubeSandbox — Backend Handoff

**Status:** active handoff
**Audience:** whoever picks up the backend next (incl. future me)
**Last updated:** 2026-06-26 (rev 5 — security + lifecycle hardening: backend NetworkPolicy (anti-spoofing), the shared session SecurityPolicy chart template (OIDC + JWT + ext-authz, default-off, needs live spike), the in-backend TTL cleanup loop (G3), and a backstop sweep CronJob. Two stale doc copies consolidated. All `go build/vet/test` + `helm lint/template` green.)
**Related:** [`01-backend-architecture.md`](./01-backend-architecture.md) · [`02-auth-design.md`](./02-auth-design.md) · [`03-implementation-plan.md`](./03-implementation-plan.md)

---

## 1. Where we are

We are at the **end of Phase 1 (G1 — backend control service): deployed and
verified running on the live `prod-k3s` cluster.** The session lifecycle works
end to end (create → provision vcluster + ttyd → working in-session `kubectl` →
delete). Edge auth/routing is now correct after fixing two blockers found during
live testing.

**One-line status (rev 3):** G1 is deployed and works. Live-tested against
`prod-k3s`: backend `/health` green, API CRUD verified (401 without identity, 201
create, 204 delete, ownership scoping), the Crossplane composition provisions a
private vcluster + ttyd with a working `kubectl`, and the browser terminal at
`kubesandbox.com/s/{id}` returns the ttyd page (HTTP 200). Two routing blockers
found and resolved this session (§2.6): **(A)** the frontend's older protected
HTTPRoute was shadowing `/api` and swallowing it into the frontend OIDC flow —
fixed by removing `/api` from the frontend chart; **(B)** the deployed Composition
was stale (ttyd ran without its `-b $BASE_PATH`, so `/s/{id}` 404'd) — resolved
when the bad Composition (owned by the now-deleted frontend ArgoCD app) was pruned
and re-applied from the backend chart with the correct `-b` + path route. **The
only thing not yet verified end-to-end is JWT with a *real* Authentik bearer
token** — the policy is active and rejects missing/invalid tokens, but a valid
token requires a browser login (§3, §5).

### Gap scorecard (vs. plan §1)

| Gap | Plan status | Now | Notes |
|---|---|---|---|
| G1 Backend control service | P0, not built | **Deployed & verified on prod-k3s** | API CRUD, ownership scoping, quota, composition provisioning all live-tested OK. |
| G2 Per-session ownership authz | P0, not built | **Backend endpoint done; policy authored (default-off), needs live spike** | `/authz` endpoint implemented + unit-tested (rev 4). The shared session `SecurityPolicy` now exists as a chart template (rev 5, `securitypolicy-session.yaml`, OIDC + JWT claimToHeaders + ext-authz, targets session routes by label). DEFAULT-OFF and **not yet verified against a live gateway** — see the VERIFICATION REQUIRED block in that file. |
| Backend NetworkPolicy (G1 hardening) | implied, not built | **Added (rev 5), default-on** | `networkpolicy.yaml` restricts backend ingress to `envoy-gateway-system`, so in-cluster pods can't spoof `X-User-*` on `/api` or `/authz`. Mirrors the working per-session NetworkPolicy. |
| G2b Path-based routing | "done" in doc, but composition wasn't | **Done & verified live** | `/s/{id}` returns ttyd 200 through the gateway; ttyd serves under `-b /s/{id}`. |
| G3 TTL enforcement | P0, not built | **Implemented (rev 5), not live-tested** | In-backend TTL loop (`cleanup.go`) deletes expired claims (status.expiresAt, falling back to creation+ttlMinutes since nothing populates expiresAt yet), background delete so a stuck finalizer can't block it. Backstop sweep CronJob deletes orphaned session namespaces (default-on, `dryRun: true`). Unit-tested; not yet run on a cluster. |
| G4 Backend Authentik client + token validation | P1, not built | **Deployed; JWT enforcing, valid-token path unverified** | Client secret present, JWT `SecurityPolicy` Accepted by Envoy, rejects missing/invalid tokens (401). Valid-bearer + `X-User-*` injection still needs a browser login to confirm. |
| G5 Frontend SPA | P1, scaffold | **ArgoCD app deleted this session** | Frontend routes pruned; must be recreated from chart ≥0.1.8. Still must attach bearer tokens to `/api` (see §4). |
| G6 Tenant/quota model | P1, partial | **Partial** | Per-user cap implemented in backend; profiles→resources implemented & verified (starter 250m/256Mi tested). |
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

### 2.6 Deployed to prod-k3s + live end-to-end testing (rev 3) — done

The backend image was built/pushed and ArgoCD synced. Everything below was
verified against the live `prod-k3s` cluster (namespace `kubesandbox`).

**What's confirmed working:**

- **Health:** `kubesandbox-backend-helm` pod Running 1/1; `/health` and `/healthz`
  return 200 (kubelet probes green).
- **API logic** (tested by hitting the backend Service directly with manual
  `X-User-*` headers, bypassing the gateway): no identity → `401`; with
  `X-User-Id` → `200`; `POST /api/sessions {profile}` → `201`;
  `DELETE` → `204`; list/get scope to the caller only.
- **Composition / provisioning:** creating a session emits all 7 managed
  resources (vcluster Helm `Release` + `Object`s for namespace, resourcequota,
  networkpolicy, shell pod, shell service, httproute). The vcluster cold-boots in
  ~4–5 min (the shell pod correctly blocks on the `vc-<...>-vcluster` kubeconfig
  secret until then). Once up, **`kubectl` inside the shell works against the
  private vcluster** (listed the node, created+read a ConfigMap). The session
  `NetworkPolicy` correctly isolates the namespace (only the gateway can reach
  the shell on 8080).
- **Profiles:** `starter` → 250m/256Mi and `standard` → 500m/512Mi both applied
  correctly to the shell pod.

**Two routing blockers were found and fixed:**

- **Blocker A — `/api` shadowed by the frontend OIDC route.** Two HTTPRoutes on
  host `kubesandbox.com` both matched `PathPrefix: /api` — the backend's
  (JWT-guarded) route and an **older** `kubesandbox-frontend-helm-protected` rule
  pointing at the frontend service under OIDC. With equal path specificity, the
  Gateway API tie-break picks the **oldest** route, so every `/api` call hit the
  frontend and got a `302` to Authentik (`client_id=kubesandbox-frontend`) — it
  never reached the backend or its JWT policy.
  **Fix:** removed the `/api` entry from `kubesandbox-charts/frontend/values.yaml`
  `protectedPaths` (now only `/terminal`), with an explanatory comment; bumped the
  frontend `Chart.yaml` to `0.1.8`. After the frontend routes were removed,
  `/api` now correctly hits the JWT filter: missing token → `401 "Jwt is
  missing"`, malformed token → `401`, **no** Authentik redirect.
- **Blocker B — stale Composition: `/s/{id}` 404'd.** The deployed shell pod ran
  `ttyd -W -p 8080 sh` *without* the `-b $BASE_PATH` flag, so ttyd served at `/`
  while the gateway forwarded `/s/{id}/...` unchanged → ttyd's own 404. The repo
  composition was correct, but the **live** Composition was stale (it was tracked
  by the frontend ArgoCD app, not the backend). When the frontend ArgoCD app was
  deleted, the stale Composition was pruned and re-applied from the backend chart
  with the fix. **Verified:** a fresh session's shell pod now renders
  `ttyd -W -b "$BASE_PATH"` with `BASE_PATH=/s/{id}`, and
  `GET https://kubesandbox.com/s/{id}/` returns **HTTP 200** with the ttyd
  terminal page (`<title>ttyd - Terminal</title>`); `/s/{id}/token` also 200.

> **Ownership note for whoever re-adds the frontend:** the `kubesandbox-session`
> Composition must be owned by **one** ArgoCD app (the backend). Historically it
> carried `tracking-id: kubesandbox-frontend-helm`, which is why deleting the
> frontend app pruned it. Keep it in the backend chart only.

**Test hygiene:** all test sessions and the throwaway `curl` pod were deleted. One
pre-existing stale session, `s-5798f4b1` (owner `you@example.com`), was left in
place stuck in a namespace reconcile error — delete it
(`kubectl delete kubesandboxsession s-5798f4b1 -n playground`).

### 2.7 Security + lifecycle hardening (rev 5) — addresses the review findings

Closes the review's items 1–3, 5–6. All in `kubesandbox-charts/kubesandbox-backend`
and `backend/`; chart bumped `0.1.6 → 0.1.7`. Verified with `go build/vet/test`
(all green, new unit tests pass) and `helm lint` + `helm template` (renders clean,
both SecurityPolicies render without a name clash, the session-route label is
matched by the policy selector).

- **Backend NetworkPolicy (review item 1, the biggest hole).**
  `templates/networkpolicy.yaml` (+ `networkPolicy.*` values, **default-on**)
  restricts backend ingress to the `envoy-gateway-system` namespace. The backend
  trusts `X-User-*`; without this, any pod could spoof identity on `/api` or
  `/authz`. Mirrors the per-session NetworkPolicy already proven on prod-k3s.
- **Shared session SecurityPolicy (review item 2 — the G2 keystone).**
  `templates/securitypolicy-session.yaml` (+ `sessionAuth.*` values,
  **default-off**). One policy for all sessions via `spec.targetSelectors`
  matching `kubesandbox.com/session-route: "true"` (now stamped on the
  composition's shell HTTPRoute). Chain: **OIDC** (cookie, rides the WS upgrade)
  → **JWT** `claimToHeaders` (gives `/authz` its `X-User-*`; the OIDC filter
  can't emit claim headers) → **ext-authz** to the backend `/authz`.
- **VERIFICATION REQUIRED (review item 3 — the live spike).** The policy is wired
  to the best-understood Envoy Gateway contract but is **unverified against a live
  gateway** and default-off so an ArgoCD sync can't touch the live gateway. Before
  enabling, confirm (on dev): the v1.8 field names (`forwardAccessToken`,
  `headersToExtAuth`, `extAuth.http.*`); that the forwarded OIDC token is a JWT the
  `jwt` provider can validate (needs the backend client's RS256 signing key); that
  ext-authz runs **on the ttyd WebSocket upgrade** and `X-User-*` + the `/s/{id}`
  path reach `/authz`; and the negative test (user B → 403). Full checklist is in
  the template's header.
- **TTL cleanup loop (review item 5 / G3).** `backend/internal/kubernetes/cleanup.go`
  — `TTLController` runs every `TTL_CLEANUP_INTERVAL`, deleting claims past expiry.
  Expiry prefers `status.expiresAt` but **falls back to creationTimestamp +
  spec.ttlMinutes** because nothing populates `status.expiresAt` today. Deletes the
  **claim only** with **background propagation** (a stuck finalizer can't block the
  loop), skips already-terminating claims, and continues past individual failures
  (post-mortem-safe). Wired into `main.go` with its own cancellable context; unit
  tested (`cleanup_test.go`).
- **Backstop sweep CronJob (G3).** `templates/sweep-cronjob.yaml` (+ `sweep.*`
  values, **default-on but `dryRun: true`**). Dedicated least-privilege SA/Role
  (namespaces get/list/delete + kubesandboxsessions get/list). Deletes session
  namespaces that are managed, older than `maxAgeHours` (24h), and have **no
  surviving claim** — a live claim is always left to Crossplane. Flip `dryRun:
  false` after reviewing a run's "WOULD delete" log.
- **Doc consolidation (review item 6).** The stale `GitOps-Homelab/docs/kubesandbox`
  copies (older, still referencing the abandoned `operators-helm/...` backend path)
  were replaced with pointer stubs to this canonical set.

> **Not changed (per your choice):** `/api` keeps **JWT bearer** auth — the SPA
> must attach a token and SSE must use a fetch-stream client (caveats §4.1–4.2
> stand). The cookie-validation alternative was explicitly deferred.

---

## 3. Identity headers: JWT policy is now live; valid-token path still unverified

**Background (still true):** Envoy Gateway's OIDC (`oauth2`) filter has no
claim→header feature — `claimToHeaders` exists only under `spec.jwt`. So the
backend's `X-User-*` identity can only come from a **JWT** `SecurityPolicy`, not
from the OIDC cookie flow the SPA uses.

**Now (rev 3):** the JWT `SecurityPolicy` (`kubesandbox-backend-helm-jwt`) is
deployed and **Accepted** by the gateway, targeting the backend `/api` HTTPRoute,
with `claimToHeaders` (`sub→X-User-Id`, `email→X-User-Email`, `name→X-User-Name`,
`groups→X-User-Groups`) and the real Authentik issuer/JWKS. Live-tested: `/api`
with no token → `401 "Jwt is missing"`; malformed token → `401`. So the filter is
in the request path and enforcing.

**Still to confirm:** that a **valid** Authentik bearer yields `200` *and* the
`X-User-*` headers actually reach the backend. This needs a real token (browser
login to `kubesandbox-backend`), which couldn't be done headlessly. Verify with:

- get a token via an Authentik OAuth flow for the `kubesandbox-backend` app, then
  `curl -H "Authorization: Bearer <token>" https://kubesandbox.com/api/sessions`
  and expect JSON (not 401/302); or
- `kubectl port-forward -n envoy-gateway-system <kubesandbox-envoy-pod> 19000:19000`
  then `curl -s localhost:19000/config_dump | grep -i x-user`.

**Pre-flight:** confirm the Authentik signing-key cert name matches
`signingKeyName` (default `"authentik Self-signed Certificate"`), else JWKS is
empty and every valid token still 401s.

---

## 4. Known caveats / decisions still open

1. **Bearer vs. cookie.** The JWT policy requires `Authorization: Bearer <token>`.
   The SPA currently authenticates by OIDC **cookie**, which JWT ignores. The
   frontend (G5) must attach a token to `/api` calls. Alternative: have the
   backend validate the OIDC session itself (fuller G4) — cleaner for browsers.
2. **SSE + headers.** Browser `EventSource` cannot set headers, so the SSE route
   can't carry a bearer via the native API. Consume SSE with a fetch-stream
   client that sets `Authorization`, or keep that one route cookie-based.
3. ~~**Authentik backend client doesn't exist.**~~ **Applied & enforcing
   (rev 3):** the `kubesandbox-backend` Authentik app + provider exist, the
   client secret is present, and the JWT policy is Accepted and rejecting
   missing/invalid tokens. Remaining: a **valid-token** check (§3) and confirming
   the **signing-key cert name** (`signingKeyName`) matches your instance.
4. **Frontend ArgoCD app was deleted (rev 3).** The SPA and its routes are gone.
   Recreate it from the frontend chart **≥ 0.1.8** (the version that no longer
   claims `/api`). Recreating from 0.1.7 reintroduces Blocker A.
5. **`kubesandbox-backend-helm` ArgoCD app was OutOfSync** at end of session —
   sync it to settle the drift that caused the stale Composition.
6. **No go.sum committed.** The Dockerfile runs `go mod tidy` at build time so CI
   works, but committing `go.sum` locally enables dependency layer caching.
7. **Couldn't compile in this environment** (no Go toolchain + restricted
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
4. ~~**Deploy & verify.**~~ — **done rev 3** (§2.6). Image built/pushed, ArgoCD
   synced, `/health` green, two routing blockers fixed.
5. ~~**Smoke test.**~~ — **done rev 3** (§2.6): created `starter` + `standard`
   sessions, watched them go `Ready`, `kubesandbox.com/s/{id}` returns the ttyd
   page (200), in-session `kubectl` works.

### Immediate follow-ups from rev 3 testing
6. **← NEXT. Recreate the frontend ArgoCD app** from chart **≥ 0.1.8** (without the
   `/api` rule) to bring the SPA back without re-introducing Blocker A. Then
   **sync `kubesandbox-backend-helm`** (was OutOfSync).
7. **Verify JWT with a real bearer** (§3): valid Authentik token → `/api` returns
   `200` and `X-User-*` reach the backend. Pre-flight the `signingKeyName` cert.
8. **Delete the stale session** `s-5798f4b1` (owner `you@example.com`), stuck in a
   namespace reconcile error.

### Phase 2 — Session ownership authz (G2)
9. ~~Implement the backend **`GET /authz`** endpoint~~ — **done rev 4.** From the
   forwarded path `/s/{id}` + `X-User-*` identity, returns 200 iff claim
   `ownerRef == sub`, else 403; unknown/unowned/malformed ids and any non-`/s/`
   path all 403 (no existence leak); no identity → 401; backend error → 503 (fail
   closed). Lives in `backend/internal/api/handlers/authz.go` (+ `SessionService.Authorize`),
   mounted at `/authz` and `/authz/*rest` in `router.go`. Reads the original URI
   from `X-Forwarded-Uri` / `X-Original-Uri` / `X-Envoy-Original-Path`, falling
   back to its own path. Unit-tested (`authz_test.go`, fake dynamic client);
   `go build/vet/test ./...` all green. **Still to do:** the gateway
   `SecurityPolicy` (item 10) and live verification.
10. ~~Add the **shared session SecurityPolicy**~~ — **authored rev 5**
    (`securitypolicy-session.yaml`, default-off): OIDC + JWT claimToHeaders +
    ext-authz, attached to all session routes by label. **← NEXT: run the live
    spike** to verify it against a dev gateway (field names, OIDC-token-is-a-JWT,
    ext-authz on the WS upgrade), then enable via prd/dev overrides.
11. **Negative test:** user B cannot open user A's `/s/{id}` (do this during the
    spike above).

### Phase 3 — TTL & safe cleanup (G3)
12. ~~Implement the TTL loop + backstop sweep CronJob~~ — **done rev 5**
    (`cleanup.go` + `sweep-cronjob.yaml`, unit-tested). Remaining: **live-test** on
    a cluster (create a short-TTL session, confirm it's reaped; check a sweep dry
    run's log) and then set `sweep.dryRun: false`.

### Later
13. Frontend SPA (G5) incl. token attachment + fetch-based SSE.
14. Observability/alerts (G7), rate limiting; starter labs (G8).

---

## 6. Definition of done (unchanged, from plan §5)

A user signs in at `kubesandbox.com`, creates a `standard` session, watches it go
`Ready`, clicks **Open terminal**, gets a working `kubectl`-enabled ttyd against
their private vcluster — and **cannot** reach anyone else's session. Sessions
auto-expire at TTL and clean up without wedging any CRD.

---

## 7. File index (changed/added this session)

**Rev 5 (security + lifecycle hardening):**

- `backend/internal/kubernetes/cleanup.go` (+ `cleanup_test.go`) — TTL controller (G3).
- `backend/internal/kubernetes/sessions.go` — added `listManaged` / `deleteByName` helpers.
- `backend/cmd/server/main.go` — start/stop the TTL loop.
- `kubesandbox-charts/kubesandbox-backend/templates/networkpolicy.yaml` — new (anti-spoofing, default-on).
- `kubesandbox-charts/kubesandbox-backend/templates/securitypolicy-session.yaml` — new G2 session policy (default-off, needs live spike).
- `kubesandbox-charts/kubesandbox-backend/templates/sweep-cronjob.yaml` — new backstop + scoped RBAC (default-on, dryRun).
- `kubesandbox-charts/kubesandbox-backend/templates/kubesandbox-session-composition.yaml` — `kubesandbox.com/session-route` label on the shell HTTPRoute.
- `kubesandbox-charts/kubesandbox-backend/values.yaml` — `networkPolicy.*`, `sessionAuth.*`, `sweep.*`.
- `kubesandbox-charts/kubesandbox-backend/Chart.yaml` — `0.1.6 → 0.1.7`.
- `GitOps-Homelab/docs/kubesandbox/*.md` — replaced stale copies with pointer stubs.

**Earlier this session:**

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

**Rev 3 (deploy + live testing) — in `kubesandbox-charts`:**

- `kubesandbox-charts/frontend/values.yaml` — removed `/api` from `protectedPaths` (Blocker A fix; only `/terminal` remains).
- `kubesandbox-charts/frontend/Chart.yaml` — version `0.1.7` → `0.1.8`.
- No backend source changes this rev; Blocker B was a deploy/ownership drift (the live `kubesandbox-session` Composition was stale and is now re-applied from the backend chart). The composition template in the repo was already correct.
