package telemetry

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Attribute keys/values used on the custom instruments. Every label set is
// small and static — never put ownerRef/email/session-id on a metric label
// (unbounded cardinality; per-user investigation belongs in traces/logs).
const (
	// SourceRequest / SourceQueue label sandbox.claimed_total.
	SourceRequest = "request"
	SourceQueue   = "queue"

	// ReasonStale / ReasonTrim label sandbox.recycled_total.
	ReasonStale = "stale"
	ReasonTrim  = "trim"

	// Assign outcomes for assign.attempts_total.
	ResultSuccess       = "success"
	ResultPoolEmpty     = "pool_empty"
	ResultAlreadyExists = "already_exists"
	ResultConflictRetry = "conflict_retry"
	ResultError         = "error"

	// Queue exits for queue.resolved_total.
	OutcomeAssigned = "assigned"
	OutcomeError    = "error"

	// Reconcile-failure stages for pool.reconcile.errors. StageReconcile is a
	// whole-pass failure (the LIST that drives the pass failed); the others are
	// per-item failures within an otherwise-successful pass that would
	// previously have been logged and swallowed.
	StageReconcile = "reconcile"
	StageAdmit     = "admit"
	StageProvision = "provision"
	StageRecycle   = "recycle"
	StageTrim      = "trim"
	StageMarkerGC  = "marker_gc"

	// SSE stream kinds for sse.active_streams.
	KindSession = "session"
	KindQueue   = "queue"

	// Seed outcomes for challenge.seed.attempts (design §12): success is a
	// completed seed, retry is a failed in-place attempt, recycled is a
	// member replaced after exhausting retries, failed is the terminal
	// fail-closed outcome.
	SeedResultSuccess  = "success"
	SeedResultRetry    = "retry"
	SeedResultRecycled = "recycled"
	SeedResultFailed   = "failed"
)

// Metrics is the backend's instrument set, shared by the HTTP handlers, the
// session service, the pool manager, the queue and the TTL controller. It is
// injected (not a package global) so tests stay hermetic; a nil *Metrics is a
// valid no-op receiver on every method.
//
// Pool-state gauges are asynchronous: reconcileOnce (the level-based source of
// truth) stores the values it already computes into atomics, and the SDK
// callback reads them at export time — no double counting, no drift.
type Metrics struct {
	sandboxProvisioned metric.Int64Counter
	sandboxClaimed     metric.Int64Counter
	sandboxRecycled    metric.Int64Counter
	sandboxExpired     metric.Int64Counter
	assignAttempts     metric.Int64Counter
	markerOrphanGC     metric.Int64Counter
	queueEnqueued      metric.Int64Counter
	queueResolved      metric.Int64Counter
	reconcileErrors    metric.Int64Counter

	provisionDuration metric.Float64Histogram
	queueWaitDuration metric.Float64Histogram
	reconcileDuration metric.Float64Histogram

	sseActiveStreams metric.Int64UpDownCounter

	// --- Guided challenges (design §12) ---
	seedDuration       metric.Float64Histogram
	seedAttempts       metric.Int64Counter
	gradeRequests      metric.Int64Counter
	tenantClientErrors metric.Int64Counter

	// contentInvalidFn feeds the content.bundle.invalid gauge: the content
	// store registers a snapshot of its quarantine set (bundle ConfigMap name
	// -> 1), read at export time like the queue-depth gauge.
	contentInvalidFn atomic.Value // func() map[string]int64

	poolAvailable atomic.Int64
	poolPending   atomic.Int64
	poolClaimed   atomic.Int64
	poolTotal     atomic.Int64
	poolTarget    atomic.Int64
	poolCapacity  atomic.Int64

	queueDepthFn atomic.Value // func() int64
}

