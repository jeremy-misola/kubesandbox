# KubeSandbox — G2 Session-Auth Spike Findings

**Status:** spike complete (2026-06-27); **decision made (2026-06-26): implementing Options A + B** — see §5.
**Audience:** Jeremy (platform owner)
**Related:** [`02-auth-design.md`](./02-auth-design.md) · [`04-backend-handoff.md`](./04-backend-handoff.md)

> **TL;DR:** The G2 design as specced — *one shared SecurityPolicy on the session
> route doing OIDC (cookie) → JWT (claimToHeaders) → ext-authz to the backend* —
> **does not work on the deployed gateway (Envoy Gateway v1.7.1)**, for two
> independent reasons: (1) `SecurityPolicy` attaches **same-namespace only**, but
> session routes live in per-session namespaces; and (2) **ext-authz runs before
> OIDC can complete the login**, so an unauthenticated browser gets `401` from
> `/authz` instead of a login redirect. The backend `/authz` logic itself is fine
> and was reached through the gateway. The session-auth approach needs a redesign
> (options below). Everything else (NetworkPolicy, TTL, sweep, `/authz` decisions)
> is verified working.

---

## 1. Environment

- **Envoy Gateway v1.7.1** (Envoy proxy v1.37.1) — *not* the v1.8 the chart/docs
  assumed. The shared `kubesandbox` Gateway lives in `envoy-gateway-system`; the LB
  is `kubesandbox-gateway-service` (`192.168.0.240`).
- 5 working OIDC `SecurityPolicy`s already exist (adguard, backstage, code-server,
  headlamp, stirling-pdf) — used as the known-good reference.

## 2. What was tested

Applied the session `SecurityPolicy` (modeled on the chart template) against a real
session route (`shell` HTTPRoute, label `kubesandbox.com/session-route=true`, in
namespace `playground-s-spike01`) and probed the live gateway. The backend OIDC
client (`kubesandbox-backend`, RS256, JWKS) was used so one client could drive both
OIDC and JWT.

## 3. Findings

### 3.1 Field names (chart corrections needed) — confirmed valid on v1.7.1
`clientIDRef` + `clientSecret` (Secret refs), `provider.issuer`, `redirectURL`,
`logoutPath`, `refreshToken`, `jwt.providers[].claimToHeaders`,
`extAuth.headersToExtAuth`, `extAuth.http.backendRefs`, `extAuth.http.path`,
`targetRefs`, `targetSelectors`. EG defaults `extAuth.failOpen: false` (fail-closed).

**The chart template is wrong for v1.7.1:** it uses inline `clientID` and
`forwardAccessToken`. The working pattern is **`clientIDRef`/`clientSecret` Secret
refs** (key names `client-id` / `client-secret`) and **`refreshToken: true`** (no
`forwardAccessToken`).

### 3.2 `SecurityPolicy` attaches **same-namespace only** — breaks "one shared policy"
A policy placed in `kubesandbox` with `targetSelectors` matching the session-route
label got **no status at all** — it did **not** attach to the route in
`playground-s-spike01`. The identical policy placed **in the session namespace**
attached immediately (status ancestor = the gateway).

