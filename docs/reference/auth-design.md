# KubeSandbox — Authentication & Authorization Design

**Status:** living design (updated 2026-07-02 — G1–G5 live and verified on prod-k3s)
**Audience:** platform engineers wiring auth for KubeSandbox
**Related:** [`backend-architecture.md`](./backend-architecture.md) · [`frontend-architecture.md`](./frontend-architecture.md)

---

## 1. Two problems, two places

"OIDC" is really two concerns, solved in two places:

| Concern | When | Who | Status |
|---|---|---|---|
| **Provision the OIDC client** in the IdP, get `client_id`/`client_secret`. | GitOps reconcile time. | **Crossplane** (Terraform provider → Authentik). | **Built.** |
| **Enforce authentication** (login flow, cookie, token validation) at request time. | Request time. | **Backend** owns the full OIDC/PKCE flow for session routes. **Envoy Gateway** JWT policy for `/api`. | **Built and live (rev 11).** |
| **Authorize per-session ownership** (this user owns this session). | Request time. | **Backend** `/authz` via ext-authz. | **Built and live (rev 11). Negative test confirmed.** |

The first two halves hand off through a **Kubernetes Secret** of client credentials
written by the Crossplane Terraform Workspace
(`kubesandbox-backend-auth.yaml` → `kubesandbox-backend-client-secret`).

---

## 2. Provisioning (built) — Crossplane → Authentik

The repo provisions Authentik OIDC clients with the Terraform provider, two ways:

- **Reusable module** — `crossplane/.../authentik-oidc-app.yaml` is a ConfigMap
  Terraform module (`goauthentik/authentik ~> 2025.2.0`) that creates an
  `authentik_provider_oauth2` + `authentik_application` and outputs
  `client_id` / `client_secret` (confidential client, strict redirect URI).
  Supports `additional_redirect_uris` (added rev 9) for multiple callback URLs.
- **Backend instance** — `operators-helm/operators/kubesandbox-backend/pre-resources/templates/kubesandbox-backend-auth.yaml`:
  Crossplane Terraform `Workspace` that creates the `kubesandbox-backend` OIDC
  client with:
  - RS256 signing key (`signingKeyName`, default `"authentik Self-signed Certificate"`)
    and `issuer_mode: per_provider`, so the JWKS endpoint is populated.
  - Redirect URI: `https://kubesandbox.com/oauth2/callback` (strict).
  - `access_code_validity: "minutes=5"` — auth codes are consumed immediately.
  - Generates a 64-char `SESSION_SECRET` via `hashicorp/random` (written once,
    `ignore_changes = all` — rotating it invalidates all live session cookies).
  - Writes **`kubesandbox-backend-client-secret`** (namespace `kubesandbox`) with
    keys: `client-id`, `client-secret`, `session-secret`, `issuer-url`, `jwks-uri`.
- **Frontend instance** — `kubesandbox-frontend/.../kubesandbox-frontend-auth.yaml`:
  creates the **public** `kubesandbox-frontend` OIDC client (Auth Code + PKCE, no
  secret in the browser) used by the SPA. Live (G5). Its issuer is trusted as an
  additional provider on the `/api` JWT policy so SPA-minted tokens are accepted
  (see §3.1).

---

## 3. Auth architecture (as built)

Two auth surfaces, handled differently:

### 3.1 `/api` — JWT bearer (G1/G4)

The backend's session control API (`POST/GET/DELETE /api/sessions`, the queue
endpoints, and the SSE streams) is protected by a JWT `SecurityPolicy`
(`securitypolicy-api.yaml`) on Envoy Gateway.

- Validates Authentik-issued JWTs against the provider JWKS endpoint.
- The SPA is a **public client** with a *different* issuer than the backend
  client, so the policy trusts **both providers** (`additionalProviders` in the
  chart): the backend `kubesandbox-backend` issuer and the SPA's
  `kubesandbox-frontend` issuer. Envoy accepts a token matching any provider.
- Maps claims to identity headers via `claimToHeaders` (on every provider):
  `sub → X-User-Id`, `email → X-User-Email`, `name → X-User-Name`, `groups → X-User-Groups`.
- Missing/invalid token → `401 "Jwt is missing"`.
- Backend reads identity from these headers (trusts the gateway-injected values).

### 3.2 `/s/{id}` — Backend-owned OIDC/PKCE + ext-authz (G2)

Session terminal routes are protected by a `SecurityPolicy` (`securitypolicy-session.yaml`)
that does **ext-authz only** — no edge OIDC or JWT filter. The backend drives
the complete auth flow:

