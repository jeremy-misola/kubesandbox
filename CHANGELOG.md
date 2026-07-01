# KubeSandbox Backend — Changelog

Historical revision log for the backend + auth build-out. This is a record of
*how* the current state (documented in `docs/04-backend-handoff.md`) was
reached — not itself required reading to understand where the project is
today. Newest first.

---

## rev 11 — 2026-07-01 — G2 enablement + prod verification

Full browser smoke test passed on prod-k3s: unauthenticated → 302 to Authentik
→ login → `/oauth2/callback` code exchange → HMAC session cookie set →
`/authz` 200 (ownership passes) → ttyd terminal loads → `kubectl` works inside
vcluster. Negative test confirmed: user B → 403. `sessionAuth.enabled: true`
is now live in the prod chart override.

Three bugs found and fixed during enablement:
1. **GitOps-Homelab composition was out of sync with the repo** — the HTTPRoute
   was still landing in the session namespace without the `session-route`
   label and without a ReferenceGrant. Fixed by porting the rev 8 changes to
   the GitOps-Homelab copy (the one that actually runs in the cluster).
2. **Backend auth/token endpoints defaulted to `{issuer}authorize/`**, wrong
   for Authentik — real endpoints are at `/application/o/authorize/` and
   `/application/o/token/`. Fixed by adding `authEndpoint`/`tokenEndpoint`
   overrides to `values-prd.yaml`.
3. **Test session ownership mismatch** — Authentik uses `sub_mode:
   hashed_user_id` so `sub` is the `uid` hash, not email. Noted for the
   frontend: use `sub` from the JWT, not email, as the identity key.

Files (in `GitOps-Homelab`):
- `operators-helm/operators/kubesandbox-backend/values/chart/values-prd.yaml`
  — `sessionAuth.enabled: false → true`; added `authEndpoint`/`tokenEndpoint` overrides.
- `operators-helm/operators/crossplane/resources/templates/kubesandbox-session-composition.yaml`
  — ported rev 8 G2 changes: `shell-httproute` namespace fixed to `kubesandbox`,
  `session-route: "true"` label, cross-namespace `backendRef.namespace` patch,
  new `shell-referencegrant` Object.

---

## rev 10 — live control-plane smoke test

Controlled smoke test against prod-k3s via port-forward, simulating the
gateway's forwarded-identity header pattern (throwaway identity, cleaned up
after).

Confirmed: `POST /api/sessions` → `201`; claim created in the correct
namespace; composite resource binding clean; vcluster Helm release installed;
session namespace provisioned with expected labels; gateway route present and
responding; `DELETE` → `204` with claim + route confirmed gone afterward.
vcluster cold-boot (~4–5 min for coredns + kubeconfig Secret publish)
confirmed normal; shell pod wait-on-Secret logic works. No stale test
resources left behind.

No file changes — verification only.

---

## rev 9 — Terraform workspace update

Wires the Terraform provisioning layer to the G2 Option B implementation.

- `kubesandbox-backend-auth.yaml` (inline Workspace): adds `hashicorp/random`
  provider; new `random_password.session_secret` (64 chars, URL-safe,
  `lifecycle { ignore_changes = all }` so it's generated once and never
  rotated — rotating it invalidates all live sessions); `allowed_redirect_uris`
  changed to `https://kubesandbox.com/oauth2/callback`; `access_code_validity`
  set to 5 minutes; new `output "session-secret"`, written to
  `kubesandbox-backend-client-secret` alongside the OIDC credentials.
- `authentik-oidc-app.yaml` (generic module): new `additional_redirect_uris`
  variable, merged into `allowed_redirect_uris` (backward-compatible).
- `kubesandbox-backend/templates/deployment.yaml` + `values.yaml`: `SESSION_SECRET`
  now reads from the same Secret as `OIDC_CLIENT_SECRET` (key `session-secret`)
  — one Secret, two keys, instead of a separate `sessionAuth.sessionSecret` block.
- `values-prd.yaml` / `values-dev.yaml`: pre-populated (but `enabled: false`)
  `sessionAuth` block with correct Authentik URLs and Secret ref.

---

## rev 8 — G2 Options A+B — backend-owned session auth

Implements the redesign from the rev 7 spike (see `docs/05-g2-spike-findings.md`).
Chart bumped `0.1.8 → 0.1.9`. `sessionAuth.enabled` stayed default-off until
the pre-flight checklist was complete (secrets present, endpoints confirmed —
completed by rev 9–11).

