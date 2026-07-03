# KubeSandbox Frontend

React 19 + Vite + TypeScript SPA for KubeSandbox. Sign in with Authentik, manage
your single ephemeral vcluster sandbox, wait in line when the pool is full, and
hand off to the browser terminal.

**Architecture:** see [`../docs/reference/frontend-architecture.md`](../docs/reference/frontend-architecture.md).

## Stack

React 19 · Vite · TypeScript · Tailwind + shadcn-style UI · TanStack Query ·
Zod · oidc-client-ts (Auth Code + PKCE) · react-router-dom.

## Develop

```bash
npm install
cp .env.example .env.local   # adjust as needed
npm run dev                  # http://localhost:5173 (proxies /api to VITE_DEV_PROXY_TARGET)
```

`npm run typecheck` · `npm run build` · `npm run preview` · `npm run lint`.

## Routes

`/` landing/sign-in · `/auth/callback` PKCE completion · `/dashboard` sandbox +
queue · `/terminal/:id` terminal hand-off · `*` not found. `/dashboard` and
`/terminal/:id` are guarded by `ProtectedRoute`.

## How auth works (short version)

- **`/api`** uses a **JWT bearer** the SPA obtains via its own Authorization
  Code + PKCE flow (public `kubesandbox-frontend` client). There is no cookie
  fallback. The `/api` JWT `SecurityPolicy` trusts the frontend issuer as an
  additional provider so these tokens are accepted.
- **`/s/{id}` (terminal)** is served by the backend with its **own** OIDC/PKCE +
  ext-authz. The SPA either embeds it (same-origin) or opens
  `${VITE_PUBLIC_BASE_URL}/s/{id}` in a new tab.
- **Identity key** is the Authentik `sub` (hashed uid), not email.
- **Tokens** (incl. the refresh token) live in `sessionStorage` (per-tab, cleared
  on tab close). Renewal uses the refresh token (`offline_access` scope) via a
  direct token-endpoint fetch — no cross-site silent-renew iframe.
- **SSE** (`/api/sessions/:id/events` and `/api/queue/events`) is read with a
  `fetch` stream (not `EventSource`) so the bearer can be attached; polling is
  the fallback.

## Create → assign or queue

`POST /api/sessions` returns either `201` (a warm sandbox was handed over
immediately) or `202` (the pool was full — the caller is queued). While queued,
the dashboard follows `/api/queue/events` for live position updates and swaps the
queue card for the sandbox card the moment a member is assigned.

## Runtime config

Vite inlines `VITE_*` at build time, but production config is injected at
**runtime** by `docker-entrypoint.sh`, which writes `/config.js` from the
container's env (set by the Helm chart). `src/config.ts` reads
`window.__ENV` first, then `import.meta.env`, then defaults. One image, many
environments.

## Container

`Dockerfile` builds the bundle and serves it with nginx on **:8080** with
`/health/livez` + `/health/readyz` and SPA fallback. CI
(`.github/workflows/frontend.yml`) builds this and pushes
`jurassicjey/kubesandbox-frontend`.
