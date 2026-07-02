# KubeSandbox Backend

The backend control service (G1) for KubeSandbox. It creates, lists, reads, and
deletes `KubeSandboxSession` claims (`platform.kubesandbox.com/v1alpha1`) via the
client-go **dynamic client**. Claims are the source of truth — there is no
application database. Identity comes from Envoy-forwarded `X-User-*` headers.

## API

Health probes are served at the root (the kubelet hits the pod directly):

| Method | Path | Description |
|---|---|---|
| GET | `/health`, `/healthz` | Liveness/readiness probe. |

The product API is served under `/api` (the Envoy HTTPRoute forwards the `/api`
prefix unchanged) and requires identity headers:

| Method | Path | Description |
|---|---|---|
| POST | `/api/sessions` | Create the caller's session. Body: `{ "profile": "standard", "ttlMinutes": 60, "workspaceImage": "...", "starterLabRef": "..." }`. Returns `409 session_exists` if the caller already has one (including one still tearing down). |
| GET | `/api/sessions` | List the caller's sessions (0 or 1). |
| GET | `/api/sessions/{id}` | Get one session the caller owns. |
| DELETE | `/api/sessions/{id}` | Delete one session the caller owns. |
| GET | `/api/sessions/{id}/events` | SSE stream of the session's lifecycle. |

`{id}` is the public id `{namespace}-{name}` (e.g. `playground-s-1a2b3c4d5e6f7a8b`),
which is also the routing path: the terminal lives at `{PublicBaseURL}/s/{id}`.

**One sandbox per user.** The claim name is derived deterministically from the
owner subject (`s-` + first 16 hex chars of `sha256(ownerRef)`), so each user
maps to exactly one claim name. The Kubernetes API server enforces the
singleton atomically: a second create for the same user fails with
`AlreadyExists` → `409` — no list-then-create race. While a deleted session is
still tearing down, the name remains occupied and re-creation returns `409`
until cleanup completes.

Ownership: list/get/delete/events are scoped to the caller (`ownerRef == sub`).
Unknown, unowned, and malformed ids all return `404` (no existence leak).

## Configuration (env)

| Var | Default | Notes |
|---|---|---|
| `PORT` | `8080` | Listen port. |
| `NAMESPACE` | `playground` | Namespace where claims are created. |
| `PUBLIC_BASE_URL` | `https://kubesandbox.com` | Origin used to build session URLs. |
| `USER_EMAIL_HEADER` | `X-User-Email` | Identity header (primary subject fallback). |
| `USER_NAME_HEADER` | `X-User-Name` | Display name header. |
| `USER_GROUPS_HEADER` | `X-User-Groups` | Comma-separated groups header. |
| `USER_ID_HEADER` | `X-User-Id` | Optional stable subject (preferred over email). |
| `TTL_CLEANUP_INTERVAL` | `1` | Minutes; reserved for the G3 TTL loop (not run in G1). |

`tenantRef = ownerRef = subject` (1 tenant = 1 user).

## Build & run

```sh
go mod tidy            # generate go.sum (network required, run once and commit)
go build ./...
go vet ./...
go run ./cmd/server    # uses your kubeconfig when out of cluster
```

The container image is built from `backend/Dockerfile` by
`.github/workflows/backend.yml` and pushed as `jurassicjey/kubesandbox-backend`.

## Scope

This is **G1 only**. Per-session ownership ext-authz (`/authz`, G2), the in-process
TTL cleanup loop (G3), and the dedicated backend Authentik client / direct JWT
validation (G4) are intentionally not implemented here — see `docs/03-implementation-plan.md`.

## Security note

Identity is trusted from injected headers, so the gateway→backend path must be
locked down (NetworkPolicy / mTLS) so callers cannot spoof `X-User-*`.
