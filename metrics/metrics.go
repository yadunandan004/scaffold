package metrics

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/yadunandan004/scaffold/metrics/providers"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	globalRegistry   *MetricRegistry
	globalStdMetrics *StandardMetrics
	meter            metric.Meter
	tracer           trace.Tracer
	initOnce         sync.Once
)

type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
}

func InitMetrics(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	var err error
	var provider *providers.PrometheusProvider

	initOnce.Do(func() {
		provider, err = providers.NewPrometheusProvider(ctx, providers.PrometheusConfig{
			ServiceName:    cfg.ServiceName,
			ServiceVersion: cfg.ServiceVersion,
			Environment:    cfg.Environment,
		})
		if err != nil {
			return
		}

		globalRegistry = NewMetricRegistry(provider)
		globalStdMetrics = initStandardMetrics(globalRegistry)

		meter = otel.Meter(cfg.ServiceName)
		tracer = otel.Tracer(cfg.ServiceName)
	})

	if err != nil {
		return nil, err
	}

	StartRuntimeGoroutineCollection(ctx, 30*time.Second)

	return globalRegistry.Shutdown, nil
}

func GetRegistry() *MetricRegistry {
	return globalRegistry
}

func Handler() http.Handler {
	if globalRegistry != nil {
		return globalRegistry.HTTPHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Metrics not initialized"))
	})
}

func Meter() metric.Meter {
	return meter
}

func Tracer() trace.Tracer {
	return tracer
}

func RecordHTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.RecordHTTPRequest(ctx, method, path, statusCode, duration.Seconds())
}

func IncrementActiveRequests(ctx context.Context) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.IncrementActiveRequests(ctx)
}

func DecrementActiveRequests(ctx context.Context) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.DecrementActiveRequests(ctx)
}

func RecordLogin(ctx context.Context, success bool) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.RecordLogin(ctx, success)
}

func RecordSignup(ctx context.Context, success bool) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.RecordSignup(ctx, success)
}

func RecordTokenRefresh(ctx context.Context, success bool) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.RecordTokenRefresh(ctx, success)
}

func RecordDBQuery(ctx context.Context, operation string, duration time.Duration) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.RecordDBQuery(ctx, operation, duration.Seconds())
}

func RecordGoroutineCreated(ctx context.Context, component, operation string) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.RecordGoroutineCreated(ctx, component, operation)
}

func RecordGoroutineFinished(ctx context.Context, component, operation string) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.RecordGoroutineFinished(ctx, component, operation)
}

func RecordCacheHit(ctx context.Context, cacheType string) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.RecordCacheHit(ctx, cacheType)
}

func RecordCacheMiss(ctx context.Context, cacheType string) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.RecordCacheMiss(ctx, cacheType)
}

func RecordWorkflow(ctx context.Context, workflowType, status string) {
	if globalStdMetrics == nil {
		return
	}
	globalStdMetrics.RecordWorkflow(ctx, workflowType, status)
}
