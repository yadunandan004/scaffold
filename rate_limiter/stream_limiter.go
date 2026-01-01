package rate_limiter

import (
	"sync/atomic"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StreamRateLimiter provides rate limiting for gRPC streams
type StreamRateLimiter struct {
	msgsPerSecond rate.Limit
	burst         int

	// Metrics (optional but useful)
	totalStreams  atomic.Int64
	activeStreams atomic.Int64
	throttledMsgs atomic.Int64
}

// NewStreamRateLimiter creates a new rate limiter
func NewStreamRateLimiter(msgsPerSecond int, burst int) *StreamRateLimiter {
	return &StreamRateLimiter{
		msgsPerSecond: rate.Limit(msgsPerSecond),
		burst:         burst,
	}
}

// StreamInterceptor returns the gRPC stream interceptor
func (srl *StreamRateLimiter) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Create a rate limiter for this specific stream
		limiter := rate.NewLimiter(srl.msgsPerSecond, srl.burst)

		// Track metrics
		srl.totalStreams.Add(1)
		srl.activeStreams.Add(1)
		defer srl.activeStreams.Add(-1)

		// Wrap the stream
		wrapped := &rateLimitedStream{
			ServerStream:  ss,
			limiter:       limiter,
			throttledMsgs: &srl.throttledMsgs,
		}

		return handler(srv, wrapped)
	}
}

// rateLimitedStream wraps the original stream with rate limiting
type rateLimitedStream struct {
	grpc.ServerStream
	limiter       *rate.Limiter
	throttledMsgs *atomic.Int64
}

// RecvMsg receives and rate limits incoming messages
func (s *rateLimitedStream) RecvMsg(m interface{}) error {
	// Wait blocks until we're under rate limit
	// This provides backpressure to the client
	ctx := s.Context()

	// Use Wait for throttling - client experiences slowdown, not errors
	err := s.limiter.Wait(ctx)
	if err != nil {
		// Only fails if request is cancelled/deadline exceeded
		return status.Error(codes.Canceled, "stream cancelled during rate limiting")
	}

	// If we had to wait, increment throttled counter
	if s.limiter.Tokens() < 1 {
		s.throttledMsgs.Add(1)
	}

	return s.ServerStream.RecvMsg(m)
}

// SendMsg sends messages with rate limiting
func (s *rateLimitedStream) SendMsg(m interface{}) error {
	// Also rate limit outgoing messages
	ctx := s.Context()

	err := s.limiter.Wait(ctx)
	if err != nil {
		return status.Error(codes.Canceled, "stream cancelled during rate limiting")
	}

	return s.ServerStream.SendMsg(m)
}

// Metrics returns current metrics
func (srl *StreamRateLimiter) Metrics() StreamMetrics {
	return StreamMetrics{
		TotalStreams:  srl.totalStreams.Load(),
		ActiveStreams: srl.activeStreams.Load(),
		ThrottledMsgs: srl.throttledMsgs.Load(),
	}
}

// StreamMetrics contains rate limiting metrics
type StreamMetrics struct {
	TotalStreams  int64
	ActiveStreams int64
	ThrottledMsgs int64
}