```
Browser hits kubesandbox.com/s/{id}
  │
  ▼ Envoy SecurityPolicy: ext-authz → GET backend /authz
      │
      ├─ No/invalid session cookie
      │     backend: generate PKCE + a random nonce, sign a state token
      │       (code_verifier + originalURL + nonce), set a short-lived nonce
      │       cookie, return 302 → Authentik
      │     Envoy: forward 302 to browser
      │     Browser: login at auth.jeremymr.dev/application/o/authorize/
      │     Authentik: redirect to kubesandbox.com/oauth2/callback?code=...&state=...
      │     Backend (/oauth2/callback route):
      │       verify state token → extract code_verifier + originalURL + nonce
      │       verify the nonce cookie matches the state nonce, then clear it
      │         (OAuth login-CSRF protection — see below)
      │       POST token endpoint /application/o/token/ → id_token
      │       parse id_token claims (sub, email, name) — no JWKS validation,
      │         trusted via server-to-server TLS
      │       set HttpOnly Secure SameSite=Lax session cookie
      │         (HMAC-SHA256: base64url(JSON({sub,email,name,exp})).sig)
      │       302 → originalURL
      │
      └─ Valid session cookie
            backend: verify HMAC, check expiry
            extract session {id} from X-Forwarded-Uri
            look up KubeSandboxSession claim → check ownerRef == cookie.sub
            owner → 200 (Envoy allows, routes to shell Service)
            not owner / unknown id / malformed → 403
            backend error → 503 (fail closed)
```

**Login-CSRF protection (nonce binding).** The signed state is otherwise
self-contained and portable, so an attacker could complete their own login and
hand the resulting `code`+`state` pair to a victim (e.g. a crafted link to
`/oauth2/callback`) to silently log the victim's browser in *as the attacker*.
To stop this, the `/authz` redirect also sets a short-lived **nonce cookie** whose
value is embedded in the signed state; `/oauth2/callback` requires both copies to
match (and clears the cookie unconditionally). The nonce cookie can only have been
set by the browser that started the flow, so a mismatch means the callback wasn't
triggered by that browser. The cookie is `SameSite=Lax` so it survives Authentik's
cross-site top-level redirect back to the callback.

**Why Option A+B (not the original Option A):** The original plan was edge OIDC →
JWT `claimToHeaders` → ext-authz in one policy. This doesn't work on
Envoy Gateway v1.7.1: (a) `SecurityPolicy` is same-namespace only, but sessions
were per-namespace; (b) ext-authz fires before the OIDC filter can log the user
in. The redesign moves HTTPRoutes to the shared `kubesandbox` namespace (Option A)
and gives the backend full ownership of the OIDC flow (Option B), eliminating both
constraints. See [`g2-session-auth-spike.md`](../history/g2-session-auth-spike.md) for the
full spike write-up.

---

## 4. Authentik specifics

### 4.1 Endpoint URLs

Authentik's OIDC endpoints are **shared** (not per-provider), even though the
issuer URL is per-provider:

| Endpoint | URL |
|---|---|
| Issuer (per-provider) | `https://auth.jeremymr.dev/application/o/kubesandbox-backend/` |
| Authorization | `https://auth.jeremymr.dev/application/o/authorize/` |
| Token | `https://auth.jeremymr.dev/application/o/token/` |
| JWKS | `https://auth.jeremymr.dev/application/o/kubesandbox-backend/jwks/` |

The backend derives endpoints from the issuer by default as `{issuer}authorize/`
and `{issuer}token/`, which are wrong. **Always set `authEndpoint` and
`tokenEndpoint` explicitly** in chart values (`values-prd.yaml`).

### 4.2 `sub` claim — hashed user ID

Authentik's `sub_mode: hashed_user_id` means the `sub` claim equals the user's
`uid` field — a **64-char hex hash** (e.g.
`ea74e1d87924d4a0ff660caa375ce9538c83db3d0152e87082c4414140b1e568`).

This is **not** the UUID or email. The backend stores this as `ownerRef` on the
claim. The frontend uses the `sub` from the JWT (not the email) as the identity
key, so ownership lines up across `/api` and `/authz` without translation.

To look up a user's `sub` via the Authentik API:
```
curl -s "https://auth.jeremymr.dev/api/v3/core/users/?search=<username>" \
  -H "Authorization: Bearer <authentik-api-token>" | jq '.[].uid'
```

---

## 5. Backend's role in auth

- **AuthN for `/api` (G1):** identity comes from **Envoy-forwarded headers**
  (`X-User-Id`, `X-User-Email`, etc.) injected by the JWT `SecurityPolicy`.
  Backend trusts the gateway-injected values and maps `X-User-Id → ownerRef`.
  The backend NetworkPolicy (`networkpolicy.yaml`) ensures only `envoy-gateway-system`
  can reach the backend — no in-cluster pod can spoof `X-User-*` headers.
- **AuthN for `/s/{id}` (G2):** backend **owns the full OIDC/PKCE flow** (see §3.2).
  No forwarded identity headers for session routes — identity comes from the
  session cookie the backend issues.
- **AuthZ for sessions:** backend `/authz` endpoint — resolves `{id}` from the
  forwarded path, checks `ownerRef == cookie.sub`. Ownership logic lives in one
  place; revocation is immediate (delete the claim → 403/404 on the next request).

---

## 6. Operational notes

1. **GitOps ordering.** Authentik client Secret must exist before the SecurityPolicy
   is valid. Respect existing sync waves (Authentik module 15, frontend auth 17,
   session HTTPRoute 18, XRD/composition/RBAC 25). The backend ArgoCD app
   (`kubesandbox-backend-helm`) and its pre-resources app
   (`kubesandbox-backend-prereqs`) must both be synced before `sessionAuth.enabled: true`
   has effect.