**Option A — session HTTPRoutes moved to the shared `kubesandbox` namespace:**
`kubesandbox-session-composition.yaml` — `shell-httproute` namespace fixed to
`kubesandbox`; cross-namespace `backendRef`; new `shell-referencegrant` Object
in the per-session namespace.

**Option B — backend owns the full OIDC flow:**
- `backend/internal/auth/session.go` — new: HMAC-SHA256 session cookie
  sign/verify (`base64url(json(claims)).base64url(hmac_sig)`, stateless).
- `backend/internal/auth/oidc.go` — new: PKCE (`GenerateCodeVerifier`,
  `CodeChallenge` S256), state JWT (`SignState`/`VerifyState`), code exchange
  (`ExchangeCode`), ID token claim parse (no JWKS validation — trusted via
  server-to-server TLS).
- `backend/internal/api/handlers/authz.go` — rewritten: cookie check → PKCE
  redirect (missing/invalid/expired) or ownership check (200/403/503).
  `IdentityMiddleware` removed from this route.
- `backend/internal/api/handlers/auth.go` — new `/oauth2/callback` handler.
- `backend/internal/api/router.go` — `/authz` drops `IdentityMiddleware`;
  `/oauth2/callback` route added (no auth).
- `backend/internal/config/config.go` — OIDC + session fields added.
- `templates/securitypolicy-session.yaml` — gutted to ext-authz only (OIDC/JWT
  blocks removed); `headersToExtAuth` forwards `cookie` + path headers.
- `templates/httproute-callback.yaml` — new: unauthenticated route for
  `/oauth2/callback`.
- `templates/deployment.yaml` / `values.yaml` — `OIDC_*`/`SESSION_*` env vars
  under `sessionAuth.enabled`; `sessionAuth` values block redesigned for Option B.

---

## rev 7 — G2 session-auth spike (live, prod-k3s)

Ran the originally-specced G2 design (one shared SecurityPolicy doing OIDC →
JWT claimToHeaders → ext-authz) against the live gateway. **It does not work on
Envoy Gateway v1.7.1**: `SecurityPolicy` attaches same-namespace only (sessions
are per-namespace), and ext-authz fires before OIDC can complete the login —
an unauthenticated request gets `401` from `/authz` instead of a login
redirect. Full write-up and the option analysis that led to rev 8's design:
[`docs/05-g2-spike-findings.md`](./docs/05-g2-spike-findings.md).

---

## rev 6 — live-tested rev 5, one bug fixed

Tested rev 5 against prod-k3s (chart 0.1.7; `sessionAuth` off, `networkPolicy`/
`sweep`/JWT on).

- **Backend NetworkPolicy — PASS.** Spoofed `X-User-Id` from `default` ns
  blocked; from `envoy-gateway-system` succeeds.
- **`/authz` ownership matrix — PASS.** owner→200, owner subpath→200,
  non-owner→403, unknown id→403, malformed id→403, non-`/s/` path→403, no
  identity→401.
- **JWT `/api` policy — Accepted** by the gateway.
- **TTL loop — running,** mechanism proven; full timed reap not watched
  end-to-end (XRD floors `ttlMinutes` at 15).
- **Sweep CronJob — bug found and fixed.** `bitnami/kubectl:1.31` no longer
  exists on Docker Hub (removed 2025) → `ErrImagePull`. Fixed: switched to
  `alpine/k8s:1.31.1` and made the sweep script's timestamp parsing portable
  across GNU and busybox `date`. Re-verified: dry-run correctly distinguished
  a planted orphan namespace from a real session; live run deleted only the
  orphan. Chart bumped `0.1.7 → 0.1.8`.

---

## rev 5 — security + lifecycle hardening

Chart bumped `0.1.6 → 0.1.7`. Verified with `go build/vet/test` and `helm
lint`/`helm template`.

- **Backend NetworkPolicy** (`networkpolicy.yaml`, default-on) — restricts
  backend ingress to `envoy-gateway-system`, closing the biggest
  identity-spoofing hole (`X-User-*` trust).
- **Shared session SecurityPolicy** (`securitypolicy-session.yaml`,
  default-off) — the original G2 design: one policy via `targetSelectors`
  matching `kubesandbox.com/session-route: "true"`, chaining OIDC → JWT
  claimToHeaders → ext-authz. (This design was later found broken in rev 7 and
  replaced in rev 8.)
- **TTL cleanup loop** (`backend/internal/kubernetes/cleanup.go`) —
  `TTLController` deletes claims past expiry (`status.expiresAt`, falling back
  to `creationTimestamp + ttlMinutes`), background delete propagation so a
  stuck finalizer can't block it, skips already-terminating claims, continues
  past individual failures. Unit-tested.
