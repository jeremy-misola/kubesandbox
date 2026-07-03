package kubernetes

import (
	"context"
	"errors"
	"log"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jeremy-misola/kubesandbox/backend/internal/telemetry"
)

// markerOrphanGrace is how long an owner marker may exist without a matching
// owned claim before the GC removes it. It covers the window between marker
// create and member claim in Assign; anything older is a leak from a crashed
// request that would otherwise block that user forever.
const markerOrphanGrace = 2 * time.Minute

// PoolConfig sizes the hot pool. All values are Helm-configurable.
type PoolConfig struct {
	// TargetWarm is how many hot, unclaimed, Ready sandboxes to maintain.
	TargetWarm int
	// MaxTotal is the concurrent-session ceiling: warm + live never exceed it.
	MaxTotal int
	// MaxWarmAge: an available member older than this is recycled, not handed out.
	MaxWarmAge time.Duration
	// Resync is the periodic full-reconcile interval (backstop behind the watch).
	Resync time.Duration
}

// PoolManager maintains the hot warm pool: it keeps TargetWarm unclaimed
// sandboxes Ready, replaces claimed/recycled ones, admits queued requests as
// members become Ready, recycles stale members, and GCs orphaned markers.
//
// It is event-driven (a watch on managed claims pokes the loop) with a periodic
// resync backstop. Reconciliation is level-based: every pass recomputes desired
// actions from a fresh LIST, so missed events are harmless.
type PoolManager struct {
	svc   *SessionService
	queue *AssignQueue
	cfg   PoolConfig
	now   func() time.Time
	poke  chan struct{}

	// metrics is the injected instrument set; nil is a valid no-op.
	metrics *telemetry.Metrics
	// pendingReady tracks not-yet-Ready members observed by reconcileOnce so
	// the first observed not-Ready -> Ready transition yields an approximate
	// provision latency (creationTimestamp -> first Ready sighting). Members
	// already Ready at process start are never recorded, avoiding restart
	// skew. Only touched from the Run goroutine; no locking needed.
	pendingReady map[string]struct{}
}

// NewPoolManager constructs a PoolManager. Zero/negative config values get safe
// defaults (target 2, cap 60, warm age 24h, resync 30s).
func NewPoolManager(svc *SessionService, queue *AssignQueue, cfg PoolConfig) *PoolManager {
	if cfg.TargetWarm <= 0 {
		cfg.TargetWarm = 2
	}
	if cfg.MaxTotal <= 0 {
		cfg.MaxTotal = 60
	}
	if cfg.MaxWarmAge <= 0 {
		cfg.MaxWarmAge = 24 * time.Hour
	}
	if cfg.Resync <= 0 {
		cfg.Resync = 30 * time.Second
	}
	svc.SetMaxWarmAge(cfg.MaxWarmAge)
	return &PoolManager{
		svc:          svc,
		queue:        queue,
		cfg:          cfg,
		now:          time.Now,
		poke:         make(chan struct{}, 1),
		pendingReady: map[string]struct{}{},
	}
}

// SetMetrics injects the telemetry instrument set (nil is a valid no-op) and
// publishes the static pool-config gauges.
func (p *PoolManager) SetMetrics(m *telemetry.Metrics) {
	p.metrics = m
	m.SetPoolConfig(int64(p.cfg.TargetWarm), int64(p.cfg.MaxTotal))
}

// Poke requests an immediate reconcile (non-blocking; coalesced).
func (p *PoolManager) Poke() {
	select {
	case p.poke <- struct{}{}:
	default:
	}
}

