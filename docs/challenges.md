# KubeSandbox — Guided Challenges

**Status:** living reference (backend implemented 2026-07-05; frontend not yet built)
**Audience:** challenge authors, platform engineers
**Related:** [`reference/backend-architecture.md`](./reference/backend-architecture.md) ·
[`history/challenges-backend-architecture.md`](./history/challenges-backend-architecture.md) ·
[`history/challenges-backend-handoff.md`](./history/challenges-backend-handoff.md) ·
[`reference/challenge-scenario-catalog.md`](./reference/challenge-scenario-catalog.md)

---

## 1. What challenges are

Challenges are guided, self-contained Kubernetes exercises that run inside a
user's sandbox (vcluster). Each challenge:

1. **Seeds** a broken or incomplete Kubernetes state into the user's vcluster
   (manifests applied via server-side apply).
2. **Presents** a task — fix the RBAC, write a NetworkPolicy, debug why a pod
   won't start, etc.
3. **Grades** the user's work on demand — declarative checks read live tenant
   state and report pass/fail per step.
4. **Resets** in seconds — deletes seeded state and re-applies, so the user can
   drill the same exercise repeatedly without creating a new sandbox.

Challenges are the product's main differentiator: instant sandbox hand-off (hot
pool) + disposable, reset-in-seconds practice environments for rapid drilling.

---

## 2. How a challenge is authored

Every challenge lives as a directory in git under
`kubesandbox-charts/kubesandbox/charts/kubesandbox-challenges/challenges/`:

```
challenges/<id>/
├── challenge.yaml        # metadata + validation checks
└── seed/                 # manifests applied into the tenant vcluster
    ├── 00-namespace.yaml
    ├── 10-serviceaccount.yaml
    ├── 20-role.yaml
    └── 30-deployment.yaml
```

The directory name **must** match the `id` field in `challenge.yaml`.

### 2.1 `challenge.yaml`

```yaml
apiVersion: content.kubesandbox.com/v1
id: troubleshoot-rbac-permissions-for-a-failing-deployment
title: Troubleshoot RBAC Permissions for a Failing Deployment
description: >
  The metrics-agent Deployment in the monitoring namespace never becomes
  ready: its container polls the Kubernetes API for pods, but the
  ServiceAccount it runs as lacks the permission to list them. A Role with
  the right permissions already exists — fix the issue by binding it to the
  ServiceAccount. Do not modify the Deployment.
category: rbac
difficulty: medium
estMinutes: 15
tags: [rbac, serviceaccount, rolebinding, ckad]
hints:
  - Check why the pod is not ready — kubectl describe pod ...
  - kubectl auth can-i list pods -n monitoring --as=...
  - A Role named pod-reader already exists in the monitoring namespace.

validate:
  - id: serviceaccount-can-list-pods
    description: The metrics-agent ServiceAccount can list pods in monitoring
    checks:
      - type: subjectCan
        serviceAccount: {namespace: monitoring, name: metrics-agent}
        access: {verb: list, resource: pods, namespace: monitoring}
  - id: rolebinding-references-role
    description: A RoleBinding in monitoring grants the pod-reader Role
    checks:
      - type: resourceExists
        target:
          apiVersion: rbac.authorization.k8s.io/v1
          kind: RoleBinding
          namespace: monitoring
        where:
          - path: .roleRef.name
            equals: pod-reader
  - id: deployment-recovers
    description: The metrics-agent Deployment reports all replicas available
    checks:
      - type: deploymentAvailable
        target: {namespace: monitoring, name: metrics-agent}
```

| Field | Required | Description |
|---|---|---|
| `apiVersion` | yes | Must be `content.kubesandbox.com/v1`. Unknown versions are quarantined. |
| `id` | yes | Lowercase alphanumeric/hyphen, max 128 chars. Must match the directory name. |
| `title` | yes | Human-readable title. |
| `description` | yes | The task the user must complete. |
| `category` | yes | One of: `rbac`, `networkpolicy`, `workloads`, `config`, `scheduling`, `storage-lite`, `troubleshooting`. |
| `difficulty` | yes | One of: `easy`, `medium`, `hard`. |
| `estMinutes` | no | Estimated completion time. |
| `tags` | no | Free-form tags for filtering. |
| `hints` | no | Progressive hints, revealed by the UI. |
| `heavy` | no | Reserved for future slow-path (Helm) bundles. Rejected by the v1 validator. |
| `validate` | yes | One or more steps, each with one or more checks. |

