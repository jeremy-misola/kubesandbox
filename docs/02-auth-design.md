# KubeSandbox — Authentication & Authorization Design

**Status:** living design (reconciled to the repo on 2026-06-26)
**Audience:** platform engineers wiring auth for KubeSandbox
**Related:** [`01-backend-architecture.md`](./01-backend-architecture.md) · [`03-implementation-plan.md`](./03-implementation-plan.md)

> **Decisions (2026-06-26):** session authz = **Option A (ext-authz to backend)**;
> **1 tenant = 1 user** (`tenantRef == ownerRef == sub`); session routing is
> **path-based** `kubesandbox.com/s/{id}`. **G1 backend identity = trust
> Envoy-forwarded identity headers** (`X-User-*`); a dedicated backend Authentik
> client for direct JWT validation is **deferred to G4**. This doc reflects those.

---

## 1. Two problems, two places

"OIDC" is really two concerns, solved in two places:

| Concern | When | Who | Status in repo |
|---|---|---|---|
| **Provision the OIDC client** in the IdP, get `client_id`/`client_secret`. | GitOps reconcile time. | **Crossplane** (Terraform provider → Authentik). | **Built.** |
| **Enforce authentication** (login flow, token validation) at request time. | Request time. | **Envoy Gateway** `SecurityPolicy` (+ backend). | Pattern exists for other apps; not yet wired for KubeSandbox sessions. |
| **Authorize per-session ownership** (this user owns this session). | Request time. | **Backend** (via ext-authz). | **Not built — the key gap.** |

The first two halves hand off through a **Kubernetes Secret** of client
credentials. The third is KubeSandbox-specific and is where the real design work
is.

---

## 2. Provisioning (built) — Crossplane → Authentik

The repo already provisions Authentik OIDC clients with the Terraform provider,
two ways:

- **Reusable module** — `crossplane/.../authentik-oidc-app.yaml` is a ConfigMap
  Terraform module (`goauthentik/authentik ~> 2025.2.0`) that creates an
  `authentik_provider_oauth2` + `authentik_application` and outputs
  `client_id` / `client_secret` (confidential client, strict redirect URI).
- **Frontend instance** — `kubesandbox-frontend/.../kubesandbox-frontend-auth.yaml`
  is a `tf.upbound.io/Workspace` that creates the **`kubesandbox-frontend`** client
  (redirect `https://kubesandbox.com/auth/callback`) and **writes
  `kubesandbox-frontend-client-secret` into the `kubesandbox` namespace** via
  `writeConnectionSecretToRef`. Authentik is at `https://auth{suffix}.jeremymr.dev`.

That connection Secret is the **contract** between provisioning and enforcement.

### To add: a backend client
The backend needs its own client (or to validate frontend-issued tokens). Mirror
the frontend Workspace to create a `kubesandbox-backend` client and write
`kubesandbox-backend-client-secret` to `kubesandbox`. Keep the
`authorization_flow` / `invalidation_flow` defaults already used in the repo.

---

## 3. Enforcement (to wire) — Envoy Gateway

Envoy Gateway's `SecurityPolicy` does edge OIDC natively and is mature here
(repo already uses `SecurityPolicy` for adguard, code-server, longhorn,
stirling-pdf). As of **Envoy Gateway v1.8.0 (May 2026)** OIDC renders a single
native `envoy.filters.http.oauth2` filter and supports gateway/route policy
merging (`MergeType`); **v1.7.0 (Feb 2026)** fixed discovered well-known
endpoints overriding an explicitly configured authorization endpoint.

Reuse the existing repo pattern. A SecurityPolicy attaches to a `Gateway` or
`HTTPRoute` and configures `provider.issuer`, `clientID`, a `clientSecret` ref
(the Crossplane-written Secret), `redirectURL`, and `logoutPath`. Model new
policies on `operators-helm/operators/*/resources/templates/*-security-policy.yaml`.

---

## 4. The central problem: per-session **ownership** authorization

Because the terminal is **ttyd reached by URL**
(`kubesandbox.com/s/{id}`, see architecture doc), edge auth must do more than
"is this a valid Authentik user." A plain OIDC SecurityPolicy would let **any**
authenticated user open **anyone's** session URL. We need: *is this authenticated
user the **owner** of the session `{id}`?*

### Decision: Option A — OIDC + ext-authz / ForwardAuth → backend
A **shared** SecurityPolicy on the session route (host `kubesandbox.com`, path
prefix `/s/`) first runs OIDC (cookie), then calls the backend authorization
endpoint with the original request path:

```
Browser ─▶ Envoy ─OIDC(cookie)─▶ ext-authz GET backend /authz   (X-Forwarded-Path: /s/{id})
                                   backend: resolve {id} → claim → 200 if ownerRef == token.sub else 403
                                 ─▶ (200) ─▶ ttyd
```

- Ownership logic lives in **one place** (the backend), next to the claims.
- No per-session secrets; revocation is immediate (delete the claim → 403/404).
- Path-based routing means **one** SecurityPolicy covers every session — no
  per-session policy to generate.
- Envoy passes identity to the backend via the forwarded token/claims.

> **Option B not chosen.** A backend-minted, session-scoped signed token validated
> by a JWT SecurityPolicy would avoid the callout but puts token issuance,
> lifetime, and revocation on us — no benefit here.

---

## 5. Edge OIDC vs. JWT — and the WebSocket caveat

