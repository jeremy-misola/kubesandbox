# KubeSandbox — Fast Provisioning Approach

**Status:** accepted and **implemented** — the recommended hot warm-pool shipped 2026-07-02. Historical rationale, kept for context.
**Audience:** Jeremy (platform owner)
**Related:** [`hot-pool-design.md`](../reference/hot-pool-design.md) · [`pause-resume-spike.md`](./pause-resume-spike.md) · [`backend-architecture.md`](../reference/backend-architecture.md)

> **Update (2026-07-02).** The recommendation in §6 (a **hot** warm pool with
> assignment, single sandbox type, queue on empty) was implemented. See
> [`hot-pool-design.md`](../reference/hot-pool-design.md) for the as-built design and
> live measurements. This document is retained as the rationale and options
> assessment that led there.

---

> **TL;DR:** Cold provisioning today takes upwards of 10 minutes; the target is a
> sandbox in the user's hands within **10 seconds**. That target rules out
> optimizing the current create-on-demand flow — even aggressively tuned, booting a
> virtual cluster from scratch is a minutes-scale operation. The only approach that
> fits a 10-second budget is to **have the sandbox already running before the user
> asks for it** (a warm pool), and to reduce "provisioning" to "assignment." This
> document explains the reasoning, then assesses the warm-pool approach against the
> alternatives. Sign-off (§2.2) fixed the budget as **hard (10 s target, 15 s
> ceiling)**, collapsed the platform to a **single sandbox type**, set the
> empty-pool fallback to a **queue**, and — with a peak arrival of only ~5/hour —
> all but eliminated the pool-sizing problem. A pause/resume spike
> ([`pause-resume-spike.md`](./pause-resume-spike.md)) then **rejected the
> "paused" warm-pool variant** (§5.6): resuming a scaled-to-zero vcluster took ~73 s
> (floor ~25 s), far outside 15 s. **The pool must therefore be *hot* (pods kept
> running), where assignment is a metadata change and effectively instant.** At this
> scale the idle cost of a hot pool is trivial. The remaining risk is the
> correctness mechanics (§4), not latency or capacity.

---

## 1. The problem

Today a sandbox is built entirely on demand: when a user requests one, the platform
creates a fresh namespace, provisions a virtual cluster, waits for it to become
healthy, starts a terminal workload, and wires up routing. End to end this takes
upwards of ten minutes. For the intended uses — learning, live workshops, demos —
that wait is not viable. A learner loses momentum, and a workshop of thirty people
starting at once turns a ten-minute wait into a stalled room.

The goal for this work is a hard one: **a usable sandbox within roughly ten
seconds of the request.**

## 2. Why the target reshapes the solution

It is worth being explicit about why ten seconds is a different kind of goal than
"make it faster." The bulk of the current wait is not wasted effort that can be
tuned away — it is the genuine cost of bringing a virtual cluster to life:
scheduling and pulling images, booting a control plane, generating credentials, and
confirming the whole thing is healthy before handing it over. These steps have a
floor measured in minutes, not seconds.

This leads to the central insight: **you cannot make the cold path ten seconds
long, so the user must not be on the cold path at all.** The slow work still has to
happen — but it has to happen *before* the user shows up, not while they wait. That
reframes the problem from "provision faster" to "pre-provision, then assign."

### 2.1 Two caveats before the target drives the design

**The target is confirmed hard (resolved).** Sign-off fixed the budget at
**10 seconds, 15 seconds maximum** (§2.2) — not the "under a minute"
[`backend-architecture.md`](../reference/backend-architecture.md) assumed. This rules
out the relaxed path where cold-path optimization alone would suffice: at a
15-second ceiling the user cannot be on a cold build, so pre-provisioning is
required. The number now legitimately drives §5.

**The cold-path number is unexplained.** This document cites 10+ minutes end to
end; the architecture doc cites vcluster cold-boot at ~4–5 minutes and calls the
rest "fast." The missing 5+ minutes are unaccounted for — likely Crossplane
reconcile/poll intervals, Helm release convergence, and image pulls, all of which
are cheap to fix and none of which require this proposal. **A measured latency
breakdown of the cold path is a prerequisite for the design phase**: it determines
refill speed, and refill speed determines pool size.