// newMetrics creates every instrument on meter.
func newMetrics(meter metric.Meter) (*Metrics, error) {
	m := &Metrics{}
	var err error

	if m.sandboxProvisioned, err = meter.Int64Counter("kubesandbox.sandbox.provisioned",
		metric.WithDescription("Warm pool members created")); err != nil {
		return nil, err
	}
	if m.sandboxClaimed, err = meter.Int64Counter("kubesandbox.sandbox.claimed",
		metric.WithDescription("Successful sandbox hand-outs, by source (request|queue)")); err != nil {
		return nil, err
	}
	if m.sandboxRecycled, err = meter.Int64Counter("kubesandbox.sandbox.recycled",
		metric.WithDescription("Pool members deleted by the manager, by reason (stale|trim)")); err != nil {
		return nil, err
	}
	if m.sandboxExpired, err = meter.Int64Counter("kubesandbox.sandbox.expired",
		metric.WithDescription("Sessions reaped by the TTL controller")); err != nil {
		return nil, err
	}
	// NB: this counts ATTEMPTS, not requests. A single Assign call records one
	// terminal result (success|pool_empty|already_exists|error) plus one
	// conflict_retry per optimistic-concurrency retry, so the total exceeds the
	// number of Assign calls. rate(...{result="success"}) tracks the claim rate;
	// never treat sum(rate(...)) as a request rate — use the HTTP metrics for that.
	if m.assignAttempts, err = meter.Int64Counter("kubesandbox.assign.attempts",
		metric.WithDescription("Assignment attempts by outcome; conflict_retry is non-terminal (success|pool_empty|already_exists|conflict_retry|error)")); err != nil {
		return nil, err
	}
	if m.markerOrphanGC, err = meter.Int64Counter("kubesandbox.marker.orphan_gc",
		metric.WithDescription("Orphaned owner markers reaped")); err != nil {
		return nil, err
	}
	if m.queueEnqueued, err = meter.Int64Counter("kubesandbox.queue.enqueued",
		metric.WithDescription("Requests queued because the pool was empty")); err != nil {
		return nil, err
	}
	if m.queueResolved, err = meter.Int64Counter("kubesandbox.queue.resolved",
		metric.WithDescription("Queue exits, by outcome (assigned|error)")); err != nil {
		return nil, err
	}
	if m.reconcileErrors, err = meter.Int64Counter("kubesandbox.pool.reconcile.errors",
		metric.WithDescription("Pool reconcile failures, by stage (reconcile|provision|recycle|trim|marker_gc)")); err != nil {
		return nil, err
	}

	if m.provisionDuration, err = meter.Float64Histogram("kubesandbox.sandbox.provision.duration",
		metric.WithDescription("Warm create to workspaceReady"), metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(provisionBuckets...)); err != nil {
		return nil, err
	}
	if m.queueWaitDuration, err = meter.Float64Histogram("kubesandbox.queue.wait.duration",
		metric.WithDescription("Enqueue to terminal queue event"), metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(queueWaitBuckets...)); err != nil {
		return nil, err
	}
	if m.reconcileDuration, err = meter.Float64Histogram("kubesandbox.pool.reconcile.duration",
		metric.WithDescription("One pool reconcile pass"), metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(reconcileBuckets...)); err != nil {
		return nil, err
	}

	if m.sseActiveStreams, err = meter.Int64UpDownCounter("kubesandbox.sse.active_streams",
		metric.WithDescription("Open SSE connections, by kind (session|queue)")); err != nil {
		return nil, err
	}

	// --- Guided challenges (design §12) ---
	if m.seedDuration, err = meter.Float64Histogram("kubesandbox.challenge.seed.duration",
		metric.WithDescription("One successful seed apply, by challenge"), metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(seedBuckets...)); err != nil {
		return nil, err
	}
	if m.seedAttempts, err = meter.Int64Counter("kubesandbox.challenge.seed.attempts",
		metric.WithDescription("Seed attempts by result (success|retry|recycled|failed)")); err != nil {
		return nil, err
	}
	if m.gradeRequests, err = meter.Int64Counter("kubesandbox.challenge.grade.requests",
		metric.WithDescription("Grade requests by challenge and pass")); err != nil {
		return nil, err
	}
	if m.tenantClientErrors, err = meter.Int64Counter("kubesandbox.challenge.tenant_client.errors",
		metric.WithDescription("Failures building or using tenant vcluster clients")); err != nil {
		return nil, err
	}
	contentInvalid, err := meter.Int64ObservableGauge("kubesandbox.challenge.content.bundle_invalid",
		metric.WithDescription("Quarantined (invalid) content bundles, by bundle ConfigMap"))
	if err != nil {
		return nil, err
	}
	if _, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		if fn, ok := m.contentInvalidFn.Load().(func() map[string]int64); ok && fn != nil {
			for bundle, v := range fn() {
				o.ObserveInt64(contentInvalid, v, metric.WithAttributes(attribute.String("bundle", bundle)))
			}
		}
		return nil
	}, contentInvalid); err != nil {
		return nil, err
	}

	// Asynchronous pool-state gauges, read from the atomics at export time.
	gauges := make(map[*atomic.Int64]metric.Int64ObservableGauge, 6)
	for _, g := range []struct {
		name, desc string
		val        *atomic.Int64
	}{
		{"kubesandbox.pool.warm.available", "Ready, unclaimed pool members", &m.poolAvailable},
		{"kubesandbox.pool.warm.pending", "Provisioned but not yet Ready members", &m.poolPending},
		{"kubesandbox.pool.claimed", "Live claimed sessions", &m.poolClaimed},
		{"kubesandbox.pool.total", "All managed claims (occupying capacity)", &m.poolTotal},
		{"kubesandbox.pool.target", "Configured warm target", &m.poolTarget},
		{"kubesandbox.pool.capacity.max", "MaxTotal concurrent-session ceiling", &m.poolCapacity},
	} {
		og, err := meter.Int64ObservableGauge(g.name, metric.WithDescription(g.desc))
		if err != nil {
			return nil, err
		}
		gauges[g.val] = og
	}
	queueDepth, err := meter.Int64ObservableGauge("kubesandbox.queue.depth",
		metric.WithDescription("Waiters in the assignment FIFO"))
	if err != nil {
		return nil, err
	}

	observables := make([]metric.Observable, 0, len(gauges)+1)
	for _, og := range gauges {
		observables = append(observables, og)
	}
	observables = append(observables, queueDepth)

	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		for val, og := range gauges {
			o.ObserveInt64(og, val.Load())
		}
		if fn, ok := m.queueDepthFn.Load().(func() int64); ok && fn != nil {
			o.ObserveInt64(queueDepth, fn())
		}
		return nil
	}, observables...)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// --- recording helpers (all nil-safe) ---

