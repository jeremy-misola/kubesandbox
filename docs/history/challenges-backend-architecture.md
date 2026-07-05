# KubeSandbox — Guided Challenges: Backend Architecture

**Status:** proposed design (2026-07-04) — resolves the open questions in
[`challenge-seeding-design-note.md`](./challenge-seeding-design-note.md); not yet implemented.
**Audience:** Jeremy (platform owner) + future maintainers
**Related:** [`platform-limitations-and-challenges-decision.md`](./platform-limitations-and-challenges-decision.md) ·
[`challenge-seeding-design-note.md`](./challenge-seeding-design-note.md) ·
[`../reference/backend-architecture.md`](../reference/backend-architecture.md) ·
[`../reference/hot-pool-design.md`](../reference/hot-pool-design.md) ·
[`../challenges.md`](../challenges.md)

> **TL;DR:** Challenges are self-contained **bundles** (metadata + seed
> manifests + declarative validation checks) authored as directories in git,
> rendered **one ConfigMap per challenge** by a tiny content chart, synced by
> ArgoCD, and watched by the backend into an in-memory catalog — adding a
> challenge is a git push, no backend rebuild. The hot pool is
> untouched: `POST /api/sessions` with a `challengeId` assigns a generic warm
> member exactly as today, then an **async seeder** applies the bundle's
> manifests into the member's own vcluster through a new **tenant-client**
> capability (kubeconfig from the composed `vc-*` Secret, reachable in-cluster —
> verified against the live composition). Seed state is tracked in **claim
> annotations** (no new store, crash-safe, visible to the existing SSE stream).
> Grading is **on-demand only**: `POST …/challenge/grade` reads live tenant
> state and evaluates the bundle's checks; `…/challenge/reset` deletes seeded
> state and re-seeds in seconds — leaning into the platform's
> reset-and-try-again differentiator. Per-user completion history is designed
> but deferred to phase 2. Helm-heavy scenarios are out of the v1 path.

---

## 1. Scope and constraints this design answers to

**Content scope** (per the decision memo): the ~70% bucket — RBAC,
NetworkPolicy, ConfigMaps/Secrets, Deployments/StatefulSets, probes, QoS,
resource management, scheduling-within-one-node, debugging/troubleshooting.
Everything CKAD/KCNA-shaped in [`challenges.md`](../challenges.md). Permanently
out: kubeadm, real/multi-node scheduling, static pods, storage operators, host
namespaces. Two composition fixes this depends on are **already live**:
NetworkPolicy `sync.toHost` (policy challenges actually enforce) and
`podSecurityStandard: restricted` (host-namespace/hostPath content is refused
by the virtual API server itself).

**Architectural constraints** (from the seeding note and hot-pool design):

| Constraint | Consequence |
|---|---|
| Warm members must stay identical, ownerless, fungible | No challenge state before assignment; `pool.go`/`warm.go` unchanged |
| Control-plane ceiling ~low-teens total vclusters | One shared pool, never per-challenge pools; grading must not add idle load |
| Sub-second assignment is the product | Seeding is post-assign, async, off the request path |
| Seed budget ~1–2 s | Manifest-apply only; Helm-in-vcluster excluded from v1 (§10) |
| Partial seed must never reach the user | Idempotent retry → recycle-and-reassign → fail closed (§6.3) |
| Backend has no database | Seed state on the claim; catalog from watched ConfigMaps; progress (phase 2) in a ConfigMap |
| Single backend replica, in-memory queue | Seeder runs in-process; crash-safety via claim-annotation reconcile, like the pool manager |

## 2. Decisions