Because the Crossplane composition puts each session's `shell` HTTPRoute in its
**own per-session namespace**, a single shared policy cannot cover them. The
auth-design assumption ("path-based routing means one SecurityPolicy covers every
session") is **false** on this gateway.

### 3.3 OIDC secret + ext-authz backend are namespace-bound
With the policy in the session namespace, it was `Accepted: False` with exact errors:
- `OIDC: secret playground-s-spike01/kubesandbox-backend-client-secret does not exist` — the OIDC `clientSecret`/`clientIDRef` resolve **in the policy's own namespace** (no cross-namespace secret ref).
- `ExtAuth: backend ref to Service kubesandbox/kubesandbox-backend-helm not permitted by any ReferenceGrant` — the cross-namespace ext-authz backendRef needs a **ReferenceGrant**.

After copying the client secret into the session namespace and adding a
ReferenceGrant, the policy became **`Accepted: True`**. So the full
OIDC+JWT+ext-authz spec is schema-valid and attaches — but see 3.4.

### 3.4 **ext-authz fires before OIDC login — the blocker**
With OIDC **alone**, an unauthenticated browser request to `/s/{id}/` returns a
clean **`302` to Authentik**:
`https://auth.jeremymr.dev/application/o/authorize/?client_id=kubesandbox-backend&...&redirect_uri=https%3A%2F%2Fkubesandbox.com%2Foauth2%2Fcallback&...` (PKCE, scope=openid, state preserving the original `/s/{id}/` URL). **OIDC works.**

But with **OIDC + ext-authz** in the same policy, the same request returns
**`401` with body `{"error":"unauthorized","message":"missing identity headers from gateway"}`** — i.e. the **backend `/authz`** response. So ext-authz ran **first**, hit `/authz` with no identity, and fail-closed `401` **before** OIDC could redirect the user to log in. On EG v1.7.1 you cannot get "log in, then check ownership" from a single combined policy.

### 3.5 Positives confirmed
- The **ext-authz callout reaches the backend `/authz` through the gateway**
  (cross-namespace via ReferenceGrant) and the backend responds — the wiring works.
- The backend `/authz` decision logic is already verified (owner 200 / non-owner
  403 / unknown 403 / no-identity 401 — see handoff §2.8).
- OIDC redirect target is **`https://kubesandbox.com/oauth2/callback`** — this must
  be registered as a redirect URI on the **`kubesandbox-backend`** Authentik client
  (it currently exists for API/JWT use only).

## 4. Why the specced design fails (summary)

| Assumption in `02-auth-design.md` | Reality on EG v1.7.1 |
|---|---|
| One shared SecurityPolicy covers all sessions (path-based). | Policy is **same-namespace only**; sessions are per-namespace → needs one policy **per session namespace**. |
| OIDC → JWT(claimToHeaders) → ext-authz delivers `X-User-*` to `/authz`. | **ext-authz runs before OIDC**, so an unauthenticated request is `401`'d before login; the chain never authenticates. |
| `clientID` inline, `forwardAccessToken`. | Use `clientIDRef`/`clientSecret` Secret refs + `refreshToken`. |

## 5. Options to make G2 work — **Decision: A + B (implementing)**

**A. Move session HTTPRoutes into the shared `kubesandbox` namespace** ✅ **chosen**
(composition change; keep pods/quota/netpol per-session, add a ReferenceGrant so the
route can reach the per-session `shell` Service). This fixes 3.2/3.3 — one shared
policy, ext-authz backend in the same namespace. **Does not by itself fix 3.4.**

**B. Backend-owned session auth** ✅ **chosen (paired with A).**
Drop the edge OIDC+JWT from the session route; the route does **ext-authz →
backend `/authz` only**. The backend becomes the auth authority: on a request with no
valid session cookie it returns a **redirect to Authentik** (Envoy ext-authz forwards
the `302` + `Location` to the browser), handles `/oauth2/callback` (PKCE code
exchange, sets session cookie), and on authenticated requests validates the cookie and
checks ownership. All auth logic lives in one place; sidesteps the
ext-authz-vs-OIDC ordering entirely. Implemented in this changeset (chart 0.1.9):
- `backend/internal/auth/session.go` — HMAC-SHA256 session cookie sign/verify
- `backend/internal/auth/oidc.go` — PKCE, state JWT, token exchange, ID token parse
- `backend/internal/api/handlers/authz.go` — cookie check → redirect or ownership
- `backend/internal/api/handlers/auth.go` — `/oauth2/callback` handler
- `kubesandbox-session-composition.yaml` — HTTPRoute → kubesandbox ns + ReferenceGrant
- `securitypolicy-session.yaml` — ext-authz only (OIDC+JWT removed)
- `httproute-callback.yaml` — unauthenticated `/oauth2/callback` → backend

**C. Per-session OIDC + an ownership sidecar.** (Not chosen — adds a sidecar.)

**D. Revisit the backend WebSocket exec-proxy.** (Not chosen — larger scope.)

## 6. Immediate chart fixes (independent of the design choice)

- `securitypolicy-session.yaml`: switch OIDC to `clientIDRef`/`clientSecret` Secret
  refs + `refreshToken`; drop inline `clientID`/`forwardAccessToken`.
- Update the `VERIFICATION REQUIRED` block: note EG **v1.7.1**, same-namespace
  attachment, and the ext-authz-before-OIDC ordering.
- Register `https://kubesandbox.com/oauth2/callback` on the `kubesandbox-backend`
  Authentik client before any session-OIDC attempt.

## 7. Test hygiene

All spike resources were removed: both spike SecurityPolicies, the copied client
secret, the ReferenceGrant, the `s-spike01` session (namespace cascading), and the
probe pod. Only the legitimate `kubesandbox-backend-helm-jwt` policy remains.
