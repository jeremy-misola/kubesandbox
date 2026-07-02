package kubernetes

import (
	"sync"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
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

// waiter is one queued owner. Subscribers receive position updates and the
// terminal assigned/error event; events are delivered best-effort (a slow
// subscriber never blocks the queue — the session still exists and is
// discoverable via GET /api/sessions).
type waiter struct {
	owner string
	req   models.CreateSessionRequest
	subs  map[chan QueueEvent]struct{}
}

// AssignQueue is an in-memory FIFO of owners waiting for a warm sandbox.
//
// Scope note: the queue is per-replica state (the backend runs a single
// replica today). Losing it on restart is safe — the user re-POSTs and either
// gets a member (pool refilled) or re-queues. The one-per-user invariant does
// NOT live here; it lives in the per-owner marker created at assignment time.
type AssignQueue struct {
	mu    sync.Mutex
	items []*waiter
}

// NewAssignQueue constructs an empty queue.
func NewAssignQueue() *AssignQueue { return &AssignQueue{} }

// Enqueue adds owner to the queue (deduplicated) and returns its 1-based
// position.
func (q *AssignQueue) Enqueue(owner string, req models.CreateSessionRequest) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, w := range q.items {
		if w.owner == owner {
			return i + 1
		}
	}
	q.items = append(q.items, &waiter{owner: owner, req: req, subs: map[chan QueueEvent]struct{}{}})
	return len(q.items)
}

// Position returns the 1-based position of owner, or false if not queued.
func (q *AssignQueue) Position(owner string) (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, w := range q.items {
		if w.owner == owner {
			return i + 1, true
		}
	}
	return 0, false
}

// Len returns the queue length.
func (q *AssignQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Head returns the owner and request at the front of the queue.
func (q *AssignQueue) Head() (string, models.CreateSessionRequest, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return "", models.CreateSessionRequest{}, false
	}
	return q.items[0].owner, q.items[0].req, true
}

// Subscribe attaches an SSE listener to owner's queue entry. It returns the
// event channel, an unsubscribe func, and whether the owner was queued. The
// current position is delivered immediately.
func (q *AssignQueue) Subscribe(owner string) (<-chan QueueEvent, func(), bool) {
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
			}, true
		}
	}
	return nil, nil, false
}

// Resolve removes owner from the queue, delivering the terminal event to
// subscribers: an "assigned" event when sess is non-nil, else an "error".
// Remaining waiters receive updated positions.
func (q *AssignQueue) Resolve(owner string, sess *models.Session, errMsg string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, w := range q.items {
		if w.owner != owner {
			continue
		}
		ev := QueueEvent{Type: "assigned", Session: sess}
		if sess == nil {
			ev = QueueEvent{Type: "error", Message: errMsg}
		}
		for ch := range w.subs {
			trySend(ch, ev)
			close(ch)
		}
		q.items = append(q.items[:i], q.items[i+1:]...)
		q.broadcastPositionsLocked()
		return
	}
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