### 2.2 Fixed parameters (sign-off, 2026-07-02)

The open questions in §7 have been answered; these values are now inputs to the
design, not variables:

- **Latency budget:** 10 seconds target, **15 seconds hard ceiling**.
- **Sandbox types:** **one**. The `starter` / `standard` / `advanced` profiles are
  removed for now — a single, uniform sandbox. This makes warm sandboxes *globally*
  fungible (no per-profile pools) and removes a whole class of sizing and
  segmentation work.
- **Peak arrival rate:** ~5 sessions/hour (roughly one every 12 minutes). This is
  the single most consequential answer: at this rate the pool almost never drains,
  so pool sizing stops being the hard problem and workshop-style bursting is *not* a
  first-class case.
- **Concurrent-session ceiling:** ~60 sandboxes on the cluster — a capacity limit
  independent of latency (§4).
- **Empty-pool fallback:** **queue** the user (with progress), not a cold build on
  the request path.
- **Idle-capacity budget:** up to **10** warm sandboxes. Combined with the 5/hour
  arrival this is generous — a pool far smaller than 10 already keeps drain
  probability negligible. It also means a **hot** pool (pods kept running) is
  affordable, which the spike (§5.6) showed is required since a paused pool cannot
  resume within budget.

**Implication.** With a single sandbox type and a ~5/hour peak, the classic
warm-pool risks — per-profile segmentation, burst sizing, idle cost — largely
evaporate. And with assignment against an already-running (hot) sandbox being a
sub-second metadata change, the **latency budget is no longer the hard part
either**. What remains genuinely hard is the correctness mechanics in §4:
redesigning the one-per-user guarantee, mutating a live claim at hand-over, and
starting the TTL clock at assignment.

## 3. The methodology: pre-provision and assign

The approach separates two things that are currently fused together:

- **Provisioning** — the slow, resource-heavy work of building a sandbox. This runs
  ahead of demand, in the background, and its latency stops mattering to the user.
- **Assignment** — the moment a user is given a sandbox. This is the only step on
  the user's critical path, and it must fit inside the ten-second budget.

Concretely, the platform keeps a small **pool of ready, unclaimed sandboxes** warm
at all times. When a user requests one, the system does not build anything — it
**hands over a sandbox that is already running**, marks it as belonging to that
user, and starts its lifecycle clock. A background process then quietly builds a
replacement to keep the pool topped up.

Because a KubeSandbox sandbox contains nothing user-specific until it is claimed —
it is a generic terminal attached to a generic virtual cluster — any warm sandbox
can be given to any user. This is what makes assignment cheap: there is no
customization to perform at hand-over time, only a change of ownership.

With a single sandbox type (§2.2), fungibility is **global**: every warm sandbox is
identical and interchangeable, so any one can be handed to any user with no
per-profile pools to maintain. (Were profiles reintroduced later, fungibility would
become per-profile and the pool would need segmenting — noted here so the
simplification is a conscious, reversible choice.)

The methodology rests on three ideas:

1. **Decouple build time from wait time.** Move all slow work off the request path
   by doing it in advance.
2. **Keep a buffer sized to demand.** Maintain enough warm sandboxes to absorb the
   expected rate of arrivals, and refill continuously as they are claimed.
3. **Make hand-over trivial.** Because sandboxes are interchangeable until claimed,
   assignment is a lightweight ownership change rather than a construction job.

## 4. What this approach must get right

Choosing the warm-pool approach commits us to solving a specific set of problems.
Naming them now is part of the assessment — these are the real work, and they are
what the alternatives avoid (at the cost of not meeting the target).

- **Pool sizing and refill.** Too small and users hit the queue; too large and
  idle sandboxes waste the budget. At the fixed ~5/hour arrival (§2.2) this is now
  easy — even a pool of two or three makes draining unlikely, well under the budget
  of ten — *provided* refill (a background rebuild after each hand-over) keeps up.
  Refill speed is gated on the cold-path number we still need to measure (§2.1).
- **The empty-pool fallback (decided: queue).** When demand outruns the pool the
  user is queued with progress feedback rather than dropped onto a cold build on the
  request path. At ~5 arrivals/hour this path should be rare, but it must exist and
  be honest about the wait.