| Question (from the seeding note) | Decision |
|---|---|
| Manifest bundle format & storage | **ConfigMaps via GitOps**: challenge directories in git, rendered one-ConfigMap-per-challenge by a content chart, ArgoCD-synced, backend informer-watched — behind a `ContentStore` interface. Content changes need no backend rebuild. A shared validator CLI runs in content CI; the backend re-validates on watch and quarantines bad bundles. |
| Challenge selection | `CreateSessionRequest.challengeId`, stamped into the existing unused **`spec.starterLabRef`** claim field at assignment. No XRD migration. |
| Seeding trigger & state | Async, immediately after `Assign()`; state machine in **claim annotations**; startup reconcile resumes interrupted seeds. |
| Grading API | **On-demand only** (`POST /api/sessions/{id}/challenge/grade`), declarative checks evaluated against the tenant API. No background polling. |
| Partial-seed failure | Idempotent server-side-apply retries on the same member; then recycle the member and re-assign once; then fail with the marker released. |
| Tenant-client credentials | Read the composed `vc-{ns}-{name}-vcluster` kubeconfig Secret, authorized by a **per-session Role/RoleBinding composed into each session namespace** — no cluster-wide secret grant (§5.2). |
| Progress tracking | Designed, **phase 2** (§11). V1 grade results are ephemeral. |
| Slow-path (Helm) scenarios | **Excluded from v1** (§10). |

## 3. Architecture overview

```
POST /api/sessions {ttlMinutes, challengeId?}
        │
        ▼
  Assign() ────────────── unchanged: marker → CAS-claim oldest Ready member
        │                 + NEW: stamp spec.starterLabRef = challengeId
        │                        annotations: challenge-id, seed-state=pending
        ▼
  201 {…, challenge:{id, seedState:"pending"}}          (or 202 queued — the
        │                                                queue already carries
        │                                                the full request, so
        │                                                challengeId survives
        ▼                                                queue admission)
  Seeder (in-process, async) ── claim annotations drive a small state machine:
        │       pending → seeding → seeded | failed
        │
        │   1. fetch vc-{ns}-{name}-vcluster Secret (session namespace)
        │   2. build tenant dynamic client from kubeconfig bytes
        │   3. server-side-apply bundle manifests (all labeled
        │      kubesandbox.com/challenge=<id>), ordered, ~1–2 s
        │
        ▼
  Frontend SSE (existing claim watch) sees annotation flip → "Ready"
        │
        ▼
  POST /api/sessions/{id}/challenge/grade  → per-step pass/fail (tenant reads)
  POST /api/sessions/{id}/challenge/reset  → delete labeled state, re-seed
```

New backend packages, mirroring the existing layout:

| Package | Role |
|---|---|
| `internal/content` | `ContentStore` interface + ConfigMap-informer implementation; bundle schema types; watch-time validation + quarantine. |
| `internal/kubernetes/tenant.go` | Tenant-client factory: Secret → kubeconfig → `dynamic.Interface`, with a small per-session cache. |
| `internal/challenges` | `Seeder` (state machine + apply engine) and `Grader` (check evaluator). Both consume the tenant-client factory. |
| `internal/api/handlers/challenges.go` | Catalog, grade, reset endpoints. |

`assign.go` gains ~10 lines (stamp challenge fields); `pool.go`, `warm.go`,
`queue.go`, `cleanup.go`, `/authz` are untouched.

## 4. The challenge bundle

One directory per challenge in git (this repo, `challenges/<id>/`, or a
dedicated content repo later — the packaging below is identical either way):

```
challenges/troubleshoot-rbac-permissions-for-a-failing-deployment/
├── challenge.yaml        # metadata + validation checks
└── seed/
    ├── 00-namespace.yaml
    ├── 10-serviceaccount.yaml
    ├── 20-role.yaml
    └── 30-deployment.yaml
```

`challenge.yaml`:

