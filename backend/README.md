# KubeSandbox Backend

The backend control service for KubeSandbox. It manages `KubeSandboxSession`
claims (`platform.kubesandbox.com/v1alpha1`) via the client-go **dynamic
client** — claims are the source of truth, there is no application database. It
also runs a **hot warm-pool** so creates are a metadata-only assignment, an
**assignment queue** for when the pool is momentarily empty, a **TTL controller**
that reaps expired sessions, and the **backend-owned OIDC/PKCE + ext-authz** flow
that guards the browser terminal.

Built with Go 1.25 and Gin.

## API

Health probes are served at the root (the kubelet hits the pod directly):

| Method | Path | Description |
|---|---|---|
| GET | `/health`, `/healthz` | Liveness/readiness probe. |

The product API is served under `/api` (the Envoy HTTPRoute forwards the `/api`
prefix unchanged) and requires identity headers injected by the gateway's JWT
policy:

| Method | Path | Description |
|---|---|---|
| POST | `/api/sessions` | Create the caller's session. Body: `{ "ttlMinutes": 60 }` (optional; `workspaceImage`/`starterLabRef` are honored only on the legacy direct-create path). Assigns a warm pool member → `201`. Empty pool → `202` with a queue position. Caller already has one → `409 session_exists`. |
| GET | `/api/sessions` | List the caller's sessions (0 or 1). |
| GET | `/api/sessions/{id}` | Get one session the caller owns. |
| DELETE | `/api/sessions/{id}` | Delete one session the caller owns. |
| GET | `/api/sessions/{id}/events` | SSE stream of the session's lifecycle. |
| GET | `/api/queue` | The caller's place in line, or `404` when not queued. |
| GET | `/api/queue/events` | SSE stream of queue progress: `queued` position updates, then a terminal `assigned` (with the session) or `error`. |