2. **Issuer correctness.** `issuer` must match the `iss` in issued tokens exactly
   (including trailing slash). Any new environment must also set `authEndpoint` and
   `tokenEndpoint` explicitly (see §4.1).
3. **TLS to Authentik.** `auth.jeremymr.dev` uses a publicly-trusted cert.
   If an internal CA is used, add a `BackendTLSPolicy` so Envoy can fetch the JWKS.
4. **Cookie/session lifetime.** Default 8 hours (`SESSION_MAX_AGE_SECONDS: 28800`).
   For sessions longer than 8 hours, increase this to match `ttlMinutes`.
5. **SESSION_SECRET rotation.** The `session-secret` key in
   `kubesandbox-backend-client-secret` is generated once with
   `lifecycle { ignore_changes = all }`. Rotating it invalidates **all live
   session cookies** — every active user will be redirected to log in again.
6. **Redirect URI exactness.** Authentik uses strict matching. The `redirectURI`
   in chart values and the URI registered in the Terraform Workspace must match
   character-for-character (`https://kubesandbox.com/oauth2/callback`).
7. **Cross-namespace Secret refs.** The SecurityPolicy references the backend
   Service in the same namespace (`kubesandbox`) — no ReferenceGrant needed for
   the policy. The per-session ReferenceGrant is for the cross-namespace
   HTTPRoute → shell Service backendRef.

---

## 7. Decision checklist

- [x] **G1 identity** chosen: backend **trusts Envoy-forwarded `X-User-*` headers** for `/api`. Backend NetworkPolicy locks the gateway→backend path (default-on, ingress from `envoy-gateway-system` only). **Live and verified (rev 3/rev 6).**
- [x] **(G4)** `kubesandbox-backend` Authentik client + `kubesandbox-backend-client-secret` created by Terraform Workspace. RS256 signing key; JWKS populated. JWT policy Accepted and enforcing. **Live (rev 2). Valid-bearer + X-User-* injection still needs one end-to-end confirm with a real token.**
- [x] **Authz model:** backend-owned OIDC/PKCE (Option B) + session HTTPRoutes in shared namespace (Option A). **Live (rev 11).**
- [x] **Backend `/authz` endpoint:** `X-Forwarded-Uri` → `{id}` → claim → `ownerRef == sub`. **Live (rev 4/rev 11).**
- [x] **Shared session SecurityPolicy** (ext-authz only, `targetSelectors` on `session-route: "true"`). One policy for all sessions. **Live (rev 8/rev 11).**
- [x] **`/oauth2/callback` route** — unauthenticated HTTPRoute for login completion. **Live (rev 8/rev 11).**
- [x] **Negative test:** user B → 403 on user A's session. **Confirmed on prod-k3s (rev 11).**
- [x] **WebSocket upgrade** (ttyd) verified end-to-end through ext-authz. **Confirmed (rev 11).**
- [x] **JWT with real bearer (G4):** valid Authentik token → `/api` returns `200` and `X-User-*` reach the backend. **Confirmed via the live SPA (G5).**
- [x] **SPA provider trust:** the `kubesandbox-frontend` issuer is added as an additional provider on the `/api` JWT policy so public-client tokens are accepted. **Live.**
- [x] **Login-CSRF nonce binding** on the `/s/{id}` flow. **Built and live.**
- [ ] **Issuer split-DNS check:** confirm internal/external issuer URLs match; `BackendTLSPolicy` if internal CA. (N/A for current homelab setup — publicly-trusted cert.)
- [ ] **Cookie lifetime ≥ max `ttlMinutes`:** default 8h / 480 min; XRD caps at 1440 min. Increase `SESSION_MAX_AGE_SECONDS` if supporting max-TTL sessions.

---

## 8. Sources

- [OIDC Authentication — Envoy Gateway docs](https://gateway.envoyproxy.io/docs/tasks/security/oidc/)
- [SecurityPolicy — Envoy Gateway docs](https://gateway.envoyproxy.io/docs/concepts/gateway_api_extensions/security-policy/)
- [Envoy Gateway v1.8.0 release notes](https://gateway.envoyproxy.io/news/releases/notes/v1.8.0/)
- [Envoy Gateway v1.7.0 release notes](https://gateway.envoyproxy.io/news/releases/notes/v1.7.0/)
- [RFC 7636 — PKCE](https://www.rfc-editor.org/rfc/rfc7636)
- In-repo: `kubesandbox-backend-auth.yaml`, `authentik-oidc-app.yaml`,
  `backend/internal/auth/session.go`, `backend/internal/auth/oidc.go`,
  `backend/internal/api/handlers/authz.go`, `backend/internal/api/handlers/auth.go`,
  `kubesandbox-charts/kubesandbox-backend/templates/securitypolicy-session.yaml`
- Spike findings: [`g2-session-auth-spike.md`](../history/g2-session-auth-spike.md)