```yaml
apiVersion: content.kubesandbox.com/v1     # bundle schema version, not a CRD
id: troubleshoot-rbac-permissions-for-a-failing-deployment
title: Troubleshoot RBAC Permissions for a Failing Deployment
description: >
  A Deployment is failing because its ServiceAccount lacks permissions to
  list pods. Fix the issue by binding an existing Role to the ServiceAccount.
category: rbac                     # rbac | networkpolicy | workloads | config |
difficulty: medium                 #   scheduling | storage-lite | troubleshooting
estMinutes: 15
tags: [rbac, serviceaccount, cka(d)]
hints:                             # optional, revealed progressively by the UI
  - Look at the pod logs of the failing Deployment.
  - kubectl auth can-i --as=system:serviceaccount:...

validate:
  - id: rolebinding-exists
    description: A RoleBinding grants the Role to the ServiceAccount
    checks:
      - type: resourceExists
        target: {apiVersion: rbac.authorization.k8s.io/v1, kind: RoleBinding,
                 namespace: monitoring, labelSelector: ""}
        where:                     # optional field predicates on the matched object(s)
          - path: .roleRef.name
            equals: pod-reader
  - id: deployment-recovers
    description: The Deployment reports all replicas available
    checks:
      - type: fieldEquals
        target: {apiVersion: apps/v1, kind: Deployment,
                 namespace: monitoring, name: metrics-agent}
        path: .status.availableReplicas
        equals: 1
```

**Check types (v1)** — deliberately declarative and read-only:

| Type | Semantics |
|---|---|
| `resourceExists` / `resourceAbsent` | Object (by name or labelSelector) is present/absent; optional `where` field predicates. |
| `fieldEquals` / `fieldMatches` | JSONPath into a named object equals a literal / matches a regex. |
| `podReady` / `deploymentAvailable` | Convenience wrappers over status conditions. |
| `subjectCan` / `subjectCannot` | `SelfSubjectAccessReview`-style check impersonating a ServiceAccount — needed for RBAC challenges where the *effect* matters, not the object shape. |