The gateway also routes two backend-owned, non-`/api` paths (no identity
headers — see [Session auth](#session-authentication)):

| Method | Path | Description |
|---|---|---|
| GET | `/authz`, `/authz/*` | Ext-authz endpoint for `/s/{id}` terminal routes: reads the session cookie, redirects to Authentik if absent, else checks ownership. |
| GET | `/oauth2/callback` | OIDC callback: verify state + nonce, exchange the code, set the session cookie, redirect to the original URL. |

`{id}` is the public id `{namespace}-{name}` (e.g. `playground-s-pool-1a2b3c4d5e`),
which is also the routing path: the terminal lives at `{PublicBaseURL}/s/{id}`.

**One sandbox per user.** At assignment the backend first creates a per-owner
marker `ConfigMap` named deterministically from the owner subject
(`sbxowner-` + first 16 hex chars of `sha256(ownerRef)`). The Kubernetes API
server enforces the singleton atomically: a second create for the same user
fails `AlreadyExists` → `409` — no list-then-create race. The marker is released
when the session is deleted or reaped, so re-creation is available as soon as
cleanup finishes.

Ownership: list/get/delete/events are scoped to the caller (`ownerRef == sub`).
Unknown, unowned, and malformed ids all return `404` (no existence leak).

## Hot warm-pool & assignment

When `POOL_ENABLED` (default), the request path never runs a cold build:

- **Pool manager** (`kubernetes/pool.go`) keeps `POOL_TARGET_WARM` unclaimed,
  `Ready` sandboxes provisioned, within a `POOL_MAX_TOTAL` concurrent ceiling.
  It is watch-driven (a watch on managed claims pokes a level-based reconcile)
  with a periodic resync backstop. Each pass admits queued users, recycles stale
  members, trims overshoot, refills to target, and GCs orphaned owner markers.
- **Assignment** (`kubernetes/assign.go`) reserves the owner marker, then claims
  a warm member with an optimistic-concurrency `Update` (loser of a race retries
  the next member). TTL starts here: `spec.expiresAt` is stamped at hand-over.
- **Queue** (`kubernetes/queue.go`, `kubernetes/queue_redis.go`) is the FIFO
  waiting line behind the `Queue` interface. With `REDIS_ADDR` set it is
  Redis-backed (shared across replicas; SSE events relay via pub/sub) and the
  backend scales horizontally — the pool reconcile loop then runs on a single
  leader elected via a coordination Lease. Without Redis it falls back to the
  in-memory single-replica queue. If the pool is empty the request is enqueued
  and admitted by the pool manager as members become `Ready`; the one-per-user
  invariant lives in the marker, not the queue. See
  `docs/redis-queue-horizontal-scaling.md`.

With `POOL_ENABLED=false` the backend falls back to legacy direct creates: a
claim named deterministically from the owner (`s-` + `sha256(ownerRef)[:16]`),
built cold on request.

## TTL & cleanup

The `TTLController` (`kubernetes/cleanup.go`) runs every `TTL_CLEANUP_INTERVAL`
and deletes any managed claim past its expiry (preferring `spec.expiresAt`, then
`status.expiresAt`, then `creationTimestamp + ttlMinutes`). Unclaimed warm
members are skipped (their clock starts at assignment). Deletes use background
propagation so a stuck finalizer can't stall the loop; the sweep CronJob in the
chart is the backstop for orphaned namespaces.

## Session authentication

`/api` trusts identity from the gateway-injected `X-User-*` headers. The terminal
routes (`/s/{id}`) are guarded by the backend itself:

- `/authz` (ext-authz) reads the signed session cookie. No/invalid cookie → `302`
  to Authentik with PKCE and a signed, nonce-bound state. Valid cookie →
  ownership check (`ownerRef == cookie.sub`): owner `200`, non-owner/unknown
  `403`, backend error `503` (fail closed).
- `/oauth2/callback` verifies the state token and a one-time nonce cookie (OAuth
  login-CSRF protection), exchanges the code over server-to-server TLS, and sets
  an `HttpOnly Secure SameSite=Lax` HMAC-signed session cookie.

Session and state tokens are HMAC-SHA256 signed with `SESSION_SECRET`; the flow
is stateless (code_verifier and original URL travel in the signed state).

## Configuration (env)

All values are injected by the `kubesandbox-backend` Helm chart; every option has
a safe default so the binary also runs locally.

| Var | Default | Notes |
|---|---|---|
| `PORT` | `8080` | Listen port. |
| `NAMESPACE` | `playground` | Namespace where claims and markers are created. |
| `PUBLIC_BASE_URL` | `https://kubesandbox.com` | Origin used to build session URLs. |
| `USER_EMAIL_HEADER` | `X-User-Email` | Identity header (subject fallback). |
| `USER_NAME_HEADER` | `X-User-Name` | Display name header. |
| `USER_GROUPS_HEADER` | `X-User-Groups` | Comma-separated groups header. |
| `USER_ID_HEADER` | `X-User-Id` | Stable subject (preferred over email). |
| `TTL_CLEANUP_INTERVAL` | `1` | Minutes between TTL sweeps. |
| `POOL_ENABLED` | `true` | Hot pool + assignment path; `false` = legacy direct creates. |
| `POOL_TARGET_WARM` | `2` | Unclaimed, `Ready` sandboxes to keep warm. |
| `POOL_MAX_TOTAL` | `60` | Concurrent-session ceiling (warm + live). |
| `POOL_MAX_WARM_AGE_HOURS` | `24` | Age past which an unclaimed member is recycled. |
| `POOL_RESYNC_SECONDS` | `30` | Pool manager periodic reconcile interval. |
| `OIDC_ISSUER` | — | Authentik provider issuer URL (backend client). |
| `OIDC_CLIENT_ID` | — | OAuth2 client_id registered in Authentik. |
| `OIDC_CLIENT_SECRET` | — | OAuth2 client_secret (from a K8s Secret). |
| `OIDC_REDIRECT_URI` | — | Callback URL (e.g. `.../oauth2/callback`). |
| `OIDC_AUTH_ENDPOINT` | `{issuer}authorize/` | Override if not derivable from issuer. |
| `OIDC_TOKEN_ENDPOINT` | `{issuer}token/` | Override if not derivable from issuer. |
| `SESSION_SECRET` | — | HMAC key for session + state tokens. Required when session auth is enabled. |
| `SESSION_COOKIE_NAME` | `kubesandbox_session` | Session cookie name. |
| `SESSION_COOKIE_DOMAIN` | host of `PUBLIC_BASE_URL` | Cookie Domain attribute. |
| `SESSION_MAX_AGE_SECONDS` | `28800` | Session cookie lifetime (8h). |

`tenantRef = ownerRef = subject` (1 tenant = 1 user).

## Build & run

```sh
go mod tidy            # generate go.sum (network required, run once and commit)
go build ./...
go vet ./...
go test ./...
go run ./cmd/server    # uses your kubeconfig when out of cluster
```

The container image is built from `backend/Dockerfile` by
`.github/workflows/backend.yml` and pushed as `jurassicjey/kubesandbox-backend`.

## Security note

Identity on `/api` is trusted from injected headers, so the gateway→backend path
must be locked down (the chart's `networkPolicy`, default-on, allows ingress only
from `envoy-gateway-system`) so callers cannot spoof `X-User-*`.

## Related docs

- [`../docs/reference/backend-architecture.md`](../docs/reference/backend-architecture.md) — architecture
- [`../docs/reference/auth-design.md`](../docs/reference/auth-design.md) — auth & authz
- [`../docs/reference/hot-pool-design.md`](../docs/reference/hot-pool-design.md) — hot warm-pool design
