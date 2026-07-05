# Design: Redis-backed assign queue for horizontal scaling

Status: implemented (backend and Helm chart; see
`kubesandbox-charts/kubesandbox/charts/kubesandbox-backend` — `redis.*`,
`queue.*`, `leaderElection.*` values, in-chart Redis StatefulSet, lease RBAC,
and render-time guards for `replicaCount > 1`). Scope: replace
the per-replica in-memory `AssignQueue` and its in-process SSE fan-out with
Redis-backed shared state, and make the `PoolManager` reconcile loop safe
under N backend replicas.

Decisions resolved during review: `Queue` interface gains `ctx`+`error` on
every method (Redis failures surface as 503 `queue_unavailable`, never as
wrong answers); `queue.depth` is reported by every pod (dashboards must
aggregate with `max()`, not `sum()`); `QUEUE_MAX_WAIT_MINUTES=60` default;
single-instance Redis with AOF `everysec`.

---

## 1. Current state

Three pieces of process-local state pin the Deployment to one replica:

1. **Queue ordering** — `AssignQueue.items []*waiter` under a `sync.Mutex`
   (`internal/kubernetes/queue.go`). Enqueue dedups by owner, `Position` is a
   linear scan, `Head`/`Resolve` drain FIFO. Two replicas = two independent
   queues: FIFO ordering breaks and both admit against the same Ready members.
2. **SSE fan-out** — each `waiter` holds `subs map[chan QueueEvent]struct{}`.
   A subscriber on pod A never sees a `Resolve()` executed on pod B.
3. **Reconcile admission** — `PoolManager.reconcileOnce` step 1 loops
   `queue.Head()` → `svc.Assign` → `queue.Resolve()`, assuming it is the only
   drainer. N replicas each running `Run()` would race the same head owner
   against the same Ready claim. (`svc.Assign` is CAS'd against the claim and
   guarded by the owner marker, so double-*claim* is already impossible — but
   the loser gets `ErrAlreadyExists` and today's code would resolve the owner
   with a spurious "you already have a sandbox" **error** event.)

Not affected: the one-per-user invariant (owner marker + claim CAS in
`assign.go`, both in the K8s API), the request-path `Assign` (stateless, safe
on any pod), and steps 2–5 of reconcile (level-triggered against K8s state —
racy across replicas only in the benign create/trim-churn sense).

## 2. Target state

- **Queue state lives in Redis**, accessed through the existing method set,
  extracted into a `Queue` interface. The in-memory implementation stays for
  local dev and as the no-Redis fallback mode.
- **SSE events relay via Redis Pub/Sub**: one global channel; each pod runs one
  subscriber loop and fans out to its local SSE connections. Authoritative
  state (position, terminal outcome) is always re-derivable from Redis + K8s,
  so lost pub/sub messages heal on reconnect — same best-effort contract as
  today's `trySend`.
- **Exactly one replica runs the reconcile loop** via Kubernetes Lease-based
  leader election (`k8s.io/client-go/tools/leaderelection`). All replicas serve
  HTTP (enqueue, position, SSE, direct Assign); only the leader drains the
  queue and manages the pool.

### Component picture

```
            ┌────────── pod A (leader) ──────────┐   ┌──────── pod B ────────┐
 client ───▶│ HTTP: Create/Position/QueueEvents  │   │ HTTP: same            │
            │ PoolManager.Run  (holds Lease)     │   │ PoolManager: standby  │
            │ pub/sub relay loop                 │   │ pub/sub relay loop    │
            └───────┬────────────────┬───────────┘   └───────┬───────────────┘
                    │ ZADD/ZRANK/ZREM│PUBLISH               │SUBSCRIBE
                    ▼                ▼                       ▼
                 ┌──────────────── Redis ────────────────────┐
                 │ queue ZSET · req payloads · queue:ch      │
                 └───────────────────────────────────────────┘
                    K8s API: claims (CAS), owner markers, coordination Lease
```

## 3. Redis data model

All keys under a configurable prefix (default `ksbx:`).