Anything not expressible declaratively (e.g. "the file was copied into the
pod") is a **v2 escape hatch** — a `probe` check that execs a command in a
bundle-defined checker pod — explicitly out of v1 to keep the grader read-only.

**Packaging and delivery (ConfigMaps via GitOps).** A minimal content chart
(`kubesandbox-charts/kubesandbox-challenges/`) renders each challenge
directory into **one ConfigMap** in the backend namespace — `challenge.yaml`
plus each `seed/*.yaml` as ConfigMap keys — labeled
`kubesandbox.com/challenge-bundle: "true"`. ArgoCD syncs it like everything
else, so *adding a challenge is a git push*: folder → merge → synced →
watched → in the catalog, seconds later, no backend rebuild or restart. The
backend's `ContentStore` runs a namespace-scoped informer on that label and
rebuilds the in-memory catalog on add/update/delete.

**Validation happens twice, deliberately:**

1. **Content CI** — a small validator CLI (same Go package the backend uses)
   lints every bundle pre-merge: schema, id-matches-directory, unknown check
   types, and a **must-apply gate**: every seed manifest passes kubeconform +
   a dry-run apply against a throwaway vcluster. Note the nuance: manifests
   must *apply* cleanly, but the resulting runtime state may be intentionally
   broken — that's what troubleshooting challenges are. A challenge whose
   puzzle is an *unappliable file* (e.g. the deprecated-API one) delivers that
   file as data (ConfigMap mounted into the shell), exempt from the gate.
2. **Backend at watch time** — re-validates each bundle and **quarantines**
   failures (skip + log + `content_bundle_invalid` metric) instead of
   crashing or failing the whole catalog. This is the guard against
   content/backend schema skew: bundles carry `apiVersion:
   content.kubesandbox.com/v1`, and unknown versions are quarantined, never
   guessed at.

Other rules: every seed object is namespaced (or a Namespace itself) and
within what the restricted profile allows; the loader injects the
`kubesandbox.com/challenge: <id>` label on every object (reset and cleanup
depend on it); a size guard warns at 256 KB per bundle — well under the ~1 MiB
ConfigMap ceiling, which one-challenge-one-ConfigMap keeps comfortably
irrelevant (a generous bundle is tens of KB; anything larger belongs in an
image or object store referenced *by* the manifests, not embedded in them).

## 5. The tenant-vcluster client (the one new capability)

### 5.1 Mechanics — verified against the live composition

The composition sets `exportKubeConfig.server` to the **in-cluster Service
DNS** (`https://{ns}-{name}-vcluster.{ns}-{name}:443`) and publishes the admin
kubeconfig as Secret `vc-{ns}-{name}-vcluster` in the session namespace. So the
backend pod can use the kubeconfig **as-is**: fetch Secret → `clientcmd.RESTConfigFromKubeConfig(bytes)`
→ `dynamic.NewForConfig`. No port-forwarding, no URL rewriting.

**Network path — verified:** the session NetworkPolicy's `podSelector` matches
only `app: shell`; the vcluster pod is unselected, therefore unrestricted.
Backend → vcluster:443 works today. If a default-deny is ever added to session
namespaces, an explicit allow (backend namespace → vcluster pod :8443) must
ship in the same change — worth a comment in the composition now.

`tenant.go` keeps a small LRU of built clients keyed by session id
(invalidated on session delete); grading a session repeatedly shouldn't
re-read the Secret every time.

### 5.2 RBAC — no cluster-wide secret grant

Today the backend has no access to session-namespace Secrets, and a
cluster-wide `get secrets` grant is unacceptable on a shared homelab cluster.
Instead the **composition composes two more objects** into every session
namespace:

- a `Role` granting `get` on secrets restricted with
  `resourceNames: [vc-{ns}-{name}-vcluster]`, and
- a `RoleBinding` to the backend ServiceAccount.

Blast radius: the backend can read exactly one Secret per session namespace —
the tenant kubeconfig it needs — and nothing anywhere else. The grant is
created and cascade-deleted with the session itself.

### 5.3 Security posture

The backend gains admin over **tenant virtual API servers only** — each of
which its user already fully controls, and whose damage potential is already
bounded by the live PSA-restricted + NetworkPolicy + ResourceQuota envelope.
The client never touches the host API with elevated rights. Bundle content is
trusted input (in-repo, CI-validated, no user-supplied manifests anywhere in
the path). Grading uses the same admin kubeconfig but only issues reads and
`SelfSubjectAccessReview`s by construction of the check types; if a v2 `probe`
check lands, revisit with a scoped in-vcluster ServiceAccount.

## 6. The seeder

### 6.1 State machine on the claim

Annotations (host-claim, backend-owned, survive restarts, already flow through
the SSE claim watch):

```
kubesandbox.com/challenge-id:  <bundle id>          # set at Assign
kubesandbox.com/seed-state:    pending | seeding | seeded | failed
kubesandbox.com/seed-attempts: <n>
```

`Assign()` stamps `challenge-id` + `seed-state: pending` in the same CAS
`Update` that claims the member — atomic with ownership, so there is no window
where a challenge session exists without its seed intent recorded.

**Every seed-state transition is itself a CAS update** (`Update` carrying the
observed `resourceVersion`, conflict → re-read and re-evaluate): the
`pending → seeding` flip acts as a lightweight lease. On today's single
replica this costs nothing; with multiple replicas it means two seeders
racing the same claim resolve at the API server — the loser sees the
conflict and moves on — and even a double-apply is harmless because SSA is
idempotent. This is the one deliberate piece of multi-replica future-proofing
in the seeder (§15).

The seeder is an in-process worker (started from `main.go` like the pool
manager): it receives sessions on a channel from the assign path for the fast
case, **and** reconciles on startup + a slow resync by listing claimed members
with `seed-state ∉ {seeded, failed}` — the same level-triggered pattern the
pool manager uses, which is what makes a crash mid-seed a non-event.

### 6.2 Applying

Server-side apply, `fieldManager: kubesandbox-seeder`, `force: true`, in
deterministic order: Namespaces → RBAC objects → everything else (the numeric
file prefixes in `seed/` are the tiebreaker). SSA makes the whole operation
idempotent — re-running a half-finished seed converges instead of erroring,
which is the entire retry story. Budget: 10 s per bundle (generous against the
measured 1–2 s), enforced with a context timeout.

### 6.3 Failure handling — the user never sees a half-seeded cluster

1. **Retry in place** (up to 3 attempts, short backoff): SSA is idempotent, so
   transient tenant-API hiccups (vcluster API still warming) just converge.
2. **Recycle and re-assign** (once): delete the claim (cascades the member),
   keep the owner marker, run `Assign()` again against the next warm member,
   seed that. This mirrors the existing claim-conflict retry philosophy.
3. **Fail closed:** set `seed-state: failed`, delete the claim, release the
   marker, surface a terminal error through the session SSE. The user can
   retry; they are never handed a cluster in an unknown state.

"Never sees" is enforced at the UX layer, not the gateway: `/authz` is
unchanged (the owner may technically open the terminal during the 1–2 s
seeding window), but the session payload exposes `challenge.seedState` and the
frontend gates the terminal + instructions panel on `seeded` — the same
pattern as the existing `workspaceReady` gate. Rejected alternative: teaching
`/authz` about seed state, which couples the auth hot path to challenge
machinery for a 2-second window.

### 6.4 Phase surfacing

No XRD change. The `Session` model derives a synthetic phase: claimed +
`challenge-id` set + not `seeded` → `phase: "Seeding"`, `message: "Preparing
your challenge…"`. The existing SSE claim watch already streams annotation
changes, so the frontend needs no new plumbing beyond rendering the phase.

## 7. Grading and reset

```
POST /api/sessions/{id}/challenge/grade        (owner-only, existing /api JWT)
→ 200 {
    challengeId: "...",
    pass: false,
    steps: [
      {id: "rolebinding-exists",   description: "...", pass: true},
      {id: "deployment-recovers",  description: "...", pass: false,
       message: "Deployment monitoring/metrics-agent: availableReplicas 0, want 1"}
    ],
    gradedAt: "2026-07-04T12:00:00Z"
  }
→ 409 if seed-state != seeded          → 404 if session has no challenge
```

Semantics: steps are independent and all evaluated (no short-circuit — the
user sees everything left to fix); a step passes iff all its checks pass;
`pass` is the conjunction. Failure `message`s name the object and the observed
vs expected value, never internals. Cost: a handful of GETs against the
*tenant* API — zero host-control-plane load, which is why on-demand-only is
safe at any catalog size. A per-session min-interval (2 s → `429`) guards
against a held-down retry key.

```
POST /api/sessions/{id}/challenge/reset
```

Deletes everything labeled `kubesandbox.com/challenge=<id>` in the tenant
(namespaces first — cascade does most of the work), waits for deletion, then
re-runs the seeder (`seed-state → pending`). Seconds end-to-end, no pool
interaction, no new sandbox. This is the "drill it again" affordance the
decision memo identifies as the product's structural edge — cheaper and faster
than delete-session + re-create, and it doesn't burn a warm member.

**User-created clutter caveat:** reset removes *seeded* state; objects the
user created without the label persist. That is acceptable (and arguably
correct) for v1; a `hard: true` variant that deletes all non-default
namespaces could come later.

## 8. API surface

| Endpoint | Auth | Behavior |
|---|---|---|
| `GET /api/challenges` | JWT | Catalog: id, title, description, category, difficulty, estMinutes, tags. Served from memory. |
| `GET /api/challenges/{id}` | JWT | Full metadata incl. step descriptions and hint count (hint *text* revealed via `?hints=n`). Never returns seed manifests or check internals. |
| `POST /api/sessions` | JWT | Existing endpoint; body gains optional `challengeId` (validated against the catalog → `400` on unknown id). Queue path carries it transparently. |
| `GET /api/sessions/{id}` | JWT | Gains `challenge: {id, title, seedState}` when applicable; synthetic `Seeding` phase. |
| `POST /api/sessions/{id}/challenge/grade` | JWT | §7. |
| `POST /api/sessions/{id}/challenge/reset` | JWT | §7. |

Model changes: `CreateSessionRequest.ChallengeID` (json `challengeId`;
`starterLabRef` kept as a deprecated alias), `Session.Challenge *ChallengeRef`.
Claim: `spec.starterLabRef` finally wired; annotations per §6.1. **No XRD
schema migration** — the field exists, unused, `maxLength: 128` (all current
catalog ids fit).

## 9. Failure modes

| Failure | Handling |
|---|---|
| Tenant API not yet accepting connections at seed time | Retry-in-place (§6.3.1) absorbs it; `workspaceReady: true` gating on assignment makes it rare. |
| Seed fails persistently (bad member) | Recycle + re-assign once, then fail closed with marker released (§6.3.2–3). |
| Backend crashes mid-seed | Startup reconcile finds `seed-state: seeding|pending`, SSA converges. |
| Pool empty on challenge create | Existing queue; `challengeId` rides the queued `CreateSessionRequest`; seeding happens at admission. |
| Grade called mid-seed / on non-challenge session | `409` / `404`. |
| User deletes seeded objects then grades | Checks fail with accurate messages — that's just grading working. |
| Kubeconfig Secret missing (composition drift) | Seed fails → recycle path; alert on the `seed_failures_total` metric. |
| TTL reaps a challenge session | Nothing special: all challenge state lives inside the vcluster, which dies with the claim. Annotations die with it too. |

## 10. Slow-path (Helm-dependency) scenarios — excluded from v1

Bundles needing an in-vcluster Helm install (Prometheus/Grafana, Headlamp,
OpenEBS — the latter already out of scope entirely) break the 1–2 s budget by
an order of magnitude. V1 simply has no such bundles; the bundle schema
reserves a `heavy: true` flag so the catalog can later mark them with an
explicit "takes ~1 min to prepare" affordance and a longer seeder budget,
without any architectural change — the seeder state machine and SSE phase
already accommodate a longer `Seeding`. Decision on whether to build that
content is deferred, consistent with the decision memo's "heavier tier"
carve-out.

## 11. Phase 2 — progress persistence (designed, not built)

V1 grade results are ephemeral. When completion history is wanted:

- **Store:** one ConfigMap per user, `sbxprogress-{sha256(owner)[:16]}` in the
  backend namespace (same naming/hygiene as the owner markers; the backend
  Role already has ConfigMap verbs). Data: `{challengeId: {firstPassedAt,
  attempts}}` as JSON. Size is trivially bounded by catalog size.
- **Write path:** on a grade where `pass: true` and no prior record — a single
  idempotent apply, off the grade hot path.
- **Read path:** `GET /api/challenges` merges completion badges;
  `GET /api/progress` for the profile view.
- **Migration:** the shape is deliberately a K/V document so a later move to a
  real DB (if multi-replica ever demands one) is a dump-and-load.

Explicitly *not* in this design: leaderboards, timed exam mode, cohort/
workshop grouping — all plausible on top of the same grade event, none
constraining the architecture now.

## 12. Observability

New metrics alongside the existing pool set: `seed_duration_seconds`
(histogram, by challenge), `seed_attempts_total` (by result:
success/retry/recycled/failed), `grade_requests_total` (by challenge, pass),
`tenant_client_errors_total`, `content_bundle_invalid` (gauge, by bundle —
quarantined content). Alert candidates: any `seed-state: failed`, any
quarantined bundle, seed p95 > 5 s (tenant API degradation), grade error rate.

## 13. Work items (implementation order)

1. **Composition:** per-session Role/RoleBinding for the kubeconfig Secret
   (§5.2); comment guarding the netpol assumption (§5.1). Ship first — inert
   without backend changes.
2. **Content pipeline:** bundle schema types + validator (shared Go package),
   the validator CLI wired into CI, the `kubesandbox-challenges` content
   chart, the backend Role addition (`list`/`watch` ConfigMaps — `get` verbs
   exist for markers), and the `ContentStore` informer implementation; 2–3
   real bundles from the catalog (one RBAC, one NetworkPolicy, one
   troubleshooting) as the vertical slice.
3. **`internal/kubernetes/tenant.go`:** Secret → client factory + cache; an
   integration test against a live session on prod-k3s.
4. **`internal/challenges`:** seeder (state machine, SSA apply, retry/recycle)
   + grader (check evaluators). Unit-test the state machine with the same
   rv-enforcing fake used by the pool tests; table-test every check type.
5. **API:** catalog/grade/reset handlers, `Assign()` stamping, model changes,
   router, metrics.
6. **Frontend:** catalog page, challenge session view (instructions + terminal
   + check panel), `Seeding` phase, reset button.
7. **Live verification** (mirrors the hot-pool checklist): create-with-challenge
   → seeded < 5 s; grade fail → fix in terminal → grade pass; reset → re-grade
   fail; kill backend mid-seed → converges; queue path with challengeId; 409/404
   surfaces.

## 14. Open questions (deliberately deferred)

- **Hint UX/economy** — free hints vs. revealing them affecting a (phase-2)
  score. Content question, not architectural.
- **`probe` check type** (exec-based validation) — needed for a minority of
  catalog items (file-copy, signal-sending challenges); requires a scoped
  checker pod design and a security pass.

## 15. Compatibility with the planned Redis-queue / multi-replica migration

A Redis-backed assign queue (to horizontally scale the backend) is planned but
separate work. This design was checked against it component by component —
**nothing here conflicts, and nothing needs redesigning later**, because every
challenge component follows the same rule that already makes the rest of the
backend replica-safe: durable state lives in the API server, workers are
level-triggered, and writes are idempotent or CAS-guarded.

| Component | Multi-replica behavior |
|---|---|
| Content store (ConfigMap informer) | Each replica watches independently and builds its own catalog. Momentary skew between replicas during a sync is harmless. |
| Seeder | State on the claim, CAS transitions (§6.1), SSA idempotent — replicas racing resolve at the API server. Optionally gate it behind the same leader election the pool manager will need anyway. |
| Grader | Stateless, request-scoped tenant reads. The per-session rate limit is per-replica in-memory (degrades to N× the limit — acceptable; move to Redis with the queue if desired). |
| Tenant-client cache | Per-replica LRU; duplication is just memory. |
| Session SSE | Already a per-request claim `Watch` — replica-safe today. |
| Queue path | `challengeId` rides inside `CreateSessionRequest`, which is exactly the struct the Redis queue will serialize — the challenge flow inherits durability for free. **The one contract:** whatever encoding the Redis migration picks must round-trip the full request struct, not just the owner. |
| Progress ConfigMap (phase 2) | Idempotent apply; concurrent first-pass writes converge. |

**Ordering recommendation: challenges first, Redis after.** Three reasons.
First, the scaling bottleneck today is the *control plane* (low-teens
vclusters total, §1) — a single Go backend replica saturates long after the
cluster does, so horizontal backend scaling has no load to absorb yet; its
real payoff is HA, which the crash-safe reconcile loops already substantially
mitigate. Second, Redis alone doesn't unlock multi-replica anyway — the pool
manager (and now the seeder) also need leader election, making it a larger
project than "swap the queue." Third, challenges are the product value this
whole track exists for, and building them first costs nothing later: the CAS
transitions and request-struct contract above are the entire interface
between the two efforts, and both are baked into v1.
