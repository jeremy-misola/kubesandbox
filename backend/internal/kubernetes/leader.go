package kubernetes

import (
	"context"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// poolLeaseName is the coordination Lease guarding the pool reconcile loop:
// exactly one backend replica runs PoolManager.Run at a time
// (docs/redis-queue-horizontal-scaling.md §5).
const poolLeaseName = "kubesandbox-backend-pool"

// RunWithLeaderElection runs fn only while this replica holds the pool Lease.
// It blocks until ctx is done, re-campaigning whenever leadership is lost.
//
// Failover: if the holder dies mid-reconcile, a standby acquires the Lease
// within ~LeaseDuration (15s). This is safe because reconciliation is
// level-triggered and every mutation is idempotent or CAS'd; the
// crashed-between-Assign-and-Resolve window is healed by the admission loop's
// ErrAlreadyExists → resolve-with-owned-session path.
func RunWithLeaderElection(ctx context.Context, client clientgo.Interface, namespace, identity string, fn func(ctx context.Context)) {
	lock := &resourcelock.LeaseLock{
		LeaseMeta:  metav1.ObjectMeta{Name: poolLeaseName, Namespace: namespace},
		Client:     client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{Identity: identity},
	}
	for ctx.Err() == nil {
		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true, // clean shutdown hands off without waiting out the lease
			LeaseDuration:   15 * time.Second,
			RenewDeadline:   10 * time.Second,
			RetryPeriod:     2 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(leaderCtx context.Context) {
					log.Printf("pool: acquired leadership (%s)", identity)
					fn(leaderCtx)
				},
				OnStoppedLeading: func() {
					log.Printf("pool: lost leadership (%s)", identity)
				},
				OnNewLeader: func(id string) {
					if id != identity {
						log.Printf("pool: current leader is %s", id)
					}
				},
			},
		})
		// RunOrDie returns when leadership is lost or ctx is done; loop to
		// re-campaign (a paused-then-resumed pod must be able to lead again).
	}
}
