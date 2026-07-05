package kubernetes

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	"github.com/jeremy-misola/kubesandbox/backend/internal/redisclient"
)

// redisZ builds a ZSET member for test fixtures.
func redisZ(score float64, member string) redis.Z { return redis.Z{Score: score, Member: member} }

// twoRedisQueues simulates two backend replicas sharing one Redis: separate
// RedisQueue instances (separate local registries and relays) on one
// miniredis. Both relays are started and confirmed subscribed.
func twoRedisQueues(t *testing.T) (qa, qb *RedisQueue, ctx context.Context) {
	t.Helper()
	mr := miniredis.RunT(t)
	qa = NewRedisQueue(redisclient.New(redisclient.Options{Addr: mr.Addr()}), RedisQueueConfig{})
	qb = NewRedisQueue(redisclient.New(redisclient.Options{Addr: mr.Addr()}), RedisQueueConfig{})

	cctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go qa.Run(cctx)
	go qb.Run(cctx)
	for _, q := range []*RedisQueue{qa, qb} {
		select {
		case <-q.relayReady:
		case <-time.After(5 * time.Second):
			t.Fatalf("relay did not become ready")
		}
	}
	return qa, qb, cctx
}

// waitEvent receives one QueueEvent with a timeout.
func waitEvent(t *testing.T, ch <-chan QueueEvent, want string) QueueEvent {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, open := <-ch:
			if !open {
				t.Fatalf("channel closed while waiting for %q", want)
			}
			if ev.Type == want {
				return ev
			}
			// skip interleaved position updates while waiting for a terminal
			if want != "queued" && ev.Type == "queued" {
				continue
			}
			t.Fatalf("event = %+v, want type %q", ev, want)
		case <-deadline:
			t.Fatalf("timed out waiting for %q event", want)
		}
	}
}

// TestRedisQueueCrossInstanceTerminal is the two-replica SSE flow from the
// design doc §6: subscribe on pod A, resolve on pod B, terminal arrives at A.
func TestRedisQueueCrossInstanceTerminal(t *testing.T) {
	qa, qb, ctx := twoRedisQueues(t)

	if _, err := qb.Enqueue(ctx, "alice", models.CreateSessionRequest{TTLMinutes: 30}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	ch, unsub, ok, err := qa.Subscribe(ctx, "alice")
	if err != nil || !ok {
		t.Fatalf("subscribe on qa = %v/%v", ok, err)
	}
	defer unsub()
	if ev := waitEvent(t, ch, "queued"); ev.Position != 1 {
		t.Fatalf("initial position = %d, want 1", ev.Position)
	}

	// Resolve on the OTHER instance; the event must cross via pub/sub.
	if err := qb.Resolve(ctx, "alice", &models.Session{Name: "s-x"}, ""); err != nil {
		t.Fatalf("resolve on qb: %v", err)
	}
	ev := waitEvent(t, ch, "assigned")
	if ev.Session == nil || ev.Session.Name != "s-x" {
		t.Fatalf("assigned event = %+v, want session s-x", ev)
	}
}

// TestRedisQueueCrossInstancePositionRebroadcast: resolving the head on pod B
// must refresh the positions of pod A's remaining subscribers.
func TestRedisQueueCrossInstancePositionRebroadcast(t *testing.T) {
	qa, qb, ctx := twoRedisQueues(t)

	qb.Enqueue(ctx, "head", models.CreateSessionRequest{})
	qb.Enqueue(ctx, "tail", models.CreateSessionRequest{})

	ch, unsub, ok, err := qa.Subscribe(ctx, "tail")
	if err != nil || !ok {
		t.Fatalf("subscribe = %v/%v", ok, err)
	}
	defer unsub()
	if ev := waitEvent(t, ch, "queued"); ev.Position != 2 {
		t.Fatalf("initial position = %d, want 2", ev.Position)
	}

	if err := qb.Resolve(ctx, "head", &models.Session{Name: "s-h"}, ""); err != nil {
		t.Fatalf("resolve head: %v", err)
	}
	// The "changed" relay message triggers a ZRANK refresh on qa.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Type == "queued" && ev.Position == 1 {
				return // rebroadcast observed
			}
		case <-deadline:
			t.Fatalf("no position-1 rebroadcast after head resolve")
		}
	}
}

