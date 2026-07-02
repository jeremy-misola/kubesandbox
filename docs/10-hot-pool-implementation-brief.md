# KubeSandbox — Hot Warm-Pool Implementation Brief (Agent Handoff)

**Status:** implementation brief (2026-07-02) — hand this to the implementing agent
**Audience:** the next agent + Jeremy
**Related:** [`08-provisioning-latency-approach.md`](./08-provisioning-latency-approach.md) · [`09-pause-resume-spike.md`](./09-pause-resume-spike.md) · [`01-backend-architecture.md`](./01-backend-architecture.md) · [`04-backend-handoff.md`](./04-backend-handoff.md)

---

## How to use this document

Everything below the line is a self-contained prompt for the implementing agent.
Copy it verbatim (or point the agent at this file). It assumes a cold start with no
memory of the prior investigation, so it restates the decisions and cites where to
verify them.

---

## PROMPT — begin

You are implementing a **hot warm-pool** for KubeSandbox so that a user gets a
usable sandbox within **10 seconds (15 s hard ceiling)** instead of the current
10+ minutes. The approach, alternatives, and the spike that settled the design are
already decided and documented — your job is to **read the decision docs, then
implement**, not to re-litigate the strategy.

### 0. Orient yourself first (do this before writing code)

Read these in order and treat them as the source of truth; if your plan conflicts
with them, stop and raise it:

1. `docs/08-provisioning-latency-approach.md` — the strategy, fixed parameters (§2.2),
   what must be gotten right (§4), and the recommendation (§6).
2. `docs/09-pause-resume-spike.md` — why the pool is **hot, not paused**, plus the
   CPU-limit finding.
3. `docs/01-backend-architecture.md` and `docs/04-backend-handoff.md` — how the
   backend, CRD, composition, TTL sweep, and auth currently work.

Then read the code you will change (verify every claim below against the actual
files — do not trust this brief blindly, it may drift):

- `backend/internal/kubernetes/sessions.go` — `Create`, `List`, `Get`, `Delete`,
  `Authorize`, and the deterministic `sessionName`/`ownerHash` singleton mechanism.
- `backend/internal/kubernetes/cleanup.go` (+ `cleanup_test.go`) — the TTL sweep.
- `backend/internal/models/session.go` — CRD types, GVR, profiles, TTL constants.
- `backend/internal/api/handlers/sessions.go`, `authz.go`, `sse.go` — REST/SSE/authz.
- `kubesandbox-charts/kubesandbox-backend/templates/kubesandbox-session-composition.yaml`
  and `...-xrd.yaml` — the Crossplane composition and schema.

### 1. Fixed decisions (do NOT redesign these)

- **Hot pool.** Keep N pre-provisioned vclusters **running** (not paused/scaled-to-zero).
  The spike proved resume from zero is ~73 s — outside budget. Assignment must be a
  metadata change against an already-Ready sandbox.
- **Single sandbox type.** The `starter`/`standard`/`advanced` profiles are collapsed
  to one uniform sandbox. Do not build per-profile pools. (If profiles still exist in
  code, reduce to one; confirm the intended path with Jeremy before deleting types.)
- **Empty-pool fallback = queue.** If no warm sandbox is available, queue the user
  with progress feedback; do not put them on a cold build synchronously.
- **Parameters:** ~5 sessions/hour peak; ~60 concurrent-session cluster ceiling;
  idle budget up to 10 warm sandboxes (keep ~2–3 hot by default). Warm pool must
  stay within the 60 ceiling alongside live sessions.
- **Persistence stays disabled** on the vcluster (interchangeable, clean-slate pool
  members).

### 2. What to build

Implement in phases; each phase should compile, pass tests, and be independently
reviewable. Open a short design note or PR description per phase.

**Phase A — cheap wins, independent of the pool (do first):**
- Raise the vcluster control-plane CPU limit in the composition (currently `200m` in
  the vcluster `Release` values). The spike showed this cuts control-plane boot ~6×.
  Use a burstable shape (modest request, generous limit); confirm the exact value
  with Jeremy. This directly speeds pool refill.
- Populate `status.expiresAt` on the claim. Today the TTL sweep falls back to
  `creationTimestamp + ttlMinutes` because `expiresAt` is never set — this is a
  prerequisite for "TTL starts at assignment" (Phase C). Verify against `cleanup.go`.

**Phase B — pool manager (the core):**
- A controller (in the Go backend; prefer informers/watches over polling) that
  maintains a target count of **hot, unclaimed** sandboxes. It provisions
  replacements in the background whenever the available count drops below target.