| Key | Type | Content |
|---|---|---|
| `ksbx:queue:z` | ZSET | member = owner subject, score = monotonic sequence number |
| `ksbx:queue:seq` | STRING | `INCR` counter feeding ZSET scores |
| `ksbx:queue:req:{owner}` | STRING | JSON `{req: CreateSessionRequest, enqueuedAt: RFC3339Nano}` |
| `ksbx:queue:ch` | Pub/Sub channel | JSON `{owner, type, position?, session?, message?}` and `{type:"changed"}` |

Design notes:

- **ZSET, not List or Stream.** The semantics map one-to-one: dedup enqueue =
  `ZADD NX`, 1-based `Position` = `ZRANK + 1` (O(log N)), `Head` =
  `ZRANGE 0 0`, `Resolve` = `ZREM` (removal from the middle is O(log N), vs
  O(N) `LREM` on a List; a Stream can't delete-by-owner or answer rank at
  all). `queue.depth` = `ZCARD`.
- **Sequence counter, not wall clock, as score.** Two replicas with skewed
  clocks must not reorder FIFO. `INCR queue:seq` is globally monotonic.
- **Payload separate from ordering** so `Position`/`Head` never deserialize
  requests they don't need, and so the payload can carry `enqueuedAt` for the
  `queue.wait.duration` histogram.
- **Atomicity via Lua** (scripts embedded with `go:embed`, loaded with
  `EVALSHA`):
  - `enqueue.lua`: if `ZSCORE` exists → return `(rank+1, existing=1)`; else
    `INCR seq`, `ZADD`, `SET req`, return `(ZCARD, existing=0)`. The
    `existing` flag gates `RecordEnqueued` so the metric keeps today's
    "dedup is not a new enqueue" behavior.
  - `resolve.lua`: `ZREM` (returns 0 → someone else resolved; stop), `GETDEL
    req`, `PUBLISH` the terminal event, `PUBLISH {type:"changed"}`. Returns the
    payload so the caller can compute wait duration for `RecordResolved`.
- **Position rebroadcast is pull, not push.** After any resolve, subscribers
  need fresh positions (today's `broadcastPositionsLocked`). Rather than the
  resolver publishing every remaining owner's position (O(N) publishes plus a
  full `ZRANGE`), it publishes one `{type:"changed"}` event; each pod then runs
  `ZRANK` only for owners it has local SSE subscribers for and pushes
  `{type:"queued", position}` to them. Cheap, and positions come from the
  authoritative ZSET at read time.
- **Self-healing on payload/order divergence**: `Head` finding a ZSET entry
  with a missing `req` payload ZREMs it and advances (leaked half-writes can't
  wedge the queue); a leader janitor pass (piggybacked on reconcile) deletes
  `req` keys with no ZSET entry.

## 4. SSE relay: why Pub/Sub (vs Streams, vs keyspace notifications)

| | Redis Pub/Sub | Streams (+groups) | Keyspace notifications |
|---|---|---|---|
| Delivery | at-most-once, fire-and-forget | at-least-once, durable | at-most-once, no payload |
| Fan-out to N pods | native (every subscriber gets every message) | consumer groups do the *opposite* (each msg → one consumer); fan-out needs per-subscriber `XREAD` on per-owner streams | native but content-free |
| Reconnect/replay | none | `XREAD` from last-id replays | none |
| Ops overhead | none | per-owner stream keys need MAXLEN/TTL lifecycle | requires `notify-keyspace-events` server config |

**Choice: Pub/Sub**, because the events are *notifications about state that
lives elsewhere*, not the state itself — and the existing contract is already
best-effort (`trySend` silently drops on a full channel today). Replay is
unnecessary when reconnect re-derives everything authoritatively:

- Client drops mid-queue, reconnects to *any* pod → `Subscribe` reads
  `ZRANK` → still queued: emit current position, resume. Every future
  `changed` event refreshes it. Nothing missed matters; position is level
  state, not a log.
- Client reconnects after being resolved → not in ZSET → the existing
  `QueueEvents` fallback (`svc.List` → emit `assigned` with the session)
  already covers exactly this; an error resolve shows as not-queued +
  no session → 404, and the client re-POSTs (same as today).

Streams would add durable per-owner keys, trim policies, and last-id tracking
to buy replay that the resync-on-subscribe pattern makes redundant. Keyspace
notifications carry no payload and need server reconfiguration. Rejected.

