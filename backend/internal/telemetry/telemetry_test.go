package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestSetupDisabled: no endpoint / explicit disable => nil Metrics (valid
// no-op receiver) and a working shutdown func.
func TestSetupDisabled(t *testing.T) {
	for _, tc := range []struct{ name, key, val string }{
		{"no endpoint", "", ""},
		{"sdk disabled", "OTEL_SDK_DISABLED", "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.key != "" {
				t.Setenv(tc.key, tc.val)
				t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
			}
			m, shutdown, err := Setup(context.Background())
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}
			if m != nil {
				t.Fatalf("expected nil Metrics when disabled")
			}
			// Every method must be nil-safe.
			m.RecordProvisioned(context.Background())
			m.RecordClaimed(context.Background(), SourceRequest)
			m.RecordAssignAttempt(context.Background(), ResultSuccess)
			m.RecordResolved(context.Background(), OutcomeAssigned, time.Second)
			m.RecordReconcile(context.Background(), time.Millisecond, nil)
			m.AddSSEStream(context.Background(), KindSession, 1)
			m.SetPoolState(1, 2, 3, 4)
			m.SetPoolConfig(2, 60)
			m.RegisterQueueDepth(func() int64 { return 0 })
			if err := shutdown(context.Background()); err != nil {
				t.Fatalf("shutdown: %v", err)
			}
		})
	}
}

// TestSetupExportsOTLP: with an endpoint configured, Setup builds a working
// pipeline that pushes OTLP metrics over HTTP on shutdown (flush).
func TestSetupExportsOTLP(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/metrics" {
			hits.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", srv.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_SERVICE_NAME", "kubesandbox-backend-test")
	t.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "60000") // rely on the flush, not the timer

	m, shutdown, err := Setup(context.Background())
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil Metrics")
	}

	ctx := context.Background()
	m.SetPoolConfig(2, 60)
	m.SetPoolState(2, 1, 5, 8)
	m.RegisterQueueDepth(func() int64 { return 3 })
	m.RecordProvisioned(ctx)
	m.RecordClaimed(ctx, SourceQueue)
	m.RecordRecycled(ctx, ReasonStale)
	m.RecordExpired(ctx, 2)
	m.RecordAssignAttempt(ctx, ResultConflictRetry)
	m.RecordMarkerOrphanGC(ctx)
	m.RecordEnqueued(ctx)
	m.RecordResolved(ctx, OutcomeAssigned, 1500*time.Millisecond)
	m.RecordProvisionDuration(ctx, 12*time.Second)
	m.RecordReconcile(ctx, 40*time.Millisecond, nil)
	m.AddSSEStream(ctx, KindQueue, 1)

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if hits.Load() == 0 {
		t.Fatal("no OTLP metrics POST received on flush")
	}
}