| Mode | Mechanism | Use for |
|---|---|---|
| **Edge OIDC** | `oauth2` filter, cookie login flow | Frontend SPA, the session terminal (browser-driven). |
| **JWT validation** | `SecurityPolicy.jwt` vs JWKS | Backend API (`api.kubesandbox.com`), programmatic clients. |

**ttyd uses WebSockets.** Browsers can't set an `Authorization` header on a WS
upgrade — but the **OIDC session cookie rides the upgrade automatically**, so
cookie-based edge enforcement is the natural fit for the terminal and avoids
query-param token hacks. Confirm Envoy forwards the upgrade (and the ext-authz
callout runs) on the WS handshake for the session route.

---

## 6. Backend's role in auth

- **AuthN (G1):** identity comes from **Envoy-forwarded headers** (`X-User-Email`
  / `X-User-Name` / `X-User-Groups`, optional `X-User-Id`) injected after edge
  OIDC; the backend maps these to `sub → ownerRef`. **Direct Authentik OIDC/JWT
  validation in the backend** (for programmatic `api.kubesandbox.com` callers) is
  the **G4** hardening step, not required to ship G1.
- **AuthZ (sessions):** own the `/authz` ext-authz endpoint — resolve `{id}` from
  the forwarded path `/s/{id}`, allow iff the claim's `ownerRef == sub`.
- **Defense in depth:** because G1 trusts injected identity headers, the
  gateway→backend path **must** be locked down (NetworkPolicy / mTLS) so callers
  can't spoof `X-User-*`; once G4 lands, re-verify forwarded tokens directly.

Division of responsibility: **edge = "valid user," backend = "owns this session."**

---

## 7. Operational caveats (decreasing importance)

1. **GitOps ordering.** Authentik client (Secret) must exist before the
   SecurityPolicy is valid. Respect existing sync waves (Authentik module 15,
   frontend auth 17, session HTTPRoute 18, XRD/composition 25); gate the backend
   and any session SecurityPolicy behind their Secret/CRD readiness.
2. **Issuer correctness / split-DNS.** `issuer` must match the `iss` in issued
   tokens. The homelab uses `auth.jeremymr.dev`; if internal vs external issuer
   URLs differ, pin endpoints (mind the v1.7 discovery-override fix).
3. **TLS to Authentik.** If Authentik uses an internal CA, add a
   `BackendTLSPolicy` so Envoy trusts it during discovery/token exchange.
4. **Cookie/refresh lifetimes ≥ session TTL.** Long terminal sessions shouldn't
   have their auth cookie expire mid-use; ensure refresh/cookie lifetimes exceed
   `ttlMinutes`.
5. **Redirect URI exactness.** Authentik redirect URI and Envoy `redirectURL`
   must match character-for-character (the classic trailing-slash failure).
6. **Cross-namespace Secret refs.** If a SecurityPolicy references the client
   Secret across namespaces, a `ReferenceGrant` may be required.

---

## 8. Decision checklist

- [x] **G1 identity** chosen: backend **trusts Envoy-forwarded `X-User-*` headers**; lock the gateway→backend path. **NetworkPolicy lock added (rev 5, default-on)** — ingress to the backend is restricted to `envoy-gateway-system`.
- [ ] **(G4)** Add `kubesandbox-backend` Authentik client + `kubesandbox-backend-client-secret` (mirror frontend Workspace) for direct JWT validation on `api.kubesandbox.com`.
- [x] Authz model chosen: **Option A** (ext-authz to backend).
- [ ] Backend `/authz` endpoint: path `/s/{id}` → claim → `ownerRef == sub`.
- [~] One shared session SecurityPolicy: **authored rev 5** (`securitypolicy-session.yaml`, default-off) — OIDC + JWT claimToHeaders + ext-authz, attached to all `/s/` routes by label. **Still to do: live spike** to verify field names + OIDC-token-is-a-JWT + ext-authz on the WS upgrade, then enable.
- [ ] JWT SecurityPolicy for `api.kubesandbox.com`.
- [ ] Negative test: user B cannot open user A's session.
- [ ] Issuer matches token `iss` (split-DNS); `BackendTLSPolicy` if internal CA.
- [ ] Cookie/refresh lifetime ≥ max `ttlMinutes`.
- [ ] GitOps ordering: client Secret → SecurityPolicy; readiness-gated.

---

## 9. Sources

- [OIDC Authentication — Envoy Gateway docs](https://gateway.envoyproxy.io/docs/tasks/security/oidc/)
- [SecurityPolicy — Envoy Gateway docs](https://gateway.envoyproxy.io/docs/concepts/gateway_api_extensions/security-policy/)
- [Envoy Gateway v1.8.0 release notes](https://gateway.envoyproxy.io/news/releases/notes/v1.8.0/)
- [Envoy Gateway v1.7.0 release notes](https://gateway.envoyproxy.io/news/releases/notes/v1.7.0/)
- [Combining OIDC and JWT authentication — Discussion #2425](https://github.com/envoyproxy/gateway/discussions/2425)
- In-repo references: `operators-helm/operators/kubesandbox-frontend/pre-resources/templates/kubesandbox-frontend-auth.yaml`, `operators-helm/operators/crossplane/resources/templates/authentik-oidc-app.yaml`, `operators-helm/operators/*/resources/templates/*-security-policy.yaml`