**Subscribe race guard**: `RedisQueue.Subscribe(owner)` registers the local
subscriber in the pod's registry *first*, then checks `ZSCORE`. If the owner is
already gone, it unregisters and returns `queued=false`, and the handler's
existing owned-session fallback fires. This ordering means a resolve landing
between check and register can't be missed.

**Relay loop lifecycle**: one goroutine per pod, started at boot, `SUBSCRIBE
ksbx:queue:ch`, reconnect with backoff on error. On (re)connect it re-syncs
every locally-registered owner (ZRANK, or terminal fallback if gone) so a
pub/sub outage degrades to "positions frozen until reconnect", never to a hung
terminal state.

## 5. Concurrent reconcilers: Lease-based leader election

**Choice: leader election** (K8s `coordination.k8s.io/v1` Lease via client-go's
`leaderelection` package), not distributed locking in Redis and not
atomic-compare-and-pop with N concurrent drainers.

Why:

- The admission loop is not the only racy part of `reconcileOnce`: refill
  (step 4) and trim (step 3) also misbehave with N writers (each replica sees
  the same warm deficit and provisions → overshoot → trim churn, wasted
  sandbox creates). Leader election fixes the whole loop with one mechanism;
  an atomic queue-pop would fix only step 1.
- The lock's failure domain should not be the queue's. A Redis lock
  (SET NX PX / redlock) makes pool reconciliation stop when Redis is down —
  but steps 2–5 don't need Redis at all and should keep healing the pool
  during a Redis outage. The Lease lives in the K8s API, which the reconciler
  already depends on existentially.
- client-go is already a dependency; `leaderelection` is the boring,
  battle-tested option (fencing via resourceVersion CAS on the Lease, jittered
  renewals). Redlock-style locks have well-known correctness caveats under
  clock skew and pauses.

Parameters: `LeaseDuration 15s / RenewDeadline 10s / RetryPeriod 2s`
(client-go defaults). Identity = pod name (`POD_NAME` via downward API).

**Failover behavior.** If the leader dies mid-reconcile, a standby acquires
the Lease within ≤ ~15s and starts `Run()`. This is safe because the loop is
level-triggered and every mutation is idempotent or CAS'd:

- Crash after `Assign` but before `Resolve`: the owner marker + claim exist;
  the queue entry remains. The new leader's admission loop calls `Assign` →
  `ErrAlreadyExists`. **Behavior change (fix)**: instead of resolving with an
  error, the loop now does `svc.List(owner)` — if a session exists, resolve
  with it as `assigned`. The user gets the correct terminal event; the genuine
  "queued while already owning" case still resolves with the session they own,
  which is strictly better UX than the current error string.
- Crash after `ZREM` but before the terminal PUBLISH: `resolve.lua` makes
  remove+publish one atomic script, so this window doesn't exist.
- Crash between marker create and claim CAS inside `Assign`: already handled
  by the existing marker-orphan GC (step 5, 2-minute grace).
- Double-leadership during a lease handoff (old leader paused, not dead):
  bounded by RenewDeadline; if both briefly reconcile, Assign's claim CAS
  prevents double-claim, `resolve.lua`'s ZREM-returns-0 prevents double
  terminal events, and refill overshoot is corrected by trim. Degenerate, not
  incorrect.

Non-leaders still: serve all HTTP routes, run the pub/sub relay, and run
`watchLoop`? — no: the watch only pokes the local reconciler, so standbys skip
both `watchLoop` and the ticker; they just block on `leaderelection.Run`.

`svc.Assign`'s existing CAS + marker remains the last line of defense
regardless of who calls it from where — leader election is an efficiency and
ordering mechanism, not the correctness boundary for claims.

## 6. Sequence: enqueue on pod B, admit on pod A (leader), SSE on pod B

1. Client `POST /api/sessions` → **pod B**. `svc.Assign` → `ErrPoolEmpty`.
2. Pod B: `queue.Enqueue(owner, req)` → `enqueue.lua` → `INCR seq=42`,
   `ZADD queue:z 42 owner`, `SET queue:req:owner {req, enqueuedAt}` → returns
   position 3, existing=0 → `RecordEnqueued`. 202 `{position: 3}`.
3. Client opens `GET /api/queue/events` → **pod B**. `Subscribe(owner)`:
   register in pod B's local registry → `ZRANK` = 2 → emit
   `queued {position: 3}` on the SSE stream.
