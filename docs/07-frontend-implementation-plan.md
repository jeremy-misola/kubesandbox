# KubeSandbox — Frontend Implementation Plan (G5)

**Status:** in progress — scaffold built; SPA→`/api` JWT trust now wired in prod
(rev 13, see CHANGELOG) but pending live confirmation; auth-bootstrap silent
renew bug found and fixed this pass (§1.1, §5)
**Audience:** whoever finishes the frontend SPA (incl. future me)
**Last updated:** 2026-07-01
**Related:** [`01-backend-architecture.md`](./01-backend-architecture.md) · [`02-auth-design.md`](./02-auth-design.md) · [`03-implementation-plan.md`](./03-implementation-plan.md) · [`04-backend-handoff.md`](./04-backend-handoff.md) · [`06-frontend-architecture.md`](./06-frontend-architecture.md)
**Design system:** [`../.github/agents/design-principles.md`](../.github/agents/design-principles.md)

---

## 0. How to read this plan

This is a **gap-to-done** plan, not a from-scratch build guide. The frontend
scaffold described in [`06-frontend-architecture.md`](./06-frontend-architecture.md)
**already exists** under `frontend/` and **typechecks clean** (`npx tsc
--noEmit` → exit 0). The data/auth/SSE layer, all six pages, the hooks, the
Docker/nginx/runtime-config machinery, and the CI contract are in place.

What is **not** done is everything that turns "compiles" into "works against
prod-k3s and meets the §9 Definition of Done": one blocking backend/infra
change, live end-to-end verification of each flow, UX-state and error-handling
hardening against the design principles, quality gates (lint/tests), and the
deploy promotion.

Steps are grouped into phases. **Phase 0 is blocking** — its wiring is done as
of rev 13, but until the rollout is confirmed live (JWKS propagation), a
signed-in SPA's tokens may still be rejected by `/api`. Phases 1–4 verify the
happy path flow by flow. Phases 5–6 harden. Phase 7 ships. Phase 8 is deferred
polish.

Each step lists concrete **files/commands** and an **acceptance** signal. Check
boxes as you go.

---

## 1. Current state audit

### 1.1 What exists and works (leave alone unless a phase says otherwise)

| Area | Files | State |
|---|---|---|
| Data client + Zod boundary | `src/lib/api.ts`, `src/lib/schemas.ts` | Complete. Bearer on every call, error mapping, `Session`/`CreateSessionRequest` schemas match `models.Session`. |
| SSE fetch-stream | `src/lib/api.ts` (`streamSessionEvents`) | Complete. Manual `event:`/`data:` frame parsing, heartbeat-tolerant, bearer-attached. |
| OIDC (PKCE, in-memory) | `src/lib/auth.ts`, `src/context/AuthProvider.tsx`, `src/hooks/useAuth.ts`, `src/pages/CallbackPage.tsx` | Complete. `oidc-client-ts` UserManager, `InMemoryWebStorage`, `sub` exposed. Silent renew on load was fixed this pass (see note below) — previously `AuthProvider` only called `getUser()` on mount and never attempted `signinSilent()`, so every page reload showed "logged out" instantly, before any SSO check happened; `ProtectedRoute` would bounce to `/` before a renew could complete. Fixed by (a) `AuthProvider`'s mount effect now calls `signinSilent()` when no in-memory user is found (skipped when the document is itself the hidden renew iframe), and (b) `CallbackPage` now detects that iframe case (`window.self !== window.top`) and calls `userManager.signinSilentCallback()` to relay the response to the parent instead of running the top-level redirect-callback/navigate flow. |
| Server-state hooks | `src/hooks/useSessions.ts`, `src/hooks/useSessionEvents.ts` | Complete. TanStack Query keys, optimistic delete, SSE→cache, polling fallback. |
| Runtime config shim | `src/config.ts`, `index.html`, `docker-entrypoint.sh`, `.env.example` | Complete. `window.__ENV` → `import.meta.env` → default resolution. |
| Routes + pages | `src/App.tsx`, `src/pages/*` | Present, typecheck clean; **unverified against live backend.** |
| Components | `src/components/*` | Present. `StatusBadge`, `SessionCard`, `CreateSessionDialog`, `ProfilePicker`, `Layout`, `ProtectedRoute`. |
| Container/deploy | `Dockerfile`, `nginx.conf` (:8080, health, SPA fallback), `.github/workflows/frontend.yml` | Complete and matches chart probe contract. |