// Run reconciles until ctx is cancelled. A watch on managed claims triggers
// immediate reconciles; the ticker is the level-based backstop.
func (p *PoolManager) Run(ctx context.Context) {
	log.Printf("pool: manager started (target=%d cap=%d maxWarmAge=%s resync=%s)",
		p.cfg.TargetWarm, p.cfg.MaxTotal, p.cfg.MaxWarmAge, p.cfg.Resync)

	go p.watchLoop(ctx)

	ticker := time.NewTicker(p.cfg.Resync)
	defer ticker.Stop()
	for {
		start := time.Now()
		err := p.reconcileOnce(ctx)
		p.metrics.RecordReconcile(ctx, time.Since(start), err)
		if err != nil {
			log.Printf("pool: reconcile error: %v", err)
		}
		select {
		case <-ctx.Done():
			log.Printf("pool: manager stopped")
			return
		case <-ticker.C:
		case <-p.poke:
		}
	}
}

// watchLoop pokes the reconciler on any managed-claim event, reconnecting with
// a small backoff when the watch drops.
func (p *PoolManager) watchLoop(ctx context.Context) {
	for {
		w, err := p.svc.WatchManaged(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
		for range w.ResultChan() {
			p.Poke()
		}
		w.Stop()
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// reconcileOnce runs one level-based pass: admit queued users, recycle stale
// members, trim overshoot, refill to target within the ceiling, GC markers.
func (p *PoolManager) reconcileOnce(ctx context.Context) error {
	claims, err := p.svc.listManaged(ctx)
	if err != nil {
		return err
	}

	now := p.now()
	total := 0   // every managed claim occupies capacity until fully gone
	claimed := 0 // live (non-deleting) claims held by an owner
	var fresh, stale, notReady []unstructured.Unstructured
	owners := map[string]bool{}             // owners holding a live (non-deleting) claim
	presentPending := map[string]struct{}{} // not-Ready members seen this pass

	for i := range claims {
		c := claims[i]
		total++
		if c.GetDeletionTimestamp() != nil {
			continue
		}
		if o := specOwner(&c); o != "" {
			owners[o] = true
			claimed++
		}
		if poolState(&c) != poolAvailable {
			continue
		}
		age := now.Sub(c.GetCreationTimestamp().Time)
		ready, _, _ := unstructured.NestedBool(c.Object, "status", "workspaceReady")

		// Approximate provision latency: first observed not-Ready -> Ready
		// transition, measured warm create -> this sighting. Members already
		// Ready when the process started were never pending here, so restarts
		// don't skew the histogram.
		//
		// Resolution note: the "Ready sighting" is only as fresh as the reconcile
		// cadence, so each sample carries up to ~cfg.Resync (default 30s) of
		// upward quantization error. Trust the histogram for coarse buckets
		// (minutes), not sub-Resync movements; a precise figure would read the
		// composition/status readiness timestamp instead of the sighting time.
		name := c.GetName()
		if ready {
			if _, wasPending := p.pendingReady[name]; wasPending {
				delete(p.pendingReady, name)
				p.metrics.RecordProvisionDuration(ctx, now.Sub(c.GetCreationTimestamp().Time))
			}
		} else {
			p.pendingReady[name] = struct{}{}
			presentPending[name] = struct{}{}
		}

		switch {
		case age > p.cfg.MaxWarmAge:
			stale = append(stale, c)
		case ready:
			fresh = append(fresh, c)
		default:
			notReady = append(notReady, c)
		}
	}

	// Forget pending members that vanished before turning Ready (trimmed or
	// recycled), so the tracking map cannot grow unbounded.
	for name := range p.pendingReady {
		if _, ok := presentPending[name]; !ok {
			delete(p.pendingReady, name)
		}
	}

	// Publish pool-state gauges from this pass's level-based truth. The
	// asynchronous gauges read these values at export time.
	p.metrics.SetPoolState(int64(len(fresh)), int64(len(notReady)), int64(claimed), int64(total))

	// 1) Admit queued requests while Ready members exist. Assign re-lists and
	// CASes internally, so this is safe against concurrent request-path
	// assignments; ErrPoolEmpty just means someone else got there first.
	admitted := 0
	for admitted < len(fresh) {
		owner, req, ok := p.queue.Head()
		if !ok {
			break
		}
		sess, err := p.svc.Assign(ctx, owner, req)
		switch {
		case err == nil:
			log.Printf("pool: admitted queued owner to %s", sess.Name)
			p.queue.Resolve(owner, sess, "")
			p.metrics.RecordClaimed(ctx, telemetry.SourceQueue)
			admitted++
		case errors.Is(err, ErrAlreadyExists):
			p.queue.Resolve(owner, nil, "you already have a sandbox")
		case errors.Is(err, ErrPoolEmpty):
			return nil // no members left this pass; refill below next pass
		default:
			log.Printf("pool: queued assignment for head failed: %v", err)
			p.queue.Resolve(owner, nil, "assignment failed; please retry")
		}
	}
	availableNow := len(fresh) - admitted

	// 2) Recycle stale members (rebuilt fresh by refill).
	for i := range stale {
		name := stale[i].GetName()
		log.Printf("pool: recycling stale member %s (age > %s)", name, p.cfg.MaxWarmAge)
		if err := p.svc.deleteByName(ctx, name); err != nil {
			log.Printf("pool: recycle %s failed: %v", name, err)
			p.metrics.RecordReconcileError(ctx, telemetry.StageRecycle)
		} else {
			p.metrics.RecordRecycled(ctx, telemetry.ReasonStale)
			total-- // capacity frees asynchronously; don't double-provision against it
		}
	}

	// 3) Trim overshoot: delete the YOUNGEST unclaimed members beyond target,
	// preferring not-Ready ones.
	warm := availableNow + len(notReady)
	if excess := warm - p.cfg.TargetWarm; excess > 0 {
		victims := append([]unstructured.Unstructured{}, notReady...)
		victims = append(victims, fresh...)
		sort.Slice(victims, func(i, j int) bool { // youngest first
			return victims[i].GetCreationTimestamp().Time.After(victims[j].GetCreationTimestamp().Time)
		})
		for i := 0; i < excess && i < len(victims); i++ {
			name := victims[i].GetName()
			log.Printf("pool: trimming excess member %s", name)
			if err := p.svc.deleteByName(ctx, name); err != nil {
				log.Printf("pool: trim %s failed: %v", name, err)
				p.metrics.RecordReconcileError(ctx, telemetry.StageTrim)
			} else {
				p.metrics.RecordRecycled(ctx, telemetry.ReasonTrim)
				warm--
				total--
			}
		}
	}

	// 4) Refill to target, respecting the concurrent ceiling (warm + live).
	for warm < p.cfg.TargetWarm && total < p.cfg.MaxTotal {
		sess, err := p.svc.CreateWarm(ctx)
		if err != nil {
			log.Printf("pool: warm provision failed: %v", err)
			p.metrics.RecordReconcileError(ctx, telemetry.StageProvision)
			break
		}
		log.Printf("pool: provisioning warm member %s (%d/%d warm, %d/%d total)",
			sess.Name, warm+1, p.cfg.TargetWarm, total+1, p.cfg.MaxTotal)
		p.metrics.RecordProvisioned(ctx)
		warm++
		total++
	}

	// 5) GC orphaned owner markers (crashed between marker create and member
	// claim). A marker is kept while its owner holds a live claim, is younger
	// than the grace window, or is still queued.
	markers, err := p.svc.listOwnerMarkers(ctx)
	if err != nil {
		return err
	}
	for i := range markers {
		m := markers[i]
		owner, _, _ := unstructured.NestedString(m.Object, "data", markerKeyOwner)
		if owner == "" || owners[owner] {
			continue
		}
		if now.Sub(m.GetCreationTimestamp().Time) < markerOrphanGrace {
			continue
		}
		if _, queued := p.queue.Position(owner); queued {
			continue
		}
		log.Printf("pool: removing orphaned owner marker %s", m.GetName())
		if err := p.svc.deleteOwnerMarker(ctx, owner); err != nil {
			log.Printf("pool: marker GC failed: %v", err)
			p.metrics.RecordReconcileError(ctx, telemetry.StageMarkerGC)
		} else {
			p.metrics.RecordMarkerOrphanGC(ctx)
		}
	}
	return nil
}