- **One-per-user enforcement breaks and must be redesigned.** Today the guarantee
  is atomic and race-free because the claim name is derived from the owner
  (`s-{sha256(owner)[:16]}` — a second create fails `AlreadyExists` at the API
  server; see architecture §10). Warm claims are created *before* the owner
  exists, so names cannot be owner-derived and that mechanism is lost. A
  replacement is needed — e.g. an optimistic-concurrency ownership patch on the
  claim plus a per-owner marker object — and two simultaneous requests must never
  be handed the same sandbox.
- **Assignment mutates a live claim.** Hand-over means setting `ownerRef` /
  `tenantRef` on an existing `KubeSandboxSession`. The XRD must not treat these
  as immutable, the Composition re-patches owner annotations onto the namespace
  on reconcile, and mutation must not churn the other composed resources. The
  `/authz` ownership check reads the claim directly, so it sees the new owner
  immediately — but this needs verifying, not assuming.
- **The TTL clock conflicts with warmth.** The TTL loop falls back to
  `creationTimestamp + ttlMinutes` because the Composition never populates
  `status.expiresAt`. Unfixed, a sandbox that sat warm for 50 minutes gives its
  user a 10-minute session — or is reaped before it is ever claimed. The
  lifecycle clock must start at *assignment*, which makes populating `expiresAt`
  (already a known gap) a prerequisite.
- **Freshness.** A sandbox that has sat warm for a long time should be recycled
  rather than handed over, so users never receive a stale or degraded environment.
- **Idle cost.** Warm capacity is capacity you pay for whether or not it is used.
  The spike (§5.6) ruled out lowering this by *pausing* the pool (resume is too
  slow), so the pool runs hot. At the sanctioned budget of ≤10 against ~5/hour
  arrival, keeping ~2–3 vclusters hot is trivial headroom; the lever that remains is
  scaling the warm count to the time of day if desired.
- **Capacity is not solved by warmth.** The pool addresses latency only, not how
  many sandboxes the cluster can run at once — a ceiling now fixed at ~60 (§2.2).
  The warm pool (≤10) sits inside that ceiling, so warm capacity and peak live
  capacity must be budgeted together against the same 60.

## 5. Options considered

The alternatives below were weighed against the ten-second target. The short
version is that only the warm-pool approach can meet it; the others are either
insufficient on their own or address a different problem.

### 5.1 Optimize the existing on-demand flow

Tune the current build path — faster reconcile loops, more generous startup
resources, pre-cached images. **Assessment:** worthwhile and probably cuts the wait
substantially (minutes, not seconds), but it cannot reach ten seconds because it is
still building a virtual cluster on the user's request. Best treated as a
*complement* to the warm pool: faster builds mean the pool refills quicker and can
therefore be smaller and more resilient to bursts. Necessary, not sufficient.

### 5.2 Warm pool (pre-provision and assign) — recommended

Keep sandboxes ready in advance; assignment is near-instant. **Assessment:** the
only approach that meets the target. It fits the architecture well because
sandboxes are user-agnostic until claimed, so hand-over is cheap. The cost is added
operational machinery (a pool manager), idle spend, and the correctness concerns in
§4. These are real but bounded, and they are the price of the target.

### 5.3 Tiered isolation (a lighter sandbox for simple cases) — dropped

Offer a lighter-weight environment for basic use and reserve the full virtual
cluster for those who need it. **Assessment:** out of scope. Sign-off collapsed the
platform to a single sandbox type (§2.2), so there are no tiers to differentiate.
Recorded here only so the option can be revisited if profiles ever return.

### 5.4 Re-architect the build engine

Replace the current declarative, poll-based provisioning with a purpose-built,
event-driven one that reacts immediately. **Assessment:** this attacks real latency
in the cold path and would meaningfully speed up builds and pool refill, but it is
the largest undertaking here and still does not make a from-scratch build
ten seconds long. It belongs in the same bucket as §5.1: a way to make the cold
path (and therefore refill) faster, not a way to avoid it.

### 5.5 Improve perceived latency only

Leave provisioning as-is but make the wait feel better with richer progress
feedback. **Assessment:** cheap and worth doing regardless, but it does not change
the actual time and therefore does not meet the target. Useful as polish and as the
honest fallback when the pool is empty (§4), not as a solution.

