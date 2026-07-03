// Package telemetry bootstraps the OpenTelemetry metrics SDK and owns every
// instrument the backend records (docs/reference/observability-architecture.md).
//
// Design: metrics are pushed OTLP/HTTP to the node-local otel-collector agent
// ($(HOST_IP):4318), whose metrics pipeline remote-writes to Mimir. The app
// never exposes a /metrics endpoint. All wiring is env-driven so the Helm
// chart owns it:
//
//	OTEL_EXPORTER_OTLP_ENDPOINT   http://$(HOST_IP):4318 (unset => telemetry off)
//	OTEL_EXPORTER_OTLP_PROTOCOL   http/protobuf
//	OTEL_SERVICE_NAME             kubesandbox-backend
//	OTEL_RESOURCE_ATTRIBUTES      service.namespace=...,service.version=...
//	OTEL_METRIC_EXPORT_INTERVAL   15000 (ms; read by the periodic reader)
//	OTEL_SDK_DISABLED             "true" disables everything (no-op Metrics)
//
// NOTE: the Go SDK does not implement OTEL_SDK_DISABLED itself (unlike some
// other language SDKs), so Setup honours it explicitly. Setup also no-ops when
// no OTLP endpoint is configured, so local dev and tests never spam export
// errors. A nil *Metrics is valid: every method is nil-safe.
package telemetry

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// meterName scopes the backend's custom instruments.
const meterName = "kubesandbox-backend"

// Histogram buckets (seconds), tuned to the SLOs in the architecture doc §5.
var (
	httpBuckets      = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5}
	provisionBuckets = []float64{1, 2, 5, 10, 20, 30, 60, 120}
	queueWaitBuckets = []float64{.5, 1, 2, 5, 10, 30, 60, 120}
	reconcileBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5}
)

// Setup builds the MeterProvider (OTLP/HTTP exporter + periodic reader),
// installs it globally (otelgin and the runtime instrumentation read the
// global), starts runtime metrics, and returns the backend's instrument set
// plus a shutdown func that flushes on exit.
//
// It returns (nil, no-op, nil) — telemetry disabled — when OTEL_SDK_DISABLED
// is truthy or no OTLP endpoint is configured.
func Setup(ctx context.Context) (*Metrics, func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }

	if disabled, _ := strconv.ParseBool(os.Getenv("OTEL_SDK_DISABLED")); disabled {
		return nil, noop, nil
	}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" &&
		os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") == "" {
		return nil, noop, nil
	}

	// Endpoint, protocol and headers all come from the standard OTEL_* env.
	exporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, noop, fmt.Errorf("otlp metric exporter: %w", err)
	}

	// service.name defaults to the meter name; OTEL_SERVICE_NAME (set by the
	// chart) and OTEL_RESOURCE_ATTRIBUTES override via WithFromEnv (later
	// options take precedence in resource.New).
	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", meterName)),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, noop, fmt.Errorf("otel resource: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		// Cumulative temporality (the SDK default) is required by the
		// prometheusremotewrite exporter in the collector — do not switch to
		// delta. Export interval comes from OTEL_METRIC_EXPORT_INTERVAL.
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		// otelgin's histogram is created internally; shape its buckets here.
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{Name: "http.server.request.duration"},
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: httpBuckets,
			}},
		)),
	)
	otel.SetMeterProvider(provider)

	// Go runtime metrics (go.memory.used, go.goroutine.count, ...).
	if err := runtime.Start(runtime.WithMeterProvider(provider)); err != nil {
		_ = provider.Shutdown(ctx)
		return nil, noop, fmt.Errorf("runtime metrics: %w", err)
	}

	m, err := newMetrics(provider.Meter(meterName))
	if err != nil {
		_ = provider.Shutdown(ctx)
		return nil, noop, fmt.Errorf("create instruments: %w", err)
	}
	return m, provider.Shutdown, nil
}