### 1.2 Gaps (the actual work)

| Gap | Where | Phase |
|---|---|---|
| SPA tokens 401 on `/api` — **config now wired** (multi-issuer JWT trust + frontend signing key, rev 13), but rollout isn't confirmed live: the frontend Authentik provider's JWKS fix needs the pre-resources Workspace to re-sync and the 300s Envoy JWKS cache to clear | `GitOps-Homelab` `values-prd.yaml`, `kubesandbox-frontend-auth.yaml`; chart template already supports it (`securitypolicy-api.yaml`) | **0** |
| Auth-bootstrap silent renew didn't actually run on page load (every refresh looked logged out) | `AuthProvider.tsx`, `CallbackPage.tsx` | **Fixed this pass** — see §1.1 note; still needs live verification against Authentik (Phase 2) |
| No live verification of login / list / create / SSE / delete / terminal | whole app | 1–4 |
| Phase enum not confirmed against a real claim (badge colors guessed) | `StatusBadge.tsx`; arch §8 item 6 | 3 |
| Toasts absent — errors are inline-only; `5xx` retry-toast pattern unimplemented | design-principles §3 | 5 |
| No global error boundary; `NotFoundPage` minimal | `src/App.tsx` | 5 |
| Accessibility + mobile pass not done (modal focus-trap/Esc, keyboard, viewport) | `CreateSessionDialog`, `Layout` | 5 |
| `lint` script references ESLint but **no eslint config exists** | `frontend/` | 6 |
| **Zero tests**, no test runner | `frontend/` | 6 |
| `animejs` is a dependency but **unused** (no status/list-transition polish) | components | 8 (deferred) |
| shadcn primitives partially extracted — `button.tsx`/`card.tsx` exist in `src/components/ui/`, but `CreateSessionDialog`'s modal and `StatusBadge` are still hand-rolled inline (no `dialog`/`badge` primitives) | `src/components/ui/`, `CreateSessionDialog.tsx`, `StatusBadge.tsx` | 8 (optional) |

---

## 2. Prerequisites (blocking infra — not frontend code)

These are backend/platform changes the SPA depends on. Per the arch §8 action
items — with **current status** noted, since several are already done.

| # | Prerequisite | Status | Action |
|---|---|---|---|
| P1 | Chart `VITE_API_BASE` = `/api` (not `/api/v1`) | ✅ Done | `kubesandbox-charts/frontend/values.yaml:164` already `/api`. |
| P2 | **Trust SPA tokens on `/api`** — add `kubesandbox-frontend` issuer+JWKS as a second JWT provider | ✅ **Wired in prod, rollout pending (rev 13)** | Repo's `kubesandbox-charts/kubesandbox-backend/values.yaml` keeps `authentication.enabled: false`/blank issuer as the documented default — the real values live in `GitOps-Homelab/.../kubesandbox-backend/values/chart/values-prd.yaml`, which now sets `authentication.enabled: true`, the primary issuer/JWKS, and `additionalProviders` for `kubesandbox-frontend`. The chart template (`securitypolicy-api.yaml`) already renders `additionalProviders` via `range`. **What's left is Phase 0's acceptance check**, not the wiring: the frontend Authentik provider previously published an empty JWKS (fixed by adding a `signing_key` in `kubesandbox-frontend-auth.yaml`), and that fix needs the pre-resources Workspace to re-sync + the Envoy JWKS cache (300s) to clear before a real bearer token will validate. |
| P3 | Frontend Authentik client redirect URI == `https://kubesandbox.com/auth/callback` | ✅ Confirmed in files | `GitOps-Homelab/.../kubesandbox-frontend-auth.yaml` registers `allowed_redirect_uris` = `https://kubesandbox.com/auth/callback` (strict), matching chart `authentication.oidc.redirectUrl` and `frontend/.env`'s `VITE_OIDC_REDIRECT_URI`. Still worth a live check that the Workspace has actually applied. |
| P4 | Remove `/terminal` OIDC SecurityPolicy from frontend chart | ✅ Done | `protectedPaths: []` in `kubesandbox-charts/frontend/values.yaml:109`. |
| P5 | Confirm `phase` enum vocabulary against a live claim | ⬜ Pending | Needed to finalize `StatusBadge` colors (item 6). Done in Phase 3. |
| P6 | Note cookie-lifetime-vs-TTL in UI copy | ⬜ Minor | 8h session cookie vs. up-to-1440m TTL → possible silent re-auth mid-session (auth §6.4). Copy tweak in Phase 5. |