// RecordProvisioned counts one warm member created.
func (m *Metrics) RecordProvisioned(ctx context.Context) {
	if m == nil {
		return
	}
	m.sandboxProvisioned.Add(ctx, 1)
}

// RecordClaimed counts one successful hand-out from source (request|queue).
func (m *Metrics) RecordClaimed(ctx context.Context, source string) {
	if m == nil {
		return
	}
	m.sandboxClaimed.Add(ctx, 1, metric.WithAttributes(attribute.String("source", source)))
}

// RecordRecycled counts one member deleted by the manager for reason (stale|trim).
func (m *Metrics) RecordRecycled(ctx context.Context, reason string) {
	if m == nil {
		return
	}
	m.sandboxRecycled.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
}

// RecordExpired counts n TTL-reaped sessions.
func (m *Metrics) RecordExpired(ctx context.Context, n int64) {
	if m == nil || n <= 0 {
		return
	}
	m.sandboxExpired.Add(ctx, n)
}

// RecordAssignAttempt counts one assignment outcome.
func (m *Metrics) RecordAssignAttempt(ctx context.Context, result string) {
	if m == nil {
		return
	}
	m.assignAttempts.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

// RecordMarkerOrphanGC counts one orphaned owner marker reaped.
func (m *Metrics) RecordMarkerOrphanGC(ctx context.Context) {
	if m == nil {
		return
	}
	m.markerOrphanGC.Add(ctx, 1)
}

// RecordEnqueued counts one request entering the queue.
func (m *Metrics) RecordEnqueued(ctx context.Context) {
	if m == nil {
		return
	}
	m.queueEnqueued.Add(ctx, 1)
}

// RecordResolved counts one queue exit and records the enqueue->terminal wait.
func (m *Metrics) RecordResolved(ctx context.Context, outcome string, wait time.Duration) {
	if m == nil {
		return
	}
	m.queueResolved.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
	if wait > 0 {
		m.queueWaitDuration.Record(ctx, wait.Seconds())
	}
}

// RecordProvisionDuration records warm create -> first observed workspaceReady.
func (m *Metrics) RecordProvisionDuration(ctx context.Context, d time.Duration) {
	if m == nil || d <= 0 {
		return
	}
	m.provisionDuration.Record(ctx, d.Seconds())
}

// RecordReconcile records the duration of one reconcile pass and counts a
// whole-pass failure (stage=reconcile) when err is non-nil.
func (m *Metrics) RecordReconcile(ctx context.Context, d time.Duration, err error) {
	if m == nil {
		return
	}
	m.reconcileDuration.Record(ctx, d.Seconds())
	if err != nil {
		m.RecordReconcileError(ctx, StageReconcile)
	}
}

// RecordReconcileError counts one reconcile failure at stage (provision|recycle|
// trim|marker_gc|reconcile). Per-item failures inside a pass are otherwise only
// logged, so without this the pass looks healthy while individual operations
// fail. Kept out of RecordReconcile so item failures don't need a pass error.
func (m *Metrics) RecordReconcileError(ctx context.Context, stage string) {
	if m == nil {
		return
	}
	m.reconcileErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("stage", stage)))
}

