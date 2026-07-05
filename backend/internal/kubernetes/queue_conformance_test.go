package kubernetes

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	"github.com/jeremy-misola/kubesandbox/backend/internal/redisclient"
)

// newMiniRedisQueue backs a RedisQueue with a fresh miniredis. The relay is
// NOT started; queue-state semantics and local terminal delivery must work
// without it (cross-pod relay behavior is covered in queue_redis_test.go).
func newMiniRedisQueue(t *testing.T, cfg RedisQueueConfig) *RedisQueue {
	t.Helper()
	mr := miniredis.RunT(t)
	return NewRedisQueue(redisclient.New(redisclient.Options{Addr: mr.Addr()}), cfg)
}

// queueImpls runs the conformance suite against both Queue implementations.
func queueImpls(t *testing.T) map[string]Queue {
	t.Helper()
	return map[string]Queue{
		"memory": NewAssignQueue(),
		"redis":  newMiniRedisQueue(t, RedisQueueConfig{}),
	}
}

func TestQueueEnqueueDedupAndPosition(t *testing.T) {
	for name, q := range queueImpls(t) {
		t.Run(name, func(t *testing.T) {
			if pos := mustEnqueue(t, q, "alice", models.CreateSessionRequest{TTLMinutes: 30}); pos != 1 {
				t.Fatalf("first enqueue pos = %d, want 1", pos)
			}
			if pos := mustEnqueue(t, q, "bob", models.CreateSessionRequest{}); pos != 2 {
				t.Fatalf("second enqueue pos = %d, want 2", pos)
			}
			// Dedup: re-enqueue returns the existing position, adds nothing.
			if pos := mustEnqueue(t, q, "alice", models.CreateSessionRequest{}); pos != 1 {
				t.Fatalf("re-enqueue pos = %d, want 1 (dedup)", pos)
			}
			if n := queueLen(t, q); n != 2 {
				t.Fatalf("len = %d, want 2", n)
			}
			if pos, ok := queuePos(t, q, "bob"); !ok || pos != 2 {
				t.Fatalf("bob position = %d/%v, want 2/true", pos, ok)
			}
			if _, ok := queuePos(t, q, "nobody"); ok {
				t.Fatalf("unknown owner must not be queued")
			}
		})
	}
}

func TestQueueFIFOHeadResolve(t *testing.T) {
	ctx := context.Background()
	for name, q := range queueImpls(t) {
		t.Run(name, func(t *testing.T) {
			mustEnqueue(t, q, "first", models.CreateSessionRequest{TTLMinutes: 11})
			mustEnqueue(t, q, "second", models.CreateSessionRequest{TTLMinutes: 22})

			owner, req, ok, err := q.Head(ctx)
			if err != nil || !ok || owner != "first" || req.TTLMinutes != 11 {
				t.Fatalf("head = %q/%d/%v/%v, want first/11/true/nil", owner, req.TTLMinutes, ok, err)
			}
			if err := q.Resolve(ctx, "first", &models.Session{Name: "s-1"}, ""); err != nil {
				t.Fatalf("resolve: %v", err)
			}
			// Head advances FIFO; positions shift.
			owner, req, ok, err = q.Head(ctx)
			if err != nil || !ok || owner != "second" || req.TTLMinutes != 22 {
				t.Fatalf("head after resolve = %q/%d/%v/%v, want second/22/true/nil", owner, req.TTLMinutes, ok, err)
			}
			if pos, ok := queuePos(t, q, "second"); !ok || pos != 1 {
				t.Fatalf("second position = %d/%v, want 1/true after head resolved", pos, ok)
			}
			if err := q.Resolve(ctx, "second", nil, "boom"); err != nil {
				t.Fatalf("resolve error-terminal: %v", err)
			}
			if _, _, ok, _ := q.Head(ctx); ok {
				t.Fatalf("queue should be empty")
			}
		})
	}
}

func TestQueueResolveNotQueuedIsNoop(t *testing.T) {
	ctx := context.Background()
	for name, q := range queueImpls(t) {
		t.Run(name, func(t *testing.T) {
			if err := q.Resolve(ctx, "ghost", nil, "nope"); err != nil {
				t.Fatalf("resolving an absent owner must be a no-op, got %v", err)
			}
		})
	}
}

func TestQueueSubscribeInitialPositionAndTerminal(t *testing.T) {
	ctx := context.Background()
	for name, q := range queueImpls(t) {
		t.Run(name, func(t *testing.T) {
			// Not queued → no subscription.
			if _, _, ok, err := q.Subscribe(ctx, "ghost"); ok || err != nil {
				t.Fatalf("subscribe for unqueued owner = %v/%v, want false/nil", ok, err)
			}

			mustEnqueue(t, q, "alice", models.CreateSessionRequest{})
			mustEnqueue(t, q, "bob", models.CreateSessionRequest{})
			ch, unsub, ok, err := q.Subscribe(ctx, "bob")
			if err != nil || !ok {
				t.Fatalf("subscribe = %v/%v, want true/nil", ok, err)
			}
			defer unsub()

			first := <-ch
			if first.Type != "queued" || first.Position != 2 {
				t.Fatalf("initial event = %+v, want queued pos 2", first)
			}

			// Terminal delivery closes the channel after exactly one
			// assigned/error event.
			if err := q.Resolve(ctx, "bob", &models.Session{Name: "s-bob"}, ""); err != nil {
				t.Fatalf("resolve: %v", err)
			}
			var last QueueEvent
			for ev := range ch {
				last = ev
			}
			if last.Type != "assigned" || last.Session == nil || last.Session.Name != "s-bob" {
				t.Fatalf("terminal = %+v, want assigned s-bob", last)
			}
		})
	}
}