### 2.2 Seed manifests (`seed/` directory)

The `seed/` directory contains the Kubernetes manifests that are applied into
the user's vcluster at assignment time. These manifests create the broken or
incomplete state the user must fix.

**Apply order:** Namespaces first, then RBAC objects, then everything else.
Within each phase, files are applied in alphabetical order — the numeric
prefixes (`00-`, `10-`, `20-`) are the tiebreaker.

Every seed object automatically gets the label
`kubesandbox.com/challenge: <id>` injected by the loader. This label is what
the reset endpoint uses to find and delete seeded state.

---

## 3. Check types (v1)

Checks are declarative and read-only — the grader never mutates tenant state.

| Type | What it asserts |
|---|---|
| `resourceExists` | An object (by name or labelSelector) exists. Optional `where` predicates filter on field values. |
| `resourceAbsent` | No object of that kind/namespace/name exists. |
| `fieldEquals` | A JSONPath into a named object equals a literal value. |
| `fieldMatches` | A JSONPath into a named object matches a regex. |
| `podReady` | A pod's `status.conditions` report `Ready=True`. |
| `deploymentAvailable` | A deployment's `status.availableReplicas >= minAvailable`. |
| `subjectCan` | A ServiceAccount can perform the given action (SelfSubjectAccessReview). |
| `subjectCannot` | A ServiceAccount cannot perform the given action. |

All steps are evaluated independently (no short-circuit). A step passes iff all
its checks pass. The overall grade passes iff all steps pass.

---

## 4. How challenges are delivered

Challenges are delivered as **ConfigMaps via GitOps** — no backend rebuild or
restart needed to add or update a challenge.

1. **Author** creates a directory under `challenges/<id>/` with `challenge.yaml`
   and `seed/*.yaml`.
2. **CI** runs `cmd/validate-bundles` (the same Go validator the backend uses)
   to lint every bundle pre-merge.
3. **ArgoCD** syncs the `kubesandbox-challenges` chart, which renders each
   challenge directory into **one ConfigMap** in the backend namespace:
   - Name: `sbxchallenge-<id>`
   - Label: `kubesandbox.com/challenge-bundle: "true"`
   - Keys: `challenge.yaml` + each seed file basename
4. **Backend** runs a namespace-scoped, label-filtered ConfigMap informer
   (`ContentStore`) that watches for add/update/delete and rebuilds the
   in-memory catalog. Invalid bundles are **quarantined** (skipped + logged +
   metric incremented) — never fatal.

End-to-end: git push → CI passes → ArgoCD syncs → catalog updated in seconds.

---

## 5. How challenges are validated

Validation happens **twice, deliberately**:

### 5.1 CI (pre-merge)

The `challenges.yml` GitHub Actions workflow runs `cmd/validate-bundles` against
every bundle in the chart. This catches:

- Schema errors (missing fields, unknown check types, unknown categories)
- Unsupported kinds (not in the `knownKinds` table in `content/bundle.go`)
- Namespacing violations (cluster-scoped kinds other than Namespace are rejected)
- Missing `metadata.name` or `metadata.namespace` on namespaced objects
- Missing `requests`/`limits` on containers (required by the host ResourceQuota)
- Non-compliant security contexts (PSA `restricted` is enforced by the vcluster)
- The `--request-timeout` / `--kubeconfig` / `--server` kubectl flag trap (see §7)

### 5.2 Backend (at watch time)

The `ContentStore` re-validates every bundle when the ConfigMap informer fires.
If a bundle fails validation, it is **quarantined** — skipped from the catalog,
logged, and a `content.bundle_invalid` gauge is incremented. This is the guard
against content/backend schema skew: a bundle with an unknown `apiVersion` is
never guessed at.

---

## 6. How challenges work at runtime

```
POST /api/sessions {ttlMinutes, challengeId}
        │
        ▼
  Assign() — stamps challengeId onto the claim (same CAS as ownership)
        │
        ▼
  201 {challenge: {id, seedState: "pending"}}
        │
        ▼
  Seeder (async, in-process):
    1. Fetch vc-{ns}-{name}-vcluster Secret (tenant kubeconfig)
    2. Build tenant dynamic client
    3. Server-side-apply seed manifests (SSA, idempotent)
    4. Flip seed-state: pending → seeding → seeded
        │
        ▼
  Frontend SSE sees annotation flip → "Ready"
        │
        ▼
  POST /api/sessions/{id}/challenge/grade  → per-step pass/fail
  POST /api/sessions/{id}/challenge/reset  → delete labeled state, re-seed
```

