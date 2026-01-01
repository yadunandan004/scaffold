package rate_limiter

import (
	"context"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryRateLimiter provides rate limiting for unary RPCs
type UnaryRateLimiter struct {
	limiter *rate.Limiter
}

// NewUnaryRateLimiter creates a limiter for regular RPCs
func NewUnaryRateLimiter(rpcPerSecond int, burst int) *UnaryRateLimiter {
	return &UnaryRateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rpcPerSecond), burst),
	}
}

// UnaryInterceptor returns the gRPC unary interceptor
func (url *UnaryRateLimiter) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Wait for rate limit - provides throttling
		err := url.limiter.Wait(ctx)
		if err != nil {
			// Context cancelled while waiting
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}

		return handler(ctx, req)
	}
}
