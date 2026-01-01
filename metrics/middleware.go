package metrics

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// GinMiddleware creates a Gin middleware for OpenTelemetry metrics and tracing
func GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" || c.Request.URL.Path == "/health" {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		spanName := c.Request.Method + " " + c.FullPath()
		if spanName == "" {
			spanName = c.Request.Method + " " + c.Request.URL.Path
		}
		var span trace.Span
		if tracer != nil {
			ctx, span = tracer.Start(ctx, spanName,
				trace.WithAttributes(
					attribute.String("http.method", c.Request.Method),
					attribute.String("http.url", c.Request.URL.String()),
					attribute.String("http.scheme", c.Request.URL.Scheme),
					attribute.String("http.host", c.Request.Host),
					attribute.String("http.user_agent", c.Request.UserAgent()),
					attribute.String("http.remote_addr", c.ClientIP()),
				),
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()
		}
		c.Request = c.Request.WithContext(ctx)
		IncrementActiveRequests(ctx)
		defer DecrementActiveRequests(ctx)
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		statusCode := c.Writer.Status()
		if span != nil {
			span.SetAttributes(
				attribute.Int("http.status_code", statusCode),
				attribute.Int64("http.response_size", int64(c.Writer.Size())),
			)
			if spanCtx := span.SpanContext(); spanCtx.HasTraceID() {
				c.Header("X-Trace-ID", spanCtx.TraceID().String())
			}
		}
		RecordHTTPRequest(ctx, c.Request.Method, c.FullPath(), statusCode, duration)
	}
}