4. A session is deleted; the claim watch fires on **pod A** (leader) →
   `Poke()` → `reconcileOnce`.
5. Pod A step 1: `Head()` → `ZRANGE queue:z 0 0` → head owner (maybe not ours;
   loop continues FIFO). When our owner reaches head with a Ready member:
   `svc.Assign(owner, req)` → marker create + claim CAS succeed → session.
6. Pod A: `Resolve(owner, sess, "")` → `resolve.lua`: `ZREM` (=1), `GETDEL`
   payload, `PUBLISH queue:ch {owner, type:"assigned", session}`, `PUBLISH
   {type:"changed"}` → script returns payload → pod A computes wait from
   `enqueuedAt` → `RecordResolved(assigned, wait)`.
7. Pod B's relay loop receives the `assigned` message → looks up owner in its
   local registry → pushes the terminal event into the SSE channel → handler
   writes `event: assigned` and closes the stream.
8. Pod B's relay also receives `changed` → for each *other* locally-subscribed
   owner: `ZRANK` → push refreshed `queued {position}` events.

## 7. Redis unavailability & the "queue loss is safe" assumption

Redis is a new hard dependency **for queuing only**. Policy: **degrade to
reject-new-enqueues; never fall back to a local queue** (a per-pod fallback
FIFO would recreate exactly the split-brain this design removes).

| Operation | Redis down → behavior |
|---|---|
| `POST /api/sessions`, warm member available | Unaffected — direct `Assign` path never touches Redis. |
| `POST /api/sessions`, pool empty | `Enqueue` fails → **503** `queue_unavailable`, "try again shortly" (today it would 202; failing loud beats silently losing the request). |
| `GET /api/queue` (Position) | 503. |
| `GET /api/queue/events` | Existing streams stay open (heartbeats); positions freeze; relay reconnects with backoff and re-syncs. New subscribes: 503. |
| Reconcile step 1 (admission) | `Head()` error → log + `RecordReconcileError(StageAdmit)`, **skip to steps 2–5** — recycle/trim/refill/GC keep the pool healthy without Redis. |
| Marker GC queued-check (step 5) | `Position` error → treat the owner as *possibly queued* and skip GC of that marker this pass (conservative: never orphan a queued user's marker because Redis blinked). |

**Does "losing the queue is safe" still hold?** Mostly yes, with one new
wrinkle. The invariants never lived in the queue: one-per-user is the K8s
marker, claims are CAS'd, and a user whose queue entry vanishes just re-POSTs
and re-queues — unchanged. What changes:

- Queue state now *survives* backend restarts/rollouts (an improvement — today
  every deploy silently drops everyone in line).
- Queue loss becomes a *Redis* restart event instead. With `appendonly yes`
  (or a managed Redis) it's rare; without persistence it equals today's
  deploy-time behavior. Either is acceptable; recommend AOF `everysec` since
  the write rate is trivial.
- New failure surface: a subscriber whose entry vanished (flush) sits on a
  frozen stream. Healing: the relay's periodic re-sync (on reconnect, plus a
  slow position-refresh tick, e.g. 30s) notices ZRANK missing + no session →
  emits a terminal `error {"queue was reset; please retry"}` and closes.

So the doc comment gets rewritten from "losing it on restart is safe" to:
"queue state is shared and durable-ish; losing it is still *recoverable*
(clients re-POST) but is surfaced to subscribers as an explicit error, not
silence."

## 8. Config & Helm

Follow the `getenv`/`getenvInt` pattern in `internal/config/config.go`:

| Config field | Env var | Default | values.yaml key |
|---|---|---|---|
| `RedisAddr` | `REDIS_ADDR` | `""` (disabled → in-memory queue) | `config.redis.addr` |
| `RedisPassword` | `REDIS_PASSWORD` | `""` | from Secret, `config.redis.existingSecret` |
| `RedisDB` | `REDIS_DB` | `0` | `config.redis.db` |
| `RedisKeyPrefix` | `REDIS_KEY_PREFIX` | `ksbx:` | `config.redis.keyPrefix` |
| `RedisDialTimeout` | `REDIS_DIAL_TIMEOUT_SECONDS` | `5` | `config.redis.dialTimeoutSeconds` |
| `QueueMaxWait` | `QUEUE_MAX_WAIT_MINUTES` | `60` | `config.queue.maxWaitMinutes` |
| `LeaderElectionEnabled` | `LEADER_ELECTION` | `true` | `config.leaderElection.enabled` |
| (identity) | `POD_NAME` | downward API | (template) |