---

## 3. Phase 0 — Unblock live auth (BLOCKING)

**Update (rev 13, CHANGELOG):** the wiring for this phase is done — both in the
chart template and in the prod values override — but the acceptance check
(a real bearer token actually validating end to end) is **not yet confirmed
live**, pending a Workspace re-sync and the Envoy JWKS cache clearing. Treat
0.1–0.3 below as done; what's open is re-verifying 0.4/acceptance after the
rollout lands.

- [x] **0.1 Wire the second JWT provider.** Done in
  `GitOps-Homelab/operators-helm/operators/kubesandbox-backend/values/chart/values-prd.yaml`
  (not the in-repo `kubesandbox-charts/kubesandbox-backend/values.yaml`, which
  intentionally keeps `authentication.enabled: false` / blank issuer as the
  chart's documented default — real environment values live in the separate
  GitOps-Homelab repo):

  ```yaml
  authentication:
    enabled: true
    jwt:
      issuer: "https://auth.jeremymr.dev/application/o/kubesandbox-backend/"
      jwksUri: "https://auth.jeremymr.dev/application/o/kubesandbox-backend/jwks/"
      additionalProviders:
        - name: kubesandbox-frontend
          issuer: "https://auth.jeremymr.dev/application/o/kubesandbox-frontend/"
          jwksUri: "https://auth.jeremymr.dev/application/o/kubesandbox-frontend/jwks/"
          claimToHeaders:
            - { claim: sub, header: X-User-Id }
            - { claim: email, header: X-User-Email }
            - { claim: name, header: X-User-Name }
            - { claim: groups, header: X-User-Groups }
  ```

  `claimToHeaders` mirrors the primary provider on both, so
  `IdentityMiddleware` sees `X-User-Id` regardless of which client issued the
  token (arch §4.1). Chart template
  (`kubesandbox-charts/kubesandbox-backend/templates/securitypolicy-api.yaml`)
  already `range`s over `additionalProviders` — no template change needed.
- [x] **0.2 Confirm the frontend provider is RS256 / has a populated JWKS —
  fix landed.** Root cause found live: the `kubesandbox-frontend` Terraform
  Workspace never set a `signing_key`, so Authentik published an empty JWKS
  (`{}`) and `/api` 401'd with "Jwks remote fetch is failed" even after 0.1.
  Fixed in
  `GitOps-Homelab/.../kubesandbox-frontend/pre-resources/templates/kubesandbox-frontend-auth.yaml`
  by adding a `signing_key_name` var + `authentik_certificate_key_pair` data
  source and setting `signing_key`, `issuer_mode = "per_provider"`,
  `sub_mode = "hashed_user_id"` on the frontend provider (mirrors the backend
  client). **Not yet verified live** — rollout requires: sync the frontend
  pre-resources ArgoCD app → confirm
  `.../kubesandbox-frontend/jwks/` returns `{"keys":[…]}` (not `{}`) → wait out
  Envoy's 300s JWKS cache (or bounce the gateway pod) → re-login in the SPA for
  a fresh RS256-signed token.
- [x] **0.3 Render + lint the chart.** Template confirmed by inspection to
  render both providers correctly (`{{- range .Values.authentication.jwt.additionalProviders }}`
  in `securitypolicy-api.yaml`); re-run `helm template`/`helm lint` after any
  further values changes as a sanity check.
