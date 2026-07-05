# Guided Challenges Backend — Implementation Handoff

**Date:** 2026-07-05 (late evening session, 2026-07-04 → 05)
**Scope:** full backend implementation of
[`docs/history/challenges-backend-architecture.md`](docs/history/challenges-backend-architecture.md)
(§13 items 0–5 + tests; frontend item 6 intentionally out of scope).
**Status:** implemented, unit/race-tested, deployed once to prod-k3s, and
live-verified through most of the §13.7 checklist. **Three fixes made after
the first deploy exist only in the working tree and need a second push** (see
"Needs a second deploy" below).

---

## 1. Where things stand

### Built and merged into the working tree

| Design item | Where | State |
|---|---|---|
| §13.0 de-risk spike | throwaway (deleted) | ✅ Verified live: a pod in the backend namespace used the composed `vc-*` Secret's kubeconfig **as-is** against the in-cluster Service DNS and listed tenant pods. §5.1 assumptions hold. |
| §13.1 composition | `kubesandbox-session-composition.yaml` | ✅ Per-session `Role` (get secrets, `resourceNames: [vc-{ns}-{name}-vcluster]`) + `RoleBinding` to the backend SA, composed into every session namespace; §5.1 comment added at the shell NetworkPolicy. Live: new members carry both. |
| §13.2 content pipeline | `backend/internal/content/`, `backend/cmd/validate-bundles/`, `kubesandbox-charts/kubesandbox/charts/kubesandbox-challenges/`, `.github/workflows/challenges.yml` | ✅ Bundle schema (`content.kubesandbox.com/v1`), shared validator, ConfigMap-watching `ContentStore` (level-triggered watch+rebuild, quarantine on invalid — never a crash), validator CLI wired into CI, 3 real bundles (RBAC / NetworkPolicy / troubleshooting). Live: `content: catalog rebuilt (3 bundles, 0 quarantined)`. |
| §13.3 tenant client | `backend/internal/kubernetes/tenant.go` | ✅ Secret → REST config → dynamic client, per-session LRU (cap 32), invalidated on **every** claim-delete path via the `SetClaimDeletedHook`. Impersonation support for `subjectCan`. |
| §13.4 seeder + grader | `backend/internal/challenges/` | ✅ Seeder: channel-fed + startup/resync reconcile, CAS state machine (`pending → seeding → seeded \| failed`), SSA `fieldManager: kubesandbox-seeder, force: true`, retry(3) → recycle-once → fail-closed. Grader: all 8 check types, all steps evaluated (no short-circuit), read-only by construction. |
| §13.5 API | `handlers/challenges.go`, `sessions.go`, `router.go`, `models`, `config.go`, `telemetry`, charts | ✅ `GET /api/challenges[/{id}]` (hints by count, text via `?hints=n`), `challengeId` on create (400 unknown, validated **before** any assignment work), `challenge` block + synthetic `Seeding` phase, grade (409/404/429), reset (202, durable CAS intent), §12 metrics, `CHALLENGES_*` env + Helm `challenges.*` values. |

### Invariants — all held

`pool.go` / `warm.go` / `queue.go` / `/authz` untouched; no XRD change
(`spec.starterLabRef` reused, state in annotations only); `Assign()` stamps
challenge intent in the **same CAS Update** that claims the member and never
blocks on seeding; no cluster-wide Secret RBAC; every seed-state transition is
rv-guarded (including, after a fix, the failure ladder); grading on-demand
only; content quarantined, never fatal.

### Local test bar

`go test ./...`, `go test -race`, `go vet`, `gofmt` all clean. Coverage:
seeder happy/retry/crash-resume/recycle/fail-closed(3 variants)/reset/
concurrent-convergence against a resourceVersion-enforcing fake (same reactor
pattern as `pool_test.go` — the stock fake ignores rv); every check type
table-tested; content quarantine; tenant LRU (cache/invalidate/evict); atomic
assign stamping; handler status contract (400/404/409/429/200/202); bundle
CLI green on the 3 shipped bundles.

---

