package rate_limiter

import (
	"context"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// RouteConfig holds rate limit configuration for a specific route
type RouteConfig struct {
	RequestsPerSecond int
	Burst             int
	// Optional: different limits for different methods
	PerMethod map[string]*MethodLimit
}

// MethodLimit holds rate limit for a specific HTTP method
type MethodLimit struct {
	RequestsPerSecond int
	Burst             int
}

// HTTPRateLimiter provides per-route rate limiting
type HTTPRateLimiter struct {
	mu            sync.RWMutex
	routeConfigs  map[string]*RouteConfig  // pattern -> config
	routeLimiters map[string]*rate.Limiter // pattern -> limiter

	// Default limits if no route config
	defaultRPS   int
	defaultBurst int
}

// NewHTTPRateLimiter creates a new rate limiter
func NewHTTPRateLimiter(defaultRPS, defaultBurst int) *HTTPRateLimiter {
	return &HTTPRateLimiter{
		routeConfigs:  make(map[string]*RouteConfig),
		routeLimiters: make(map[string]*rate.Limiter),
		defaultRPS:    defaultRPS,
		defaultBurst:  defaultBurst,
	}
}

// RegisterRoute registers rate limit config for a route
func (h *HTTPRateLimiter) RegisterRoute(pattern string, config *RouteConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.routeConfigs[pattern] = config
	h.routeLimiters[pattern] = rate.NewLimiter(
		rate.Limit(config.RequestsPerSecond),
		config.Burst,
	)
}

// Middleware returns the HTTP middleware function
func (h *HTTPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the route pattern from request (set by your router)
		pattern := getRoutePattern(r)

		// Get limiter for this route
		limiter := h.GetLimiterForRoute(pattern, r.Method)

		// Wait for rate limit (blocking/throttling approach)
		err := limiter.Wait(r.Context())
		if err != nil {
			// Context cancelled while waiting
			http.Error(w, "Request cancelled", http.StatusServiceUnavailable)
			return
		}

		// Add rate limit info to request for handlers to use
		ctx := context.WithValue(r.Context(), rateLimitContextKey{}, &RateLimitInfo{
			Limit:     limiter.Limit(),
			Burst:     limiter.Burst(),
			Available: limiter.Tokens(),
		})

		// Call next handler with updated request
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetLimiterForRoute gets or creates a limiter for a route
func (h *HTTPRateLimiter) GetLimiterForRoute(pattern, method string) *rate.Limiter {
	h.mu.RLock()

	// Check for route-specific config
	if config, exists := h.routeConfigs[pattern]; exists {
		// Check for method-specific limits
		if config.PerMethod != nil {
			if methodLimit, ok := config.PerMethod[method]; ok {
				h.mu.RUnlock()
				return rate.NewLimiter(
					rate.Limit(methodLimit.RequestsPerSecond),
					methodLimit.Burst,
				)
			}
		}

		// Use route limiter
		if limiter, ok := h.routeLimiters[pattern]; ok {
			h.mu.RUnlock()
			return limiter
		}
	}
	h.mu.RUnlock()

	// Return default limiter
	return rate.NewLimiter(rate.Limit(h.defaultRPS), h.defaultBurst)
}

type rateLimitContextKey struct{}

// RateLimitInfo stored in request
type RateLimitInfo struct {
	Limit     rate.Limit
	Burst     int
	Available float64
}

// GetRateLimitInfo retrieves rate limit info from request
func GetRateLimitInfo(ctx context.Context) *RateLimitInfo {
	if info, ok := ctx.Value(rateLimitContextKey{}).(*RateLimitInfo); ok {
		return info
	}
	return nil
}

type routePatternKey struct{}

// SetRoutePattern adds route pattern to request
func SetRoutePattern(ctx context.Context, pattern string) context.Context {
	return context.WithValue(ctx, routePatternKey{}, pattern)
}

func getRoutePattern(r *http.Request) string {
	if pattern, ok := r.Context().Value(routePatternKey{}).(string); ok {
		return pattern
	}
	return r.URL.Path
}