- **Backstop sweep CronJob** (`sweep-cronjob.yaml`, default-on, `dryRun:
  true`) — least-privilege SA/Role; deletes managed session namespaces older
  than `maxAgeHours` with no surviving claim.
- **Doc consolidation** — stale `GitOps-Homelab/docs/kubesandbox` copies
  replaced with pointer stubs to the canonical docs in this repo.

---

## rev 4 — `/authz` endpoint implemented

Backend `/authz` ownership-check endpoint implemented and unit-tested (G2
backend-side logic, ahead of the gateway-side wiring done in rev 5–8).

---

## rev 3 — deployed to prod-k3s + live end-to-end testing

Backend image built/pushed, ArgoCD synced, verified against live prod-k3s
(namespace `kubesandbox`).

**Confirmed working:** health probes green; API logic (401/201/204, ownership
scoping) tested by hitting the Service directly with manual `X-User-*`
headers; composition provisions all 7 managed resources; vcluster cold-boots
in ~4–5 min then `kubectl` works inside the shell; session NetworkPolicy
isolates the namespace; `starter`/`standard` profiles apply correct resources.

**Two routing blockers found and fixed:**
- **Blocker A** — `/api` was shadowed by an older
  `kubesandbox-frontend-helm-protected` HTTPRoute with equal path specificity;
  the Gateway API tie-break picks the oldest route, so every `/api` call hit
  the frontend's OIDC flow instead of the backend. Fixed by removing `/api`
  from the frontend chart's `protectedPaths` (frontend chart bumped to 0.1.8).
- **Blocker B** — the live Composition was stale (still owned by the frontend
  ArgoCD app) and ran `ttyd` without the `-b $BASE_PATH` flag, so `/s/{id}`
  404'd. Fixed by deleting the frontend ArgoCD app (pruning the stale
  Composition) and re-applying from the backend chart.

> The `kubesandbox-session` Composition must be owned by **one** ArgoCD app
> (the backend) going forward — historically it carried a frontend tracking
> ID, which is why deleting the frontend app pruned it.

Test hygiene: all test sessions and the throwaway curl pod deleted; one
pre-existing stale session (`s-5798f4b1`) flagged for cleanup (later
confirmed gone).

Files: `kubesandbox-charts/frontend/values.yaml` (removed `/api` from
`protectedPaths`), `frontend/Chart.yaml` (`0.1.7 → 0.1.8`).

---

## rev 2 — Authentik backend client + JWT policy enablement (G4)

New Crossplane Terraform `Workspace`
(`kubesandbox-backend/pre-resources/templates/kubesandbox-backend-auth.yaml`,
mirroring the frontend's) creates the `kubesandbox-backend` OIDC client and
writes `kubesandbox-backend-client-secret`. Uses an **RS256 signing key** +
`issuer_mode: per_provider` (deviation from the frontend client — needed so
Authentik populates the JWKS endpoint; without it tokens are opaque/HS256 and
JWT validation fails). ArgoCD wiring: `preResources` enabled on the
`kubesandbox-backend` operator entry. JWT policy enabled with real
issuer/JWKS values for prod and dev. Verified via `helm template`/`helm lint`.
`BackendTLSPolicy` not added — `auth.jeremymr.dev` has a publicly-trusted cert.

---

## Initial build — G1 backend control service + G2b/annotations fixes

New Go module at `backend/` (Gin + `client-go` dynamic client). Endpoints:
`POST/GET/DELETE /api/sessions`, `GET /api/sessions/{id}/events` (SSE),
`/health`+`/healthz` at root. Identity via `X-User-Id`/`X-User-Email` headers
(401 if missing). Claim CRUD against `platform.kubesandbox.com/v1alpha1` with
opaque names (`s-xxxxxxxx`). Ownership scoping via a hashed label,
double-checked against `spec.ownerRef`. Per-user concurrency cap (default 3,
429 over). Profiles → resources (starter 250m/256Mi, standard 500m/512Mi,
advanced 1/1Gi), TTL clamped 15–1440.

Also fixed in this pass:
- **Path-based routing (G2b)** — composition was emitting a host-based route
  (`{ns}-{name}.kubesandbox.com`) despite docs claiming this was done; fixed
  to path-based (`/s/{ns}-{name}`) with ttyd's `-b $BASE_PATH` flag.
- **Owner/tenant label-safety** — composition was stamping `ownerRef`/`tenantRef`
  into namespace *labels*, which can't contain `@` (breaks email owners).
  Moved to namespace *annotations*.
- **JWT identity source scaffolded** (`securitypolicy-api.yaml`, default off)
  — targets the backend `/api` route, maps claims to headers.