// TestRedisQueueEnqueueRaceDedup: concurrent enqueues of one owner across two
// instances must produce exactly one entry (Lua enqueue is atomic).
func TestRedisQueueEnqueueRaceDedup(t *testing.T) {
	qa, qb, ctx := twoRedisQueues(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		for _, q := range []*RedisQueue{qa, qb} {
			wg.Add(1)
			go func(q *RedisQueue) {
				defer wg.Done()
				q.Enqueue(ctx, "same-owner", models.CreateSessionRequest{})
			}(q)
		}
	}
	wg.Wait()
	if n := queueLen(t, qa); n != 1 {
		t.Fatalf("len = %d after racing enqueues, want 1", n)
	}
}

// TestRedisQueueJanitor covers max-wait expiry and payload/order divergence
// self-healing.
func TestRedisQueueJanitor(t *testing.T) {
	mr := miniredis.RunT(t)
	q := NewRedisQueue(redisclient.New(redisclient.Options{Addr: mr.Addr()}),
		RedisQueueConfig{MaxWait: 50 * time.Millisecond})
	ctx := context.Background()

	q.Enqueue(ctx, "old", models.CreateSessionRequest{})
	ch, unsub, ok, err := q.Subscribe(ctx, "old")
	if err != nil || !ok {
		t.Fatalf("subscribe = %v/%v", ok, err)
	}
	defer unsub()
	waitEvent(t, ch, "queued")

	// Orphaned payload (resolved entry whose DEL was lost).
	q.rdb.Set(ctx, q.reqKey("ghost"), `{"req":{},"enqueuedAt":"2026-01-01T00:00:00Z"}`, 0)
	// Order entry without payload (half-write leak).
	q.rdb.ZAdd(ctx, q.zKey(), redisZ(1000, "no-payload"))

	time.Sleep(60 * time.Millisecond) // let "old" exceed MaxWait

	if err := q.Janitor(ctx); err != nil {
		t.Fatalf("janitor: %v", err)
	}

	// Expired entry got a terminal error (delivered locally without relay).
	ev := waitEvent(t, ch, "error")
	if ev.Message == "" {
		t.Fatalf("expiry terminal should carry a message")
	}
	if n := queueLen(t, q); n != 0 {
		t.Fatalf("len = %d after janitor, want 0", n)
	}
	if err := q.rdb.Get(ctx, q.reqKey("ghost")).Err(); err == nil {
		t.Fatalf("orphaned payload should be deleted")
	}
}

// TestRedisQueueHeadSkipsCorruptEntries: a head entry with no payload must be
// dropped, not wedge admission.
func TestRedisQueueHeadSkipsCorruptEntries(t *testing.T) {
	mr := miniredis.RunT(t)
	q := NewRedisQueue(redisclient.New(redisclient.Options{Addr: mr.Addr()}), RedisQueueConfig{})
	ctx := context.Background()

	q.rdb.ZAdd(ctx, q.zKey(), redisZ(1, "broken")) // no payload
	q.Enqueue(ctx, "fine", models.CreateSessionRequest{TTLMinutes: 7})

	owner, req, ok, err := q.Head(ctx)
	if err != nil || !ok || owner != "fine" || req.TTLMinutes != 7 {
		t.Fatalf("head = %q/%d/%v/%v, want fine/7/true/nil", owner, req.TTLMinutes, ok, err)
	}
	if n := queueLen(t, q); n != 1 {
		t.Fatalf("len = %d, want 1 (broken entry dropped)", n)
	}
}

// TestRedisQueueUnavailableSurfacesErrors: with Redis down, every queue
// operation returns an error (handlers map these to 503) instead of a wrong
// answer.
func TestRedisQueueUnavailableSurfacesErrors(t *testing.T) {
	mr := miniredis.RunT(t)
	q := NewRedisQueue(redisclient.New(redisclient.Options{Addr: mr.Addr()}), RedisQueueConfig{})
	ctx := context.Background()
	q.Enqueue(ctx, "alice", models.CreateSessionRequest{})

	mr.Close() // Redis outage

	if _, err := q.Enqueue(ctx, "bob", models.CreateSessionRequest{}); err == nil {
		t.Fatalf("enqueue must fail when redis is down")
	}
	if _, _, err := q.Position(ctx, "alice"); err == nil {
		t.Fatalf("position must fail when redis is down")
	}
	if _, _, _, err := q.Head(ctx); err == nil {
		t.Fatalf("head must fail when redis is down")
	}
	if _, _, _, err := q.Subscribe(ctx, "alice"); err == nil {
		t.Fatalf("subscribe must fail when redis is down")
	}
}