### 5.6 Paused warm pool (vcluster sleep/resume) — rejected by spike

Keep the pool warm but **asleep**: vcluster workloads scaled to zero, resumed on
demand, trading a little resume latency for near-zero idle CPU/memory. This was the
most attractive variant on paper because it attacks the idle-cost trade-off in §5.2.
**Assessment: rejected.** The spike
([`pause-resume-spike.md`](./pause-resume-spike.md)) measured resume
(scale-to-zero → ready) at **~73 s**, with a floor around **25 s** even fully tuned,
against a **15 s** ceiling. The cause is structural: a scale-up recreates the pod
from scratch each time — re-running the binary-copy init container and re-booting the
control-plane API — and persistence does not avoid either. So pausing cannot meet
the budget. Two useful side-findings carried forward: (1) the production **200m CPU
limit on the control plane inflates cold boot ~6×** (196 s → 33 s of control-plane
boot when raised) and should be fixed regardless; (2) the vcluster **Helm install
itself is fast (~10 s)**, so the 10-minute production total lives in the layers
around vcluster — reinforcing the cold-path measurement gate (§2.1).

### 5.7 Scheduled pre-provisioning for workshops — not needed for now

Workshops are the hardest sizing case for a demand-driven pool — but they are
**announced in advance**, so one could bulk pre-create sandboxes on a calendar
trigger. **Assessment:** not warranted at the stated ~5/hour peak (§2.2); there is
no burst to plan for. Kept on record as the natural answer *if* real workshop bursts
(dozens starting at once) ever become a use case — at which point the concurrent
ceiling of 60, not the pool, becomes the binding constraint.

## 6. Recommendation

Adopt the **pre-provision-and-assign direction (§5.2) with a *hot* pool** as the
primary strategy. With sign-off (§2.2) and the spike (§5.6) the picture is now
settled — one sandbox type, a tiny pool of ~2–3 hot vclusters against ~5/hour
arrival, a queue when it drains — so the remaining uncertainty is the correctness
mechanics (§4), not latency, sizing, or pool posture. One gate remains before the
design is written:

1. **Measure the cold path** (§2.1). Produce a latency breakdown explaining the gap
   between the ~77 s tuned vcluster boot (spike) and the 10+ min production total.
   This informs refill speed — the one sizing input that still matters — and the
   spike already points at the likely culprits (Crossplane reconcile/poll and the
   serialized namespace → vcluster → secret → shell-pod chain).

Supporting moves:

- **Raise the control-plane CPU limit** (from 200m). The spike showed this alone
  cuts cold boot ~3× and directly speeds refill; do it regardless of the pool work.
- Use **cold-path optimization (§5.1)** and, if justified, a **faster build engine
  (§5.4)** to make pool refill quick.
- Use **better progress feedback (§5.5)** for the queued (empty-pool) case.
- Tiered isolation (§5.3), a paused pool (§5.6), and scheduled pre-provisioning
  (§5.7) are **out of scope** under the current parameters, recorded only for future
  reference.

The next step, once the cold-path gate is measured, is a design document covering:
the redesigned one-per-user guarantee, claim-mutation semantics at assignment, the
`expiresAt`/TTL-starts-at-assignment fix, the queue mechanism, hot-pool management
(warm count, refill, freshness/recycling), and the control-plane CPU fix — the items
in §4.

## 7. Sign-off status

Resolved (folded into §2.2 / §5.6):

- **Latency budget** — 10 s target, 15 s ceiling. *Hard requirement.*
- **Arrival rate** — ~5/hour peak; workshop bursting not a first-class case.
- **Concurrent ceiling** — ~60 sandboxes.
- **Profiles / sandbox types** — collapsed to one.
- **Empty-pool fallback** — queue.
- **Idle budget** — up to 10 warm sandboxes.
- **vcluster pause→resume latency** — measured ~73 s (floor ~25 s); **paused pool
  rejected, pool runs hot** ([`pause-resume-spike.md`](./pause-resume-spike.md)).

Still open — the one remaining design gate (§6):

- **Where does the cold-path time go?** The spike showed vcluster boot is ~77 s
  tuned and the Helm install ~10 s, so the 10+ min production total is in the layers
  around vcluster. A measured breakdown is still a prerequisite because it sets
  refill speed (§2.1).
