# KubeSandbox Backend — Changelog

Historical revision log for the backend + auth build-out. This is a record of
*how* the current state (documented in `docs/04-backend-handoff.md`) was
reached — not itself required reading to understand where the project is
today. Newest first.

---

## rev 14 — 2026-07-01 — SPA auth: refresh tokens replace broken cross-site silent renew

Fixed two live SPA auth bugs, both rooted in the same design flaw: docs/06
§4.1's "in-memory tokens + iframe silent renew" posture doesn't work when the
app (`kubesandbox.com`) and Authentik (`auth.jeremymr.dev`) are **cross-site**.
Authentik's `SameSite=Lax` session cookie is never sent inside the hidden
`prompt=none` iframe (and Safari/Chrome block third-party cookies outright),
so `signinSilent()` always ends in `login_required`.

Symptoms fixed:
1. **Every page refresh logged the user out** — reload started with no
   in-memory user, the iframe renew failed as above, and `ProtectedRoute`
   bounced to `/`.
2. **Landing page "Sign in" button disabled for ~5 s** — for signed-out
   visitors, `AuthProvider` ran the same doomed `signinSilent()`: full
   Authentik round trip + booting the entire SPA bundle inside the hidden
   iframe at `/auth/callback` before the failure relayed back and `loading`
   flipped false.

New token posture (revises docs/06 §4.1/§5):
- **`offline_access` scope + refresh tokens.** Renewal is now a direct fetch
  to the token endpoint — no iframe, no third-party cookies. Requires the
  `offline_access` scope mapping on the Authentik provider (see Terraform
  change below).
- **OIDC user stored in `sessionStorage`** (was `InMemoryWebStorage`) —
  per-tab, cleared on tab close, survives refresh; still never `localStorage`.
- **`signinSilent()` only attempted when a stored user exists** (it carries
  the refresh token). No stored user ⇒ signed out immediately — this is what
  un-sticks the landing button. Applied in both `AuthProvider` startup and
  `getAccessToken()`.

Dead code removed with the iframe flow: `CallbackPage`'s
`signinSilentCallback()` relay-to-parent branch, `AuthProvider`'s
`isSilentRenewIframe` guard, and the unused `getUser()` export in `lib/auth.ts`.

Terraform (Crossplane Workspace): the frontend provider now sets
`property_mappings` explicitly via an `authentik_property_mapping_provider_scope`
data source — the default `openid`/`email`/`profile` mappings (previously
auto-attached by Authentik when the field was omitted) are pinned, plus
`offline_access`. Without the mapping Authentik silently ignores the scope and
issues no refresh token.

Rollout: sync frontend pre-resources (Workspace re-applies) → rebuild/deploy
the SPA image → existing sessions have no refresh token, so sign out/in once.
`tsc --noEmit` + `vite build` clean; not yet verified live.

Files:
- `kubesandbox/frontend/src/lib/auth.ts` — `sessionStorage` userStore;
  renew-only-if-user-exists in `getAccessToken()`; dropped unused `getUser()`.
- `kubesandbox/frontend/src/config.ts` — scope += `offline_access`.
- `kubesandbox/frontend/src/context/AuthProvider.tsx` — skip silent sign-in
  when no stored user; removed iframe guard.
- `kubesandbox/frontend/src/pages/CallbackPage.tsx` — removed iframe relay branch.
- `GitOps-Homelab/operators-helm/operators/kubesandbox-frontend/pre-resources/templates/kubesandbox-frontend-auth.yaml`
  — explicit `property_mappings` incl. `offline_access`.

---

## rev 13 — 2026-07-01 — G5 frontend design + SPA→/api auth enablement

Started **G5 (frontend SPA)** and unblocked the SPA→`/api` bearer path end to
end. The `/api` JWT policy now trusts the SPA's public client, and the frontend
Authentik provider was fixed to actually publish a JWKS.