- Warm sandboxes are `KubeSandboxSession` claims with **no owner** and a pool marker
  (e.g. a `kubesandbox.com/pool: available` label). They provision through the
  existing composition, including the vcluster and shell pod, and reach Ready before
  being considered "available."
- Naming: warm claims cannot use the current owner-derived name
  (`s-{sha256(owner)[:16]}`), because the owner is unknown at warm time. Give pool
  members generic names (e.g. `s-pool-<rand>`). See Phase D for how the one-per-user
  guarantee is preserved.
- Respect the concurrent ceiling: never let (warm + live) exceed the configured
  cap (~60). Make target warm count and cap configurable (Helm values).
- **Constraint learned in the spike:** vcluster refuses two vclusters in one
  namespace — keep one vcluster per namespace, as the composition already does.

**Phase C — assignment (hand-over):**
- New backend path: on a create request, **claim an available pool member** instead
  of building one. Set `ownerRef`/`tenantRef`, flip the pool marker to
  claimed/owned, and start the lifecycle clock by setting `status.expiresAt = now +
  ttl` (Phase A). Return the sandbox URL.
- This must be **atomic under concurrency**: two simultaneous requests must never get
  the same member. Use optimistic concurrency (update with a `resourceVersion`
  precondition; retry on conflict with the next free member).
- The claim is now **mutated after creation**. Verify the XRD does not treat
  `ownerRef`/`tenantRef` as immutable, that the composition re-patches the owner
  annotations onto the namespace on reconcile without churning other composed
  resources, and that `authz.go` reads the new owner immediately. Test this, don't
  assume it (the composition patches owner/tenant as namespace annotations today).

**Phase D — one-per-user guarantee (correctness-critical):**
- The current guarantee is atomic because the claim name = hash(owner). With generic
  pool names that mechanism is gone. Replace it — e.g. a **per-owner marker object**
  created atomically (name derived from owner, so a duplicate create fails
  `AlreadyExists` at the API server) that records which pool member the user holds.
  The marker create and the member claim must not be able to leave a user with two
  sandboxes or a member owned by two users. Write concurrency tests.

**Phase E — queue + freshness:**
- Empty-pool queue: hold the request with progress over the existing SSE channel;
  admit when a member becomes available (either a refill completes or one frees up).
- Freshness/recycling: a warm member older than a max-warm-age is recycled rather
  than handed out. Make max-warm-age configurable.

### 3. The one remaining open gate

`docs/08` §6 still lists **"measure the cold path"** as open: the spike showed
vcluster boot is ~77 s tuned and the Helm install ~10 s, so the 10+ min production
total lives in the layers around vcluster (suspected: Crossplane reconcile/poll
intervals and the serialized namespace → vcluster → kubeconfig secret → shell-pod
chain). Before finalizing pool sizing/refill assumptions, **produce a measured
per-stage breakdown of one cold provision** (timestamp each milestone). This sets
how fast the pool refills, which sets how small it can safely be.

### 4. Testing & verification (required)

- Unit tests for: atomic assignment under concurrent requests, one-per-user under
  concurrency, TTL-starts-at-assignment, and pool refill logic. Extend the existing
  `*_test.go` files; match their style.
- **Live verification against the cluster** (a Kubernetes MCP against `prod-k3s` is
  available). Do all cluster testing in **dedicated throwaway namespaces**, never in
  production session namespaces, and **tear down every resource you create**
  (releases, namespaces, PVCs, secrets). Measure real assignment latency end-to-end
  and confirm it is within the 15 s ceiling.
- Verify assignment→authz: after hand-over, the owner (200) and a non-owner (403)
  see the correct `/authz` results.

### 5. Guardrails

- Do not implement a paused/scale-to-zero pool, per-profile pools, or vcluster
  persistence — all explicitly rejected (`docs/08` §5.3/§5.6, `docs/09`).
- Do not touch production/live session resources during testing.
- Keep changes reviewable and phased; land Phase A independently.
- Where a value needs a human decision (CPU limit target, default warm count, cap,
  max-warm-age, TTL defaults), surface it to Jeremy rather than guessing.
- If reality contradicts this brief (code moved, composition differs, XRD blocks
  mutation), stop and report before working around it.

### 6. Definition of done

- A hot pool maintained by a background controller, within the concurrent ceiling.
- Assignment is atomic, sub-15 s (measured live), starts TTL at hand-over, and
  preserves one-sandbox-per-user under concurrency.
- Empty-pool requests queue gracefully; stale members are recycled.
- Control-plane CPU limit raised; `status.expiresAt` populated; TTL sweep verified.
- Tests (unit + live) pass; all spike/test resources cleaned up; a short design note
  per phase and an updated `docs/` entry describing the implemented design.

## PROMPT — end
