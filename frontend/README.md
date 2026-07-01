# KubeSandbox Frontend (G5)

React 19 + Vite + TypeScript SPA for KubeSandbox. Sign in with Authentik, manage
ephemeral vcluster sandboxes, and hand off to the browser terminal.

**Architecture:** see [`../docs/06-frontend-architecture.md`](../docs/06-frontend-architecture.md).

## Stack

React 19 · Vite · TypeScript · Tailwind + shadcn-style UI · TanStack Query ·
Zod · oidc-client-ts (Auth Code + PKCE) · react-router-dom.

## Develop

```bash
npm install
cp .env.example .env.local   # adjust as needed
npm run dev                  # http://localhost:5173 (proxies /api to VITE_DEV_PROXY_TARGET)
```

`npm run typecheck` · `npm run build` · `npm run preview`.

## How auth works (short version)

- **`/api`** uses a **JWT bearer** the SPA obtains via its own PKCE flow. There
  is no cookie fallback. Requires the `/api` JWT SecurityPolicy to trust the
  `kubesandbox-frontend` issuer (see doc §4.1 / §8 action items).
- **`/s/{id}` (terminal)** is served by the backend with its **own** OIDC/PKCE +
  ext-authz. The SPA just opens `${VITE_PUBLIC_BASE_URL}/s/{id}` in a new tab.
- **Identity key** is the Authentik `sub` (hashed uid), not email.
- **SSE** (`/api/sessions/:id/events`) is read with a `fetch` stream (not
  `EventSource`) so the bearer can be attached; polling is the fallback.

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
