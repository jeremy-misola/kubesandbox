package kubernetes

import (
	"context"
	"sync"
	"time"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	"github.com/jeremy-misola/kubesandbox/backend/internal/telemetry"
)

// QueueEvent is streamed to SSE subscribers of a queued create request.
type QueueEvent struct {
	// Type is "queued" (position update), "assigned" (Session set), or
	// "error" (Message set).
	Type     string          `json:"type"`
	Position int             `json:"position,omitempty"`
	Session  *models.Session `json:"session,omitempty"`
	Message  string          `json:"message,omitempty"`
}

// Queue is the warm-pool waiting line (docs/redis-queue-horizontal-scaling.md).
//
// Two implementations exist: AssignQueue (in-memory, single-replica only) and
// RedisQueue (shared state, safe behind N replicas). Every method takes a ctx
// and can return an error because the Redis implementation crosses the
// network; the in-memory implementation always returns nil errors.
//
// Semantics common to both:
//   - Enqueue dedups by owner and returns the 1-based position.
//   - Head/Resolve drain FIFO. Resolving an owner that is not queued is a
//     harmless no-op.
//   - Subscribe delivers best-effort position updates and exactly one
//     terminal "assigned"/"error" event, after which the channel is closed.
//     A slow subscriber never blocks the queue.
type Queue interface {
	Enqueue(ctx context.Context, owner string, req models.CreateSessionRequest) (int, error)
	Position(ctx context.Context, owner string) (int, bool, error)
	Len(ctx context.Context) (int, error)
	Head(ctx context.Context) (string, models.CreateSessionRequest, bool, error)
	Resolve(ctx context.Context, owner string, sess *models.Session, errMsg string) error
	Subscribe(ctx context.Context, owner string) (<-chan QueueEvent, func(), bool, error)
	SetMetrics(m *telemetry.Metrics)
}

// waiter is one queued owner. Subscribers receive position updates and the
// terminal assigned/error event; events are delivered best-effort (a slow
// subscriber never blocks the queue — the session still exists and is
// discoverable via GET /api/sessions).
type waiter struct {
	owner string
	req   models.CreateSessionRequest
	subs  map[chan QueueEvent]struct{}
	// enqueuedAt feeds the queue.wait.duration histogram on Resolve.
	enqueuedAt time.Time
}

// AssignQueue is an in-memory FIFO of owners waiting for a warm sandbox.
//
// Scope note: this implementation is per-replica state and is only valid when
// the backend runs a single replica (REDIS_ADDR unset). Losing it on restart
// is safe — the user re-POSTs and either gets a member (pool refilled) or
// re-queues. The one-per-user invariant does NOT live here; it lives in the
// per-owner marker created at assignment time. For N replicas, use RedisQueue.
type AssignQueue struct {
	mu    sync.Mutex
	items []*waiter

	// metrics is the injected instrument set; nil is a valid no-op.
	metrics *telemetry.Metrics
}

// compile-time interface check.
var _ Queue = (*AssignQueue)(nil)

// NewAssignQueue constructs an empty queue.
func NewAssignQueue() *AssignQueue { return &AssignQueue{} }

// SetMetrics injects the telemetry instrument set (nil is a valid no-op) and
// wires the queue.depth gauge to the queue length.
func (q *AssignQueue) SetMetrics(m *telemetry.Metrics) {
	q.metrics = m
	m.RegisterQueueDepth(func() int64 {
		q.mu.Lock()
		defer q.mu.Unlock()
		return int64(len(q.items))
	})
}

// Enqueue adds owner to the queue (deduplicated) and returns its 1-based
// position.
func (q *AssignQueue) Enqueue(_ context.Context, owner string, req models.CreateSessionRequest) (int, error) {
	q.mu.Lock()
	for i, w := range q.items {
		if w.owner == owner {
			pos := i + 1
			q.mu.Unlock()
			return pos, nil // already queued — not a new enqueue, nothing to record
		}
	}
	q.items = append(q.items, &waiter{
		owner:      owner,
		req:        req,
		subs:       map[chan QueueEvent]struct{}{},
		enqueuedAt: time.Now(),
	})
	pos := len(q.items)
	q.mu.Unlock()

	// Record outside the lock: an instrument Add must never sit on the queue's
	// hot path (cheap today, but the SDK is not ours to trust to never block).
	q.metrics.RecordEnqueued(context.Background())
	return pos, nil
}

// Position returns the 1-based position of owner, or false if not queued.
func (q *AssignQueue) Position(_ context.Context, owner string) (int, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, w := range q.items {
		if w.owner == owner {
			return i + 1, true, nil
		}
	}
	return 0, false, nil
}

// Len returns the queue length.
func (q *AssignQueue) Len(_ context.Context) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items), nil
}

// Head returns the owner and request at the front of the queue.
func (q *AssignQueue) Head(_ context.Context) (string, models.CreateSessionRequest, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return "", models.CreateSessionRequest{}, false, nil
	}
	return q.items[0].owner, q.items[0].req, true, nil
}

// Subscribe attaches an SSE listener to owner's queue entry. It returns the
// event channel, an unsubscribe func, and whether the owner was queued. The
// current position is delivered immediately.
func (q *AssignQueue) Subscribe(_ context.Context, owner string) (<-chan QueueEvent, func(), bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, w := range q.items {
		if w.owner == owner {
			ch := make(chan QueueEvent, 8)
			w.subs[ch] = struct{}{}
			trySend(ch, QueueEvent{Type: "queued", Position: i + 1})
			ww := w
			return ch, func() {
				q.mu.Lock()
				delete(ww.subs, ch)
				q.mu.Unlock()
			}, true, nil
		}
	}
	return nil, nil, false, nil
}

// Resolve removes owner from the queue, delivering the terminal event to
// subscribers: an "assigned" event when sess is non-nil, else an "error".
// Remaining waiters receive updated positions.
func (q *AssignQueue) Resolve(_ context.Context, owner string, sess *models.Session, errMsg string) error {
	q.mu.Lock()
	found := false
	outcome := telemetry.OutcomeAssigned
	var wait time.Duration
	for i, w := range q.items {
		if w.owner != owner {
			continue
		}
		ev := QueueEvent{Type: "assigned", Session: sess}
		if sess == nil {
			ev = QueueEvent{Type: "error", Message: errMsg}
			outcome = telemetry.OutcomeError
		}
		wait = time.Since(w.enqueuedAt)
		found = true
		for ch := range w.subs {
			trySend(ch, ev)
			close(ch)
		}
		q.items = append(q.items[:i], q.items[i+1:]...)
		q.broadcastPositionsLocked()
		break
	}
	q.mu.Unlock()

	// Record outside the lock (see Enqueue).
	if found {
		q.metrics.RecordResolved(context.Background(), outcome, wait)
	}
	return nil
}

// broadcastPositionsLocked pushes fresh positions to every subscriber. Caller
// must hold q.mu.
func (q *AssignQueue) broadcastPositionsLocked() {
	for i, w := range q.items {
		for ch := range w.subs {
			trySend(ch, QueueEvent{Type: "queued", Position: i + 1})
		}
	}
}

// trySend never blocks: queue progress is best-effort delivery.
func trySend(ch chan QueueEvent, ev QueueEvent) {
	select {
	case ch <- ev:
	default:
	}
}
