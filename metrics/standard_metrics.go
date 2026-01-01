package metrics

import (
	"context"
	"sync"

	"github.com/yadunandan004/scaffold/metrics/providers"
)

type StandardMetrics struct {
	httpRequestDuration    providers.Histogram
	httpRequestCounter     providers.Counter
	httpActiveRequests     providers.UpDownCounter
	loginCounter           providers.Counter
	signupCounter          providers.Counter
	tokenRefreshCounter    providers.Counter
	dbQueryDuration        providers.Histogram
	dbConnectionPoolActive providers.Gauge
	goroutineCreated       providers.Counter
	goroutineFinished      providers.Counter
	cacheHitCounter        providers.Counter
	cacheMissCounter       providers.Counter
	workflowCounter        providers.Counter
	mu                     sync.RWMutex
}

var standardMetrics *StandardMetrics
var standardMetricsOnce sync.Once

func initStandardMetrics(registry *MetricRegistry) *StandardMetrics {
	standardMetricsOnce.Do(func() {
		standardMetrics = &StandardMetrics{
			httpRequestDuration: registry.MustRegisterHistogram(
				"http_request_duration_seconds",
				"Duration of HTTP requests in seconds",
				"s",
			),
			httpRequestCounter: registry.MustRegisterCounter(
				"http_requests_total",
				"Total number of HTTP requests",
				"1",
			),
			httpActiveRequests: registry.MustRegisterUpDownCounter(
				"http_active_requests",
				"Number of active HTTP requests",
				"1",
			),
			loginCounter: registry.MustRegisterCounter(
				"login_attempts_total",
				"Total number of login attempts",
				"1",
			),
			signupCounter: registry.MustRegisterCounter(
				"signup_attempts_total",
				"Total number of signup attempts",
				"1",
			),
			tokenRefreshCounter: registry.MustRegisterCounter(
				"token_refresh_attempts_total",
				"Total number of token refresh attempts",
				"1",
			),
			dbQueryDuration: registry.MustRegisterHistogram(
				"db_query_duration_seconds",
				"Duration of database queries in seconds",
				"s",
			),
			dbConnectionPoolActive: registry.MustRegisterGauge(
				"db_connection_pool_active",
				"Number of active database connections",
				"1",
			),
			goroutineCreated: registry.MustRegisterCounter(
				"goroutines_created_total",
				"Total number of goroutines created",
				"1",
			),
			goroutineFinished: registry.MustRegisterCounter(
				"goroutines_finished_total",
				"Total number of goroutines finished",
				"1",
			),
			cacheHitCounter: registry.MustRegisterCounter(
				"cache_hits_total",
				"Total number of cache hits",
				"1",
			),
			cacheMissCounter: registry.MustRegisterCounter(
				"cache_misses_total",
				"Total number of cache misses",
				"1",
			),
			workflowCounter: registry.MustRegisterCounter(
				"workflows_total",
				"Total number of workflows executed",
				"1",
			),
		}
	})
	return standardMetrics
}

func (sm *StandardMetrics) RecordHTTPRequest(ctx context.Context, method, path string, statusCode int, duration float64) {
	labels := providers.Labels(
		"method", method,
		"path", path,
		"status_code", itoa(statusCode),
	)
	sm.httpRequestDuration.Record(ctx, duration, labels...)
	sm.httpRequestCounter.Inc(ctx, labels...)
}

func itoa(i int) string {
	if i < 0 {
		return "-" + itoa(-i)
	}
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

func (sm *StandardMetrics) IncrementActiveRequests(ctx context.Context) {
	sm.httpActiveRequests.Inc(ctx)
}

func (sm *StandardMetrics) DecrementActiveRequests(ctx context.Context) {
	sm.httpActiveRequests.Dec(ctx)
}

func (sm *StandardMetrics) RecordLogin(ctx context.Context, success bool) {
	successStr := "false"
	if success {
		successStr = "true"
	}
	sm.loginCounter.Inc(ctx, providers.Labels("success", successStr)...)
}

func (sm *StandardMetrics) RecordSignup(ctx context.Context, success bool) {
	successStr := "false"
	if success {
		successStr = "true"
	}
	sm.signupCounter.Inc(ctx, providers.Labels("success", successStr)...)
}

func (sm *StandardMetrics) RecordTokenRefresh(ctx context.Context, success bool) {
	successStr := "false"
	if success {
		successStr = "true"
	}
	sm.tokenRefreshCounter.Inc(ctx, providers.Labels("success", successStr)...)
}

func (sm *StandardMetrics) RecordDBQuery(ctx context.Context, operation string, duration float64) {
	sm.dbQueryDuration.Record(ctx, duration, providers.Labels("operation", operation)...)
}

func (sm *StandardMetrics) RecordGoroutineCreated(ctx context.Context, component, operation string) {
	sm.goroutineCreated.Inc(ctx, providers.Labels("component", component, "operation", operation)...)
}

func (sm *StandardMetrics) RecordGoroutineFinished(ctx context.Context, component, operation string) {
	sm.goroutineFinished.Inc(ctx, providers.Labels("component", component, "operation", operation)...)
}

func (sm *StandardMetrics) RecordCacheHit(ctx context.Context, cacheType string) {
	sm.cacheHitCounter.Inc(ctx, providers.Labels("cache_type", cacheType)...)
}

func (sm *StandardMetrics) RecordCacheMiss(ctx context.Context, cacheType string) {
	sm.cacheMissCounter.Inc(ctx, providers.Labels("cache_type", cacheType)...)
}

func (sm *StandardMetrics) RecordWorkflow(ctx context.Context, workflowType, status string) {
	sm.workflowCounter.Inc(ctx, providers.Labels("workflow_type", workflowType, "status", status)...)
}