## 2. Live verification (§13.7) — scoreboard

Run against prod-k3s after Jeremy's push, driving the API from a curl pod in
`envoy-gateway-system` (the backend NetworkPolicy only admits that namespace)
with `X-User-Id` headers as synthetic users.

| Check | Result |
|---|---|
| Create with `challengeId` → seeded | ✅ 201 at 02:42:24 with `phase: Seeding`, `seeder: … seeded` at 02:42:28 → **~4 s**, inside the 5 s target. |
| Grade fail → fix in terminal → grade pass | ✅ Initial grade: all 3 steps fail with actionable messages (SSAR denied / RoleBinding not found / `availableReplicas 0, want >= 1`). Created the RoleBinding via the session's own shell pod; two steps flipped immediately, third after the probe recovered → `pass: true`. (One content bug found on the way — see §3.2.) |
| Reset → re-grade fail | ✅ 202 `reseeding` → labeled namespace deleted, waited, re-seeded (51 s end-to-end; namespace termination dominates) → re-grade: all steps fail again. |
| 409 / 404 / 429 surfaces | ✅ 409 grade+reset mid-seed (`pending` and `seeding` both observed), 404 no-challenge session, 404 non-owner (no existence leak), 404 unknown catalog id, 400 unknown id on create, 429 under the 2 s grade interval. |
| Kill backend mid-seed → converges | ✅ Killed the pod mid-reset (widest window). On restart the reconcile resumed the claim; re-applies hit `namespace … is being terminated`, the ladder exhausted retries, **recycled the member, re-assigned, seeded the replacement ~2 s later**. User lands on a healthy seeded session either way. Exposed two windows now fixed (§3.3, §3.4). |
| Queue path with `challengeId` | ✅ Pool drained → user-3 queued #1 (plain), user-4 queued #2 with `challengeId` (202s). FIFO held: user-3 admitted first as refill completed, then user-4's session arrived `Ready` **already seeded** with the ConfigMap-key challenge (`challenge.seedState: seeded`, no client action) — the id rode the queued `CreateSessionRequest` exactly as designed, and a grade against it returned the expected all-fail broken state. |

---

## 3. Bugs found and fixed along the way

### 3.1 Escalation could delete a healthy claim from a stale view *(caught by `-race` test, fixed BEFORE the deploy — in prod)*
Claim deletion isn't rv-guarded, so a seeder whose view aged (a racing worker
had just CAS'd the claim to `seeded`) could recycle/fail-close a healthy
session. Fix: every destructive step in the failure ladder now writes an
rv-guarded **fence** first (`seed-recycles` stamp for recycle, the `failed`
marker for fail-closed) and backs off to re-read on conflict.
`TestSeederConcurrentProcessSingleApplyWins` locks it in.

### 3.2 `--request-timeout` silently disables kubectl's in-cluster config *(content bug, found live — fix pending push)*
The RBAC bundle's readiness probe used `kubectl get pods --request-timeout=5s`.
**Any command-line override makes kubectl skip its in-cluster-config fallback**
and dial `localhost:8080`, so the probe could never succeed even after the
user's correct fix. Verified interactively (same pod: with flag → localhost
fallback; without → `Using in-cluster configuration`). Fixed in
`challenges/troubleshoot-…/seed/30-deployment.yaml` (probe now relies on its
own `timeoutSeconds`) with a warning comment. For the live session I patched
the tenant deployment manually so the grade-pass leg could be verified.

### 3.3 Pool marker GC races the recycle window *(observed live during the crash test — fix pending push)*
`pool: removing orphaned owner marker` fired in the ~1 s gap between the
recycle's claim-delete and re-assign: for that instant the owner holds no
claim and the marker is past the 2-minute grace, so the (untouchable-by-design)
pool GC reaped it. One-per-user survived via Assign's legacy-holder guard, but
a concurrent create by the same user in that window could double-claim. Fix in
`ReassignForRecycle` (no `pool.go` changes): abort with `ErrAlreadyExists` if
the owner acquired a claim meanwhile (their newer session wins; seeder stands
down without releasing anything), and re-create the marker if it was reaped.
A truly simultaneous List/CAS interleaving remains theoretically possible on
one replica for one TTL — documented in the code, negligible in practice;
closing it fully would need GC awareness of recycle intent inside `pool.go`.