Frontend design + scaffold:
- **`docs/06-frontend-architecture.md`** (new) — living design for G5: scope,
  lean stack (React 19 + Vite + TS + Tailwind/shadcn + TanStack Query + Zod +
  oidc-client-ts), route map, the two auth surfaces (`/api` bearer vs. `/s/{id}`
  backend-owned OIDC), SSE via fetch-stream, runtime-config shim, and open
  action items. Recommends **SPA Auth-Code+PKCE (public client, in-memory
  token)** over an edge-OIDC+BFF, keeping the backend stateless.
- **`frontend/`** (new) — Vite + React 19 + TS SPA scaffold: typed Zod-validated
  API client (bearer + SSE fetch-stream with polling fallback), PKCE auth,
  TanStack Query hooks, pages (landing/callback/dashboard/detail/terminal),
  nginx Dockerfile (`:8080`, `/health/{livez,readyz}`), and a `docker-entrypoint.sh`
  runtime-config shim so one image promotes across envs. `tsc --noEmit` +
  `vite build` clean.

Two auth fixes that unblock a valid SPA token on `/api`:
1. **Multi-issuer JWT trust.** The `/api` JWT `SecurityPolicy` trusted only the
   `kubesandbox-backend` issuer, so tokens minted for the SPA's
   `kubesandbox-frontend` client (different `iss`) 401'd. Added an
   `additionalProviders` list (Envoy accepts a token matching ANY provider) and
   populated it in prod with the frontend issuer + JWKS + `claimToHeaders`.
   Verified live: `SecurityPolicy` `kubesandbox-backend-helm-jwt` renders both
   providers and is `Accepted`.
2. **Frontend provider published an empty JWKS.** After #1, `/api` still 401'd
   with `Jwks remote fetch is failed`. Live cluster introspection (exec in
   `envoy-gateway-system`) showed `…/kubesandbox-frontend/jwks/` → `{}` while
   `…/kubesandbox-backend/jwks/` returned an RS256 key — DNS/egress/TLS all fine.
   Root cause: the `kubesandbox-frontend` Terraform Workspace never set a
   `signing_key`, so Authentik published no JWKS (HS256 fallback). Fixed by
   mirroring the backend provider: added a `signing_key_name` var + a
   `authentik_certificate_key_pair` data source, and set `signing_key`,
   `issuer_mode = "per_provider"`, and `sub_mode = "hashed_user_id"` on the
   frontend provider (the last keeps `sub` == `ownerRef` consistent across
   `/api` and `/s/{id}`).

Rollout for #2: sync the frontend pre-resources app → Crossplane re-applies the
Workspace → confirm `…/kubesandbox-frontend/jwks/` returns `{"keys":[…]}` → wait
out the 300s Envoy JWKS cache (or restart the kubesandbox proxy pod) → re-login
in the SPA for a fresh RS256 token. Not yet verified live (pending sync).

Files:
- `kubesandbox/docs/06-frontend-architecture.md` — new design doc.
- `kubesandbox/frontend/**` — new SPA scaffold.
- `kubesandbox/kubesandbox-charts/kubesandbox-backend/templates/securitypolicy-api.yaml`
  — `additionalProviders` range on the JWT policy.
- `kubesandbox/kubesandbox-charts/kubesandbox-backend/values.yaml` —
  `authentication.jwt.additionalProviders` (default `[]`, documented).
- `GitOps-Homelab/operators-helm/operators/kubesandbox-backend/values/chart/values-prd.yaml`
  — trust the `kubesandbox-frontend` issuer/JWKS on `/api`.
- `GitOps-Homelab/operators-helm/operators/kubesandbox-frontend/pre-resources/templates/kubesandbox-frontend-auth.yaml`
  — add signing key + `issuer_mode`/`sub_mode` so the provider publishes a JWKS.

---

## rev 12 — 2026-07-01 — Security hardening: backend RBAC scope + OAuth login-CSRF fix

Two gaps found during a security/architecture review of the live G1+G2 build,
fixed and verified same-day. Neither changes any user-visible behavior —
`go build`/`go vet`/`go test ./...` and `helm lint`/`helm template` all pass
unchanged for the existing suite, plus new tests added for the CSRF fix.

