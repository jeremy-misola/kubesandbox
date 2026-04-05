# KubeSandbox Backend

A Go backend service for managing KubeSandbox sessions. This service provides a REST API to create, list, delete, and monitor Kubernetes sandbox sessions using the KubeSandboxSession CRD.

## Features

- **Session Management**: Create, list, get, and delete sandbox sessions
- **Real-time Updates**: Server-Sent Events (SSE) for session status changes
- **Kubeconfig Download**: Retrieve vcluster kubeconfig for sessions
- **TTL Cleanup**: Background worker to cleanup expired sessions
- **Authentication**: Header-based authentication (via Envoy Gateway)
- **Kubernetes Native**: Uses the KubeSandboxSession CRD

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/sessions` | List all sessions for the authenticated user |
| POST | `/api/v1/sessions` | Create a new session |
| GET | `/api/v1/sessions/{name}` | Get a specific session |
| DELETE | `/api/v1/sessions/{name}` | Delete a session |
| GET | `/api/v1/sessions/{name}/kubeconfig` | Get vcluster kubeconfig |
| GET | `/api/v1/sessions/events` | SSE stream for session updates |
| GET | `/api/v1/user` | Get current authenticated user |
| GET | `/health` | Health check endpoint |

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `NAMESPACE` | `playground` | Namespace for sessions |
| `USER_EMAIL_HEADER` | `X-User-Email` | Header for user email |
| `USER_NAME_HEADER` | `X-User-Name` | Header for user name |
| `USER_GROUPS_HEADER` | `X-User-Groups` | Header for user groups |
| `TTL_CLEANUP_INTERVAL` | `1` | Cleanup interval in minutes |

## Development

```bash
# Run locally
go run ./cmd/main.go

# Build
go build -o backend ./cmd/main.go

# Build Docker image
docker build -t kubesandbox-backend .
```

## Authentication

The backend expects authentication headers to be injected by Envoy Gateway. In development mode, you can use:

```bash
# Development mode (bypasses auth)
curl -H "X-Development-Mode: true" http://localhost:8080/api/v1/sessions

# Mock user
curl -H "X-Mock-User-Email: user@example.com" http://localhost:8080/api/v1/sessions
```

## RBAC

The backend requires the following Kubernetes permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubesandbox-backend
rules:
  - apiGroups: ["platform.kubesandbox.com"]
    resources: ["kubesandboxsessions"]
    verbs: ["get", "list", "watch", "create", "delete"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list"]
```

## Example Session Creation

```bash
curl -X POST http://localhost:8080/api/v1/sessions \
  -H "Content-Type: application/json" \
  -H "X-Development-Mode: true" \
  -d '{
    "name": "my-session",
    "tenantRef": "demo-tenant",
    "profile": "starter",
    "ttlMinutes": 60
  }'
```

## License

MIT