### 3.4 Reset flag cleared too early → crash-resume re-applies into a terminating namespace *(observed live — fix pending push)*
The `seed-reset` flag was cleared in the `pending→seeding` CAS, before the
delete-and-wait finished. A crash in that window resumed as a plain apply
against the still-terminating namespace (`unable to create new content…`),
burning the retry ladder + a recycle to converge. Fix: the flag now survives
until the terminal `seeded` CAS, so a resumed reset re-runs the idempotent
delete-and-wait first. `TestSeederCrashMidResetResumesWithDelete` covers it.

---

## 4. Needs a second deploy (working tree > prod)

The first push included everything through §3.1. Still local-only:

1. `kubesandbox-charts/.../troubleshoot-rbac.../seed/30-deployment.yaml` — probe fix (§3.2). Chart-only; ArgoCD sync updates the ConfigMap, next seed/reset picks it up. **Without it the RBAC challenge is uncompletable** (deployment-recovers step can never pass).
2. `backend/internal/kubernetes/challenge.go` — `ReassignForRecycle` guards (§3.3). Needs image rebuild.
3. `backend/internal/challenges/seeder.go` — reset-flag durability + stand-down on `ErrAlreadyExists` (§3.4). Needs image rebuild.
4. New tests (`seeder_test.go` additions) + this file.

`go test ./...`, `-race`, vet, fmt re-verified clean after all of the above.
Suggested flow: push → CI image build → `kubectl rollout restart` the backend
(image tag is `latest`) → re-run the RBAC challenge end-to-end once to confirm
the probe fix (create → grade fail → bind → grade pass, no manual patching).

---

## 5. Test debris on prod-k3s — CLEANED UP

All four verify-user sessions were deleted through the API (204s, markers
released), and the `challenge-verify` curl pod in `envoy-gateway-system` was
removed. The pool manager is refilling to target on its own. Nothing from the
verification remains; Jeremy's own session was never touched.

---

## 6. Operational notes for whoever picks this up

- **Config:** `CHALLENGES_ENABLED` + `CHALLENGE_*` env (chart `challenges.*`).
  All defaults match the design; nothing needs tuning.
- **Metrics (§12):** `kubesandbox.challenge.seed.duration` (by challenge),
  `challenge.seed.attempts` (success|retry|recycled|failed — alert on any
  `failed`), `challenge.grade.requests` (by challenge, pass),
  `challenge.tenant_client.errors`, `challenge.content.bundle_invalid` (gauge
  by bundle — alert on any). Pool dashboard JSON not yet extended.
- **Adding a challenge** is a git push: directory under
  `kubesandbox-charts/kubesandbox/charts/kubesandbox-challenges/challenges/`,
  CI lints it (`challenges.yml` workflow runs `cmd/validate-bundles`), ArgoCD
  syncs the ConfigMap, the backend catalog picks it up within seconds — no
  backend rebuild. Authoring constraints the validator enforces: kinds must be
  in the known-kinds table (`content/bundle.go`), everything namespaced (or a
  Namespace), **requests+limits on every pod** (host ResourceQuota rejects
  otherwise), PSA-restricted-compliant securityContexts, no `--request-timeout`
  in kubectl probes (§3.2).
- **Known deliberate scope cuts** (per design): no hint economy, no
  exec/probe checks, no progress persistence, no heavy/Helm bundles, no
  frontend, no Redis/leader election. Reset removes *seeded* state only —
  user-created unlabeled objects persist (§7 caveat).
- **Reset duration:** ~50 s observed, dominated by tenant namespace
  termination — fine vs. the "seconds, no new sandbox" goal, but the frontend
  should show the Seeding phase, not a spinner-less wait.
- **CHANGELOG:** rev 27 entry written (in the deployed push). It predates
  §3.2–3.4; this file is the accurate post-verification record.