// AddSSEStream tracks an SSE connection of kind (session|queue) opening
// (delta=+1) or closing (delta=-1).
func (m *Metrics) AddSSEStream(ctx context.Context, kind string, delta int64) {
	if m == nil {
		return
	}
	m.sseActiveStreams.Add(ctx, delta, metric.WithAttributes(attribute.String("kind", kind)))
}

// SetPoolState publishes the pool-state gauges computed by reconcileOnce.
func (m *Metrics) SetPoolState(available, pending, claimed, total int64) {
	if m == nil {
		return
	}
	m.poolAvailable.Store(available)
	m.poolPending.Store(pending)
	m.poolClaimed.Store(claimed)
	m.poolTotal.Store(total)
}

// SetPoolConfig publishes the configured warm target and capacity ceiling.
func (m *Metrics) SetPoolConfig(target, capacity int64) {
	if m == nil {
		return
	}
	m.poolTarget.Store(target)
	m.poolCapacity.Store(capacity)
}

// RegisterQueueDepth wires the queue.depth gauge to fn (typically queue.Len).
func (m *Metrics) RegisterQueueDepth(fn func() int64) {
	if m == nil || fn == nil {
		return
	}
	m.queueDepthFn.Store(fn)
}

// --- Guided challenges (design §12), all nil-safe ---

// RecordSeedDuration records one successful end-to-end seed apply. The
// challenge label is bounded by catalog size, not user count.
func (m *Metrics) RecordSeedDuration(ctx context.Context, challenge string, d time.Duration) {
	if m == nil || d <= 0 {
		return
	}
	m.seedDuration.Record(ctx, d.Seconds(), metric.WithAttributes(attribute.String("challenge", challenge)))
}

// RecordSeedAttempt counts one seed outcome (success|retry|recycled|failed).
// Alert candidate: any result="failed" (design §12).
func (m *Metrics) RecordSeedAttempt(ctx context.Context, result string) {
	if m == nil {
		return
	}
	m.seedAttempts.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

// RecordGrade counts one grade request by challenge and outcome.
func (m *Metrics) RecordGrade(ctx context.Context, challenge string, pass bool) {
	if m == nil {
		return
	}
	m.gradeRequests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("challenge", challenge), attribute.Bool("pass", pass)))
}

// RecordTenantClientError counts one tenant-client build/use failure.
func (m *Metrics) RecordTenantClientError(ctx context.Context) {
	if m == nil {
		return
	}
	m.tenantClientErrors.Add(ctx, 1)
}

// RegisterContentInvalid wires the content bundle_invalid gauge to fn (a
// snapshot of the content store's quarantine set).
func (m *Metrics) RegisterContentInvalid(fn func() map[string]int64) {
	if m == nil || fn == nil {
		return
	}
	m.contentInvalidFn.Store(fn)
}