`QueueMaxWait` is new-behavior config: because entries now persist, the leader
janitor resolves entries older than this with a terminal error (today,
process restarts implicitly capped waits; something must replace that).

Semantics: `REDIS_ADDR` empty → `NewAssignQueue()` (in-memory), exactly
today's behavior, valid only at `replicaCount: 1`. Non-empty → `RedisQueue`.
The chart should fail template rendering if `replicaCount > 1` and
`config.redis.addr` is empty.

Chart changes besides env: a `Role`/`RoleBinding` for
`coordination.k8s.io/leases` (get/create/update) in the release namespace;
`POD_NAME` downward-API env; optionally a Redis subchart (bitnami) behind
`redis.enabled` — or point `addr` at an external/managed instance
(**open question below**).

## 9. Rollout / migration

No data migration exists to perform — the in-memory queue is legally droppable
(documented, and true: clients re-POST). Zero-downtime sequence:

1. **Ship the code** with `REDIS_ADDR` unset. Pure refactor behind the `Queue`
   interface; behavior identical; still 1 replica.
2. **Deploy Redis** (subchart or managed). No backend change yet.
3. **Set `config.redis.addr`**, rolling update at 1 replica. During the roll
   the old pod holds an in-memory queue while the new pod uses Redis; anyone
   queued on the old pod loses their entry when it terminates — identical to
   what every deploy does today. Their open SSE stream drops; client retry
   logic (re-POST) already handles it.
4. **Scale `replicaCount`** to N. Leader election was already active at 1
   replica (harmless), so nothing else changes.

Rollback at any step is the reverse; step 3's rollback re-strands Redis-queued
users symmetrically (accepted, same blast radius as a deploy).

## 10. Implementation plan (reviewable steps)

Each step compiles, passes tests, and is independently revertable.

**Step 1 — `internal/redisclient` package.** Thin wrapper over
`github.com/redis/go-redis/v9`: constructor from `config.Config`, key-prefix
helper, embedded Lua scripts (`go:embed`, `ScriptRun` with EVALSHA→EVAL
fallback), `Ping` for a readiness probe contribution. No callers yet.
Test dep: `github.com/alicebob/miniredis/v2` (supports ZSET, Lua, pub/sub).

**Step 2 — extract the `Queue` interface.** In `internal/kubernetes`:

```go
type Queue interface {
    Enqueue(ctx context.Context, owner string, req models.CreateSessionRequest) (int, error)
    Position(ctx context.Context, owner string) (int, bool, error)
    Len(ctx context.Context) (int, error)
    Head(ctx context.Context) (string, models.CreateSessionRequest, bool, error)
    Resolve(ctx context.Context, owner string, sess *models.Session, errMsg string) error
    Subscribe(ctx context.Context, owner string) (<-chan QueueEvent, func(), bool, error)
    SetMetrics(m *telemetry.Metrics)
}
```

Note the signature change: every method gains `ctx` and an `error` (Redis can
fail; the in-memory impl returns nil errors). `AssignQueue` adapts;
`SessionHandler`, `PoolManager`, `main.go` take the interface. Handlers map
queue errors → 503 `queue_unavailable`. Behavior unchanged otherwise.