### Key properties

- **Seeding is async and off the request path.** `Assign()` returns immediately
  (sub-second). The seeder runs in the background.
- **Crash-safe.** Seed state lives in claim annotations. On restart, the seeder
  reconciles any claim with `seed-state ∉ {seeded, failed}` and converges via
  idempotent SSA.
- **Failure ladder.** 3 in-place retries → recycle the member + re-assign once
  → fail closed with the marker released. The user never sees a half-seeded
  cluster.
- **Reset is cheap.** Deletes everything labeled `kubesandbox.com/challenge=<id>`
  in the tenant, waits for deletion, re-seeds. No pool interaction, no new
  sandbox. ~50 seconds (dominated by namespace termination).
- **Grading is on-demand and read-only.** A handful of GETs against the tenant
  API — zero host control-plane load. Rate-limited to one grade per 2 seconds
  per session.

---

## 7. Authoring constraints

Every seed manifest must respect these rules (enforced by the validator):

| Constraint | Why |
|---|---|
| **Kind must be in the known-kinds table** | No discovery/RESTMapper machinery in the seeder or grader. Extend `knownKinds` in `backend/internal/content/bundle.go` if a new kind is genuinely needed. |
| **Everything namespaced (or a Namespace)** | No cluster-scoped objects besides Namespaces may leak into a tenant. |
| **`metadata.name` required on every object** | SSA requires a name. |
| **`metadata.namespace` required on namespaced objects** | Explicit, never implicit. |
| **`requests` and `limits` on every container** | The host session namespace carries a ResourceQuota; admission rejects pods without them. |
| **PSA `restricted`-compliant `securityContext`** | The vcluster enforces the restricted Pod Security Standard. Use `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, `capabilities.drop: ["ALL"]`, `seccompProfile: {type: RuntimeDefault}`. |
| **No `--request-timeout`, `--kubeconfig`, or `--server` in kubectl commands** | **Any command-line flag makes kubectl skip its in-cluster-config fallback and dial `localhost:8080`** (verified live 2026-07-05). Use the probe's own `timeoutSeconds` instead. |
| **Bundle size < 256 KB** | Well under the ~1 MiB ConfigMap ceiling. Large payloads belong in an image or object store referenced by the manifests, not embedded in them. |

### The `--request-timeout` trap (important)

This is the most common authoring mistake. If a seed manifest's `command` or
`args` contains `kubectl get pods --request-timeout=5s`, the probe will **never
work** — even after the user applies the correct fix — because:

- `--request-timeout` is classified by kubectl as a "connection override" flag.
- Any connection override flag causes kubectl to skip the in-cluster-config
  auto-detection path.
- Without in-cluster config, kubectl falls back to dialing `localhost:8080`.
- The vcluster API server is not on localhost:8080, so every call fails.

**Fix:** Drop the flag and rely on the probe's own `timeoutSeconds` field, or
the container's resource limits, to bound execution time.

---

## 8. How to add a new challenge

1. **Pick an id** — lowercase alphanumeric/hyphen, matches the directory name.
2. **Create the directory** under `challenges/<id>/`.
3. **Write `challenge.yaml`** — metadata, hints, and validation checks.
4. **Write seed manifests** in `seed/` — the broken state the user must fix.
   Use numeric prefixes for apply ordering.
5. **Run the validator locally** — `go run ./backend/cmd/validate-bundles/ ./kubesandbox-charts/kubesandbox/charts/kubesandbox-challenges/challenges/`
6. **Commit and push** — CI runs the same validator. ArgoCD syncs the ConfigMap.
   The backend picks it up within seconds. No backend rebuild.

### What's not yet built (v1 scope cuts)

- **Frontend** — challenge catalog page, challenge session view (instructions +
  terminal + check panel), reset button. The API works (verified live), but
  there's no UI yet.
- **Progress persistence** — grade results are ephemeral. Phase 2 will store
  completion history in per-user ConfigMaps.
- **Heavy/Helm bundles** — excluded from v1. The `heavy` flag is reserved.
- **Exec/probe checks** — the `probe` check type (exec-based validation) is a
  v2 escape hatch, explicitly out of v1 to keep the grader read-only.
- **Hint economy** — hints are free and always visible. No scoring impact yet.