- [ ] **0.4 Deploy & confirm live.** ArgoCD sync the backend app (and the
  frontend pre-resources app for 0.2) if not already synced; confirm the
  `SecurityPolicy` is `Accepted` with both providers (already true for the
  backend provider per CHANGELOG rev 13 — pending re-confirmation for the
  frontend one after the JWKS fix rolls out).

**Acceptance (still open):** with a real SPA token,
`curl -H "Authorization: Bearer <token>" https://kubesandbox.com/api/sessions`
→ `200`, and the backend logs show `X-User-Id` populated (arch §8 item 3). This
is also tracked as the last open item in `docs/02-auth-design.md` §7.

---

## 4. Phase 1 — Local dev loop

Get a fast inner loop before touching prod.

- [ ] **1.1 Install + run.** `cd frontend && npm ci && npm run dev`.
- [ ] **1.2 Dev proxy for `/api`.** Already wired: `vite.config.ts` proxies
  `/api` (which also covers the SSE path `/api/sessions/:id/events`) to
  `VITE_DEV_PROXY_TARGET ?? http://localhost:8080`. Just confirm it points at a
  reachable backend. (Terminal `/s/{id}` is cross-origin to prod — not proxied.)
- [ ] **1.3 Local env.** Copy `.env.example` → `.env.local`; point OIDC at the
  real Authentik frontend client and `VITE_PUBLIC_BASE_URL` at prod (terminal
  handoff is cross-origin anyway).
- [ ] **1.4 Sanity gates.** `npm run typecheck` (already green) and `npm run
  build` succeed.

**Acceptance:** `npm run dev` serves the landing page; `npm run build` produces
`dist/` with hashed assets.

---

## 5. Phase 2 — Auth flow, end to end

- [ ] **2.1 Sign-in.** From `/`, "Sign in" → Authentik → `/auth/callback`
  completes PKCE → lands on `/dashboard`. Confirm `CallbackPage`'s StrictMode
  double-run guard holds (no duplicate token exchange).
- [ ] **2.2 In-memory posture.** Confirm no tokens land in `localStorage`;
  reload triggers silent renew against the Authentik SSO session. **Code fixed
  this pass** — `AuthProvider` previously never attempted `signinSilent()` on
  load (it only read the now-empty in-memory store and let `ProtectedRoute`
  redirect before any renew could happen); `CallbackPage` didn't handle being
  loaded inside the hidden renew iframe either. Both are fixed (§1.1). Still
  needs a live browser check against Authentik once P2's rollout (Phase 0) is
  confirmed.
- [ ] **2.3 `returnTo`.** Deep-link to `/dashboard/:id` while logged out →
  bounce to login → return to the intended route after callback.
- [ ] **2.4 Guarded routes.** `ProtectedRoute` redirects unauthenticated users;
  `Layout` shows identity + working "Sign out".
- [ ] **2.5 401 handling.** Force an expired/invalid token and confirm the app
  triggers silent renew / re-auth rather than a dead screen (arch §4.5).

**Acceptance:** full login/logout/deep-link cycle works; token never persisted.

---

## 6. Phase 3 — Session lifecycle, end to end

- [ ] **3.1 Empty state.** New user sees the "Create your first sandbox" CTA
  (design-principles §6). Already implemented — verify live.
- [ ] **3.2 Create.** `CreateSessionDialog` → `POST /api/sessions` (profile +
  TTL) → `201` → optimistic list update, dialog closes.
- [ ] **3.3 Live provisioning (the headline feature).** New session streams
  `Pending` → provisioning → `Ready` via SSE **without refresh**
  (`useSessionEvents` → cache). Confirm the 25s heartbeat keeps the stream open
  through the ~4–5 min vcluster cold-boot, and the fallback **polls** if the
  stream drops (arch §4.3).