1. **Backend RBAC was cluster-wide for no reason.** The backend's `ClusterRole`
   (bound via `ClusterRoleBinding`) granted `secrets: get,list` and
   `namespaces: get,list` across the *entire* cluster, on top of
   `kubesandboxsessions` access. Grep confirmed the Go code never calls the
   Secrets or Namespaces API — those two grants were vestigial. Since the
   backend ServiceAccount is the most exposed component in the system (it
   terminates OIDC, holds `SESSION_SECRET`, and is reachable from the gateway),
   a compromise would have let an attacker read every Secret in every
   namespace on the management cluster. Fixed by converting to a
   namespace-scoped `Role`/`RoleBinding` in `.Values.config.namespace`
   (matching exactly what `SessionService` actually uses — it always calls
   `client.Resource(GVR).Namespace(cfg.Namespace)`) and dropping the
   `secrets`/`namespaces` rules entirely. Verified with `helm template` that
   the cross-namespace case still works (backend Deployment in one namespace,
   `Role`/`RoleBinding` in another via the standard subject-namespace pattern).
   The Crossplane aggregation `ClusterRole` and the sweep CronJob's
   `ClusterRole` (which legitimately needs cluster scope — `namespaces` is a
   cluster-scoped resource type) were left untouched.

2. **OIDC/PKCE `state` wasn't bound to the initiating browser (login CSRF).**
   The signed `state` parameter (code_verifier + original URL + expiry) was
   fully self-contained and portable. An attacker could complete their own
   Authentik login, capture their own valid `code`+`state` pair, and get a
   victim to click a crafted link to `/oauth2/callback` — silently logging the
   victim's browser in as the attacker (classic OAuth login CSRF, RFC 6749
   §10.12). Fixed by adding a `Nonce` field to `StateClaims`: `/authz`'s
   `redirectToLogin` now generates a random nonce, signs it into the state,
   and sets it as an `HttpOnly`/`Secure`/`SameSite=Lax` cookie alongside the
   redirect; `/oauth2/callback` now requires the cookie's value to match the
   nonce in the verified state (constant-time compare) before exchanging the
   code, and clears the cookie on every attempt. Added test coverage:
   missing/mismatched nonce rejected before token exchange is attempted,
   matching nonce completes login end-to-end (fake token endpoint), nonce
   cookie cleared on both success and failure, and the redirect response's
   cookie/state nonce verified to actually match.

Also updated `kubesandbox_architecture.excalidraw` to match the as-built
system: split the old single "SecurityPolicy (OIDC/Ext-Authz)" box into the
real two-policy model (ext-authz-only on `/s/{id}` vs. JWT bearer on `/api`),
noted the frontend SPA still isn't built, added the Sweep CronJob (dryRun) to
the cluster group, and added a dated status footer summarizing G1–G5 state
plus these two fixes.

Files:
- `kubesandbox-charts/kubesandbox-backend/templates/clusterrole.yaml` —
  `ClusterRole` → namespace-scoped `Role`; dropped `secrets`/`namespaces` rules.
- `kubesandbox-charts/kubesandbox-backend/templates/clusterrolebinding.yaml` —
  `ClusterRoleBinding` → `RoleBinding` in `.Values.config.namespace`.
- `backend/internal/auth/session.go` — `StateClaims.Nonce` field.
- `backend/internal/auth/oidc.go` — `GenerateNonce()` (factored out of
  `GenerateCodeVerifier()` via a shared `randomURLSafeString` helper).
- `backend/internal/api/handlers/authz.go` — `redirectToLogin` generates and
  cookies the nonce; new `oauthNonceCookieName`/`oauthNonceMaxAge` constants.
- `backend/internal/api/handlers/auth.go` — `Callback` verifies the nonce
  cookie against the state (constant-time) before code exchange; new
  `clearNonceCookie` helper.
- `backend/internal/api/handlers/auth_test.go` — new: nonce
  missing/mismatched/matching/cleared test cases.
- `backend/internal/api/handlers/authz_test.go` — new
  `TestAuthzLoginRedirectSetsMatchingNonceCookie`.
- `kubesandbox_architecture.excalidraw` — updated to reflect as-built G1–G5
  state and both fixes above.

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
