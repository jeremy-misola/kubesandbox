# KubeSandbox — Backend Handoff

**Status:** active handoff
**Audience:** whoever picks up the backend next (incl. future me)
**Last updated:** 2026-07-01
**Related:** [`01-backend-architecture.md`](./01-backend-architecture.md) · [`02-auth-design.md`](./02-auth-design.md) · [`03-implementation-plan.md`](./03-implementation-plan.md) · [`05-g2-spike-findings.md`](./05-g2-spike-findings.md)
**Full revision history:** [`../CHANGELOG.md`](../CHANGELOG.md)

---

## 1. Where we are

End of **Phase 2 (G2 — session ownership authz): fully enabled and end-to-end
verified on prod-k3s.** The complete browser auth flow works: unauthenticated
hit → 302 PKCE redirect to Authentik → login → `/oauth2/callback` code exchange
→ HMAC session cookie → `/authz` ownership check → ttyd terminal. The negative
test (user B → 403) passes. `sessionAuth.enabled: true` is live in the prod
chart override.

## 2. Gap scorecard

| Gap | Status |
|---|---|
| G1 Backend control service | **Deployed & verified on prod-k3s.** API CRUD, ownership scoping, one-sandbox-per-user enforcement, composition provisioning all live-tested OK. *(Design revised: per-user cap replaced by a singleton enforced via deterministic claim naming.)* |
| G2 Per-session ownership authz | **✅ Fully enabled and verified on prod-k3s.** Session HTTPRoutes in shared `kubesandbox` ns + `session-route: "true"` label + ReferenceGrant; ext-authz SecurityPolicy active; backend owns the OIDC/PKCE flow. Negative test (user B → 403) confirmed. |
| Backend NetworkPolicy (G1 hardening) | **Added, default-on, verified live.** Restricts backend ingress to `envoy-gateway-system`; spoofed `X-User-*` from other namespaces fails. |
| G2b Path-based routing | **Done & verified live.** `/s/{id}` returns ttyd 200 through the gateway. |
| G3 TTL enforcement | **Implemented, not live-tested.** In-backend TTL loop + backstop sweep CronJob (`dryRun: true`). Unit-tested only — see §5 next steps. |
| G4 Backend Authentik client + token validation | **Deployed; JWT enforcing, valid-token path unverified.** Rejects missing/invalid tokens (401). Valid-bearer + `X-User-*` injection still needs a browser login to confirm — needs the frontend to exist. |
| G5 Frontend SPA | **Not built.** Must attach bearer tokens to `/api` (see §3). |
| G6 Tenant/quota model | **Partial.** One sandbox per user enforced structurally (deterministic claim name → `AlreadyExists`); profiles→resources implemented & verified. |
| G7 Observability | **Not started.** |
| G8 Starter labs | **Not started.** |

---

## 3. Identity headers: JWT policy is live; valid-token path still unverified

**Background:** Envoy Gateway's OIDC (`oauth2`) filter has no claim→header
feature — `claimToHeaders` exists only under `spec.jwt`. So the backend's
`X-User-*` identity for `/api` can only come from a **JWT** `SecurityPolicy`,
not from a cookie.

The JWT `SecurityPolicy` (`kubesandbox-backend-helm-jwt`) is deployed and
**Accepted** by the gateway, targeting `/api`, with `claimToHeaders`
(`sub→X-User-Id`, `email→X-User-Email`, `name→X-User-Name`,
`groups→X-User-Groups`) against the real Authentik issuer/JWKS. Live-tested:
missing token → `401`; malformed token → `401`.

**Still to confirm:** that a **valid** Authentik bearer yields `200` *and* the
`X-User-*` headers reach the backend. Needs a real token (browser login to
`kubesandbox-backend`) — blocked on the frontend (G5) existing. Verify with:

```
curl -H "Authorization: Bearer <token>" https://kubesandbox.com/api/sessions
```

**Pre-flight:** confirm the Authentik signing-key cert name matches
`signingKeyName` (default `"authentik Self-signed Certificate"`), else JWKS is
empty and every valid token still 401s.

---

## 4. Open caveats / decisions

1. **Bearer vs. cookie.** The JWT policy requires `Authorization: Bearer
   <token>`. The frontend (G5) must attach a token to every `/api` call —
   there is no cookie fallback for that route.
2. **SSE + headers.** Browser `EventSource` cannot set headers, so the SSE
   route can't carry a bearer via the native API. Consume SSE with a
   fetch-stream client that sets `Authorization`, or keep that one route
   cookie-based.
3. **No frontend exists.** The SPA and its ArgoCD app were removed during the
   backend rewrite. Recreate it from the frontend chart **≥ 0.1.8** (the
   version that no longer claims `/api` — recreating from 0.1.7 reintroduces
   the routing conflict described in the changelog's rev 3 entry).
4. **Authentik OIDC endpoint quirk.** The backend derives auth/token endpoints
   from the issuer as `{issuer}authorize/` and `{issuer}token/` by default, but
   Authentik's real endpoints are shared (not per-provider): `/application/o/authorize/`
   and `/application/o/token/`. Any new environment must set `authEndpoint`/
   `tokenEndpoint` overrides explicitly (already done in `values-prd.yaml`).
5. **Authentik `sub` is not the email or UUID.** `sub_mode: hashed_user_id`
   means `sub` is the user's `uid` field — a 64-char hex hash. The backend
   stores this as `ownerRef`. The frontend (G5) must use `sub` from the JWT,
   not email, as the identity key when creating sessions.

---

## 5. Next steps (in order)

### ← NEXT — Phase 3: TTL & safe cleanup (G3)
1. **Live-test the TTL loop** — create a session, wait for `ttlMinutes` to
   elapse (XRD floors at 15 min), confirm the backend reaps it. Check backend
   logs for `ttl: deleting expired session`. Then set `sweep.dryRun: false` in
   the prod chart override to activate the backstop sweep CronJob.

### Phase 4 — Frontend SPA (G5)
2. Build the frontend SPA. Key constraint: `/api` uses **JWT bearer auth** so
   the SPA must attach `Authorization: Bearer <token>` to all `/api` calls.
   Browser `EventSource` (SSE) can't set headers — use a fetch-stream client or
   keep SSE behind cookie auth. The `ownerRef` stored in claims is the
   Authentik `sub` (hashed uid), **not** the email — use the `sub` from the
   JWT when displaying/routing sessions.

### Later
3. Verify JWT with a real bearer (§3): valid Authentik token → `/api` returns
   `200` and `X-User-*` reach the backend. Needed once the frontend exists.
4. Observability/alerts (G7), rate limiting; starter labs (G8).

---

## 6. Definition of done (unchanged, from plan §5)

A user signs in at `kubesandbox.com`, creates a `standard` session, watches it
go `Ready`, clicks **Open terminal**, gets a working `kubectl`-enabled ttyd
against their private vcluster — and **cannot** reach anyone else's session.
Sessions auto-expire at TTL and clean up without wedging any CRD.