- [ ] **3.4 Confirm phase vocabulary (P5).** Read a real claim's `phase` values
  from the Crossplane `function-auto-ready` pipeline; reconcile
  `StatusBadge.tone()` (fail/error, terminating, pending) against them. Keep
  `workspaceReady` as the authoritative "usable" gate; treat `phase` as a label
  (arch §8 item 6).
- [ ] **3.5 Provisioning affordance.** Since cold-boot exceeds the 1s+ threshold,
  show phase **+ elapsed time**, not an indefinite spinner (design-principles §2,
  §7). Add elapsed-time display if not present.
- [ ] **3.6 Delete.** Optimistic removal from the list; rollback on error;
  `invalidate` on settle. Confirm the sandbox actually tears down.
- [ ] **3.7 Quota (429).** Hit the concurrency cap → surface "you've reached your
  session limit" (already mapped in `CreateSessionDialog`) — verify live.
- [ ] **3.8 404 semantics.** Unknown/unowned id → same `not_found`; confirm no
  existence leak and a clean "session not found" on the detail page.

**Acceptance:** create → watch `Ready` live → delete, with correct badges and
no manual refresh.

---

## 7. Phase 4 — Terminal hand-off

- [ ] **4.1 Gate the button.** "Open terminal" enabled only when
  `workspaceReady === true` (already wired in `SessionCard` + detail).
- [ ] **4.2 New-tab hand-off.** `TerminalPage` / links open
  `{VITE_PUBLIC_BASE_URL}/s/{id}` in a **new tab**, never an iframe — the first
  hit 302s to Authentik's login, which can't be framed (arch §4.4).
- [ ] **4.3 Silent SSO bounce.** Because the user already authenticated for the
  SPA, `/s/{id}` round-trips through the existing Authentik session invisibly and
  lands in ttyd. Verify the popup-blocked manual fallback button works.
- [ ] **4.4 `kubectl` smoke test.** In ttyd, confirm a working, isolated
  `kubectl` against the user's private vcluster.

**Acceptance:** click "Open terminal" → working ttyd, no visible second login,
never another user's session.

---

## 8. Phase 5 — UX hardening (design principles)

- [ ] **5.1 Four states everywhere.** Confirm loading (skeletons), empty, error,
  and partial states on every data view (design-principles §1). Dashboard has
  them; audit the detail page.
- [ ] **5.2 Toasts.** Add a lightweight toaster for transient failures — `5xx`
  retry-with-backoff + toast, delete failures, stream-lost notices
  (design-principles §3/§4). Today errors are inline-only.
- [ ] **5.3 Error messages.** Every user-facing error = what happened + why + how
  to fix (design-principles §4). Sweep `ApiError` surfaces.
- [ ] **5.4 Global error boundary.** Wrap the router so a render throw shows a
  recoverable screen, not a blank page.
- [ ] **5.5 Accessibility.** `CreateSessionDialog`: focus-trap, `Esc` to close,
  `aria-modal`, restore focus on close. Keyboard-navigable actions; visible focus
  rings; the TTL range input needs a label association + text value (present) and
  `aria-valuetext`.
- [ ] **5.6 Mobile / responsive.** Verify the `sm:grid-cols-2` dashboard, header,
  and dialog at narrow widths.
- [ ] **5.7 Copy: cookie vs TTL (P6).** Note that long-TTL sessions may prompt a
  silent re-auth in the terminal (auth §6.4).

**Acceptance:** every failure path is legible and recoverable; dialog is
keyboard- and screen-reader-usable; layout holds on mobile.

---

## 9. Phase 6 — Quality gates

- [ ] **6.1 ESLint.** `npm run lint` is defined as `eslint .` but **eslint isn't
  installed and no config exists**. Add `eslint` + `typescript-eslint` +
  `eslint-plugin-react-hooks` + `eslint-plugin-react-refresh` to devDeps and a
  flat `eslint.config.js`. Fix violations.
- [ ] **6.2 Test runner.** Add Vitest + React Testing Library.
- [ ] **6.3 Unit tests (highest value first):**
  - `lib/api.ts` SSE frame parser — heartbeats, multi-line `data:`, `deleted`
    vs `update`, malformed frames.
  - `lib/schemas.ts` — `sessionListSchema` null→`[]`, TTL bounds, profile enum.
  - Error mapping — status/code → `ApiError`.
  - `StatusBadge.tone()` — phase/`workspaceReady` → label+class.