**Step 3 — `RedisQueue` core.** Enqueue/Position/Len/Head/Resolve over the
ZSET model (§3), Lua for enqueue and resolve, metrics parity (`existing` flag
gates `RecordEnqueued`; resolve payload's `enqueuedAt` feeds `RecordResolved`).
`RegisterQueueDepth` reads a locally cached ZCARD refreshed by a 10s ticker —
an OTel gauge callback must never block on the network at export time.
Conformance test suite runs the same scenarios against both implementations
(dedup, 1-based positions, FIFO drain, resolve of a non-queued owner is a
no-op, depth).

**Step 4 — SSE relay.** Per-pod relay goroutine (`SUBSCRIBE queue:ch`,
reconnect + re-sync on error, slow refresh tick), local registry
`map[owner]map[chan QueueEvent]struct{}` guarded by a mutex (the moral
successor of `waiter.subs`), `Subscribe` with the register-then-check ordering
(§4). `trySend` semantics preserved. Test: two `RedisQueue` instances on one
miniredis; Subscribe on A, Resolve on B; assert terminal event and position
rebroadcast on A.

**Step 5 — PoolManager under leader election.** Wrap `Run` in
`leaderelection.RunOrDie` (leased); standbys idle. Admission loop changes:
`Head`/`Resolve` errors → record + skip admission, continue steps 2–5;
`ErrAlreadyExists` → look up the owner's session and resolve `assigned` with
it (the failover-correctness fix, §5); marker-GC treats a Redis error on
`Position` as "possibly queued". Add the janitor: orphaned `req` keys and
`QueueMaxWait` expiry.

**Step 6 — config + Helm.** `config.go` additions per §8; chart: env,
Secret-sourced `REDIS_PASSWORD`, Lease RBAC, `POD_NAME`, replicaCount guard,
optional Redis subchart. Update the README architecture section and the
`AssignQueue` doc comment (§7's rewrite).

**Step 7 — tests.**

- `pool_test.go`: existing 9 tests switch to the `Queue` interface (in-memory
  impl → minimal churn: mostly `ctx` plumbing). `TestPoolAdmitsQueuedRequests`,
  `TestPoolQueueFIFOWhenScarce`, `TestPoolEmptyAssignFallsBackToQueueFlow`
  additionally run against a miniredis-backed queue (table-driven over both
  impls).
- **New concurrency test**: one miniredis, one fake dynamic client with M
  Ready members, two `PoolManager` instances sharing the `RedisQueue`, N > M
  owners enqueued; run `reconcileOnce` concurrently from both (deliberately
  bypassing leader election to prove the belt-and-braces layer). Assert: each
  admitted owner has exactly one claim, exactly one terminal event per owner,
  no `error` terminals for owners who received sessions, FIFO prefix respected.
- **New leader election test**: two managers, fake Lease (envtest or the
  `k8s.io/client-go/tools/leaderelection` fake clock harness); kill the
  leader's ctx; assert the standby takes over and drains the remaining queue.
- `challenges_test.go:141` (`NewSessionHandler(svc, k8s.NewAssignQueue(), …)`)
  compiles unchanged once the handler takes the interface; no test logic
  changes.
- Enqueue-race test: 20 goroutines across 2 `RedisQueue` instances enqueue the
  same owner; assert depth 1 and one `RecordEnqueued`.

## 11. Open questions / trade-offs

Resolved (see the status note at the top): interface shape (ctx+error),
gauge aggregation (per-pod + dashboard `max()`), `QueueMaxWait` (60m),
Redis topology (single instance + AOF; go-redis `FailoverClient` allows a
later move to Sentinel without API changes).

Still open / accepted:

1. **Dual-queue window during the cutover roll (step 3 of §9).** Accepted:
   ~pod-termination-seconds of two coexisting queues at 1 replica. If that's
   unacceptable, `strategy: Recreate` for that one deploy closes it at the
   cost of a brief hard outage.
2. **Wait-duration histogram now crosses pods** (`enqueuedAt` written on pod
   B, read on pod A): wall-clock skew leaks into `queue.wait.duration`.
   Kubernetes nodes are NTP-disciplined; acceptable for a histogram. A
   paranoid alternative stores the enqueue `seq` and derives wait from Redis
   `TIME` at both ends.
3. **Pre-existing trim-vs-assign race (found by the concurrency test).**
   Reconcile step 3 selects trim victims from a stale LIST snapshot and
   deletes by name unconditionally — a member claimed (by a request-path
   `Assign` on any pod) between the LIST and the delete gets destroyed while
   owned. This exists **today at 1 replica**; leader election bounds it back
   to exactly today's exposure (one reconciler racing request-path assigns)
   but does not eliminate it. Proper fix, out of scope here: re-GET each
   victim and precondition the delete on `resourceVersion` (or on
   `spec.ownerRef == ""` via a JSON-patch test). The concurrency test pins
   admission-only config (`TargetWarm == MaxTotal`) to keep this separate
   race out of its assertions; a follow-up should fix and test trim itself.
