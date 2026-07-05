package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	"github.com/jeremy-misola/kubesandbox/backend/internal/redisclient"
)

// TestConcurrentReconcilersDoNotDoubleAdmit simulates two backend replicas
// racing reconcileOnce against one shared Redis queue and one cluster (shared
// fake dynamic client whose update reactor enforces resourceVersion CAS).
// Leader election normally serializes this; the test deliberately bypasses it
// to prove the belt-and-braces layer (owner-marker create + claim CAS +
// atomic ZREM in resolve) still guarantees:
//
//   - no owner is admitted twice (exactly one claim per admitted owner),
//   - no Ready member is claimed by two owners,
//   - FIFO: with M members and N>M waiters, the first M owners win,
//   - the queue retains exactly the un-admitted owners, in order.
func TestConcurrentReconcilersDoNotDoubleAdmit(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	const members = 3
	const waiters = 5

	// Shared cluster: 3 Ready members, MaxTotal == members so refill can't
	// add capacity and the race is purely over admission.
	ros := make([]runtime.Object, 0, members)
	for i := 1; i <= members; i++ {
		ros = append(ros, poolMember(fmt.Sprintf("s-pool-%d", i), now.Add(-10*time.Minute), true))
	}
	svc, _ := newFakeService(t, ros...)
	svc.now = func() time.Time { return now }

	// Shared queue: one RedisQueue instance per "replica", same miniredis.
	mr := miniredis.RunT(t)
	qa := NewRedisQueue(redisclient.New(redisclient.Options{Addr: mr.Addr()}), RedisQueueConfig{})
	qb := NewRedisQueue(redisclient.New(redisclient.Options{Addr: mr.Addr()}), RedisQueueConfig{})

	// TargetWarm == MaxTotal == members keeps steps 2-4 (recycle/trim/refill)
	// quiescent: this test isolates the ADMISSION race. Concurrent trim over a
	// stale LIST snapshot is a separate, pre-existing race (it can delete a
	// just-claimed member even at 1 replica when a request-path Assign lands
	// mid-reconcile) that leader election intentionally does not address —
	// see the design doc's open questions.
	cfg := PoolConfig{TargetWarm: members, MaxTotal: members}
	pmA := NewPoolManager(svc, qa, cfg)
	pmA.now = func() time.Time { return now }
	pmB := NewPoolManager(svc, qb, cfg)
	pmB.now = func() time.Time { return now }

	ctx := context.Background()
	for i := 1; i <= waiters; i++ {
		mustEnqueue(t, qa, fmt.Sprintf("owner-%d", i), models.CreateSessionRequest{})
	}

	// Race: both "replicas" reconcile repeatedly and concurrently. Multiple
	// passes are needed because a loser backs off for the pass when it
	// observes a marker whose claim is still in flight.
	var wg sync.WaitGroup
	for _, pm := range []*PoolManager{pmA, pmB} {
		wg.Add(1)
		go func(pm *PoolManager) {
			defer wg.Done()
			for i := 0; i < 8; i++ {
				_ = pm.reconcileOnce(ctx)
			}
		}(pm)
	}
	wg.Wait()

	// Every admitted owner holds exactly one claim.
	claimsByOwner := map[string]int{}
	claims, err := svc.listManaged(ctx)
	if err != nil {
		t.Fatalf("listManaged: %v", err)
	}
	for i := range claims {
		if o := specOwner(&claims[i]); o != "" {
			claimsByOwner[o]++
		}
	}
	for owner, n := range claimsByOwner {
		if n != 1 {
			t.Fatalf("owner %q holds %d claims, want 1", owner, n)
		}
	}
	if len(claimsByOwner) != members {
		t.Fatalf("admitted %d owners, want %d (one per ready member): %v",
			len(claimsByOwner), members, claimsByOwner)
	}

	// FIFO: the first `members` owners won; the rest still wait in order.
	for i := 1; i <= members; i++ {
		owner := fmt.Sprintf("owner-%d", i)
		if claimsByOwner[owner] != 1 {
			t.Fatalf("FIFO violated: %s not admitted (admitted set %v)", owner, claimsByOwner)
		}
	}
	if n := queueLen(t, qb); n != waiters-members {
		t.Fatalf("queue len = %d, want %d", n, waiters-members)
	}
	for i := members + 1; i <= waiters; i++ {
		owner := fmt.Sprintf("owner-%d", i)
		pos, ok := queuePos(t, qb, owner)
		if !ok || pos != i-members {
			t.Fatalf("%s position = %d/%v, want %d/true", owner, pos, ok, i-members)
		}
	}
}