- [ ] **6.4 Hook/component tests.** `useSessions` optimistic delete rollback;
  `useSessionEvents` cache writes + polling fallback (mocked stream);
  `CreateSessionDialog` submit + quota error.
- [ ] **6.5 CI.** Add `typecheck` + `lint` + `test` to
  `.github/workflows/frontend.yml` (gate the image build). The build/push path
  contract already matches the scaffold — no path change needed.

**Acceptance:** `npm run lint` and `npm run test` pass in CI; image builds only
when they do.

---

## 10. Phase 7 — Build & deploy

- [ ] **7.1 Image.** Multi-stage build already correct (`node:22-alpine` →
  `nginx:1.27-alpine`, :8080, `/health/{livez,readyz}`, SPA fallback,
  `docker-entrypoint.sh` writes `/config.js`). Build locally and smoke-test the
  container: confirm `/config.js` reflects injected env and `/health/readyz` →
  200.
- [ ] **7.2 Chart.** `kubesandbox-charts/frontend` is at **v0.1.9** (already past
  the 0.1.8 that dropped `/api`; do not regress). Public HTTPRoute serves `/`,
  `/assets`, health; `protectedPaths: []` (terminal is backend-owned).
- [ ] **7.3 Runtime env.** Chart `env:` sets `VITE_*`; confirm they flow into
  `window.__ENV` at container start (not baked at build).
- [ ] **7.4 Promote.** One image, env-injected config, across environments. Sync
  ArgoCD; verify the deployed SPA hits the real `/api` and terminal origin.

**Acceptance:** deployed image serves the app; health probes green; live config
correct.

---

## 11. Phase 8 — Deferred polish (optional, do last)

- [ ] **8.1 anime.js.** It's already a dependency but unused. Add light polish
  only — status-badge transitions, list enter/leave (arch §2, design-principles
  §8.1). Keep it additive; respect `prefers-reduced-motion`.
- [ ] **8.2 shadcn extraction.** `button.tsx`/`card.tsx` already live in
  `components/ui/`. Optionally extract the rest — `CreateSessionDialog`'s modal
  markup and `StatusBadge` — into `components/ui/{dialog,badge}.tsx` for
  consistency with the `ui/` convention (design-principles §9). Cosmetic —
  current inline versions work.
- [ ] **8.3 Immersive visuals (explicitly out of scope for first ship).**
  react-three-fiber / ShaderGradient / liquid-glass live in
  `useful-repos-for-frontend/` and are a later pass (arch §2).

---

## 12. Definition of Done

Ship when this end-to-end walk passes on prod (arch §9):

- [ ] Open `kubesandbox.com`, sign in via Authentik.
- [ ] See an (initially empty) dashboard with a create CTA.
- [ ] Create a `standard` session.
- [ ] Watch it stream `Pending` → provisioning → `Ready` **without refreshing**.
- [ ] Click **Open terminal** → working `kubectl`-enabled ttyd against your
  private vcluster.
- [ ] Never see anyone else's session (ownership scoped on `sub`).
- [ ] Delete a session → optimistic removal + real teardown.
- [ ] Quality gates (typecheck, lint, tests) green in CI; image deployed via
  chart v0.1.9+.

---

## 13. Sequencing summary

```
P0 (blocking infra) ─► P1 dev loop ─► P2 auth ─► P3 sessions ─► P4 terminal
                                                      │
                                                      ▼
                                          P5 UX hardening ─► P6 quality gates ─► P7 deploy
                                                                                     │
                                                                                     ▼
                                                                            P8 polish (deferred)
```

Phase 0 gates everything (tokens must be accepted). Phases 1–4 are the happy-path
bring-up and can surface bugs that feed back into the scaffold. Phases 5–6
harden and gate. Phase 7 ships. Phase 8 is additive and can slip without
blocking the Definition of Done.
