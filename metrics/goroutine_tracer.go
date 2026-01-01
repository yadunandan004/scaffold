package metrics

import (
	"context"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// TraceGoroutine wraps a goroutine with OpenTelemetry tracing and metrics
func TraceGoroutine(ctx context.Context, component, operation string, fn func()) {
	RecordGoroutineCreated(ctx, component, operation)

	tracer := Tracer()
	if tracer == nil {
		go func() {
			defer RecordGoroutineFinished(ctx, component, operation)
			fn()
		}()
		return
	}

	ctx, span := tracer.Start(ctx, "goroutine."+component+"."+operation)
	span.SetAttributes(
		attribute.String("goroutine.component", component),
		attribute.String("goroutine.operation", operation),
	)

	go func() {
		defer func() {
			span.End()
			RecordGoroutineFinished(ctx, component, operation)
		}()
		fn()
	}()
}

// TraceGoroutineWithReturn wraps a goroutine that returns an error
func TraceGoroutineWithReturn(ctx context.Context, component, operation string, fn func() error) error {
	RecordGoroutineCreated(ctx, component, operation)

	tracer := Tracer()
	if tracer == nil {
		errChan := make(chan error, 1)
		go func() {
			defer RecordGoroutineFinished(ctx, component, operation)
			errChan <- fn()
		}()
		return <-errChan
	}

	// Start tracing span
	ctx, span := tracer.Start(ctx, "goroutine."+component+"."+operation)
	span.SetAttributes(
		attribute.String("goroutine.component", component),
		attribute.String("goroutine.operation", operation),
	)

	errChan := make(chan error, 1)

	go func() {
		defer func() {
			span.End()
			// Record completion
			RecordGoroutineFinished(ctx, component, operation)
		}()

		err := fn()
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.Bool("error", true))
		}
		errChan <- err
	}()

	return <-errChan
}

// StartRuntimeGoroutineCollection starts periodic collection of runtime goroutine stats
func StartRuntimeGoroutineCollection(ctx context.Context, interval time.Duration) {
	if interval == 0 {
		interval = 30 * time.Second // Default to 30 seconds
	}

	TraceGoroutine(ctx, "runtime", "goroutine_collector", func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				collectRuntimeGoroutineStats(ctx)
			}
		}
	})
}

// collectRuntimeGoroutineStats collects current runtime goroutine count
func collectRuntimeGoroutineStats(ctx context.Context) {
	// Get current goroutine count from runtime
	currentCount := int64(runtime.NumGoroutine())

	// Record the runtime total (this will be a separate metric from our tracked components)
	attrs := []attribute.KeyValue{
		attribute.String("component", "runtime"),
		attribute.String("operation", "total"),
	}

	// We use Gauge for runtime total since it's an absolute count, not a delta
	if runtimeGoroutineGauge := createRuntimeGoroutineGauge(); runtimeGoroutineGauge != nil {
		runtimeGoroutineGauge.Record(ctx, currentCount, metric.WithAttributes(attrs...))
	}
}

// createRuntimeGoroutineGauge creates a gauge for runtime goroutine count
func createRuntimeGoroutineGauge() metric.Int64Gauge {
	meter := Meter()
	if meter == nil {
		return nil
	}

	gauge, err := meter.Int64Gauge(
		"goroutines_runtime_total",
		metric.WithDescription("Total number of goroutines reported by runtime.NumGoroutine()"),
	)
	if err != nil {
		return nil
	}

	return gauge
}
