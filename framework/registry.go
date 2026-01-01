package framework

import (
	"fmt"
	"github.com/yadunandan004/scaffold/auth"
	"github.com/yadunandan004/scaffold/rate_limiter"
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yadunandan004/scaffold/request"
)

type Registry struct {
	engine      *gin.Engine
	auth        *auth.AuthService
	rateLimiter *rate_limiter.HTTPRateLimiter
}

func NewRegistry(engine *gin.Engine, auth *auth.AuthService) *Registry {
	limiter := rate_limiter.NewHTTPRateLimiter(100, 200) // 100 req/s default, burst 200

	return &Registry{
		engine:      engine,
		auth:        auth,
		rateLimiter: limiter,
	}
}

func (r *Registry) AddGroup(group RouteGroup) {
	ginGroup := r.engine.Group(group.BasePath)
	for _, route := range group.RouteList {
		// Register route-specific rate limits if configured
		if route.RateLimitRPS > 0 && route.RateLimitBurst > 0 {
			pattern := group.BasePath + route.Path
			config := &rate_limiter.RouteConfig{
				RequestsPerSecond: route.RateLimitRPS,
				Burst:             route.RateLimitBurst,
			}
			r.rateLimiter.RegisterRoute(pattern, config)
		}

		// Create handler with rate limiting
		handler := r.createRouteHandler(route, group.BasePath)

		switch route.Method {
		case "GET":
			ginGroup.GET(route.Path, handler)
		case "POST":
			ginGroup.POST(route.Path, handler)
		case "PUT":
			ginGroup.PUT(route.Path, handler)
		case "DELETE":
			ginGroup.DELETE(route.Path, handler)
		case "PATCH":
			ginGroup.PATCH(route.Path, handler)
		case "HEAD":
			ginGroup.HEAD(route.Path, handler)
		case "OPTIONS":
			ginGroup.OPTIONS(route.Path, handler)
		}
	}
}

// createRouteHandler creates a Gin handler that wraps your custom handler
func (r *Registry) createRouteHandler(route Route, basePath string) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		// Apply rate limiting first (blocking/throttling)
		pattern := basePath + route.Path
		limiter := r.rateLimiter.GetLimiterForRoute(pattern, route.Method)

		// Block until rate limit allows (throttling approach)
		err := limiter.Wait(ginCtx.Request.Context())
		if err != nil {
			// Context cancelled while waiting
			ginCtx.JSON(503, gin.H{"error": "Service unavailable"})
			return
		}

		var opts []request.HttpCtxOption
		ctx := request.NewApiContextForHttp(ginCtx, opts...)

		// Check authentication
		if !route.ShouldSkipAuth && !r.checkAuth(ctx) {
			ctx.JSON(401, gin.H{"error": "Unauthorized"})
			return
		}

		// Start transaction if not skipped (OPTIONS always skips transaction)
		if !route.ShouldSkipTxn && route.Method != "OPTIONS" {
			// Use BeginTransactionForModel with a generic type
			tx, err := request.BeginTransaction(ctx)
			if err != nil {
				log.Printf("[Registry] Failed to start transaction: %v", err)
				ctx.JSON(500, gin.H{"error": fmt.Sprintf("Failed to start transaction: %v", err)})
				return
			}
			defer func() {
				// Check if response was successful
				if ginCtx.Writer.Status() >= 200 && ginCtx.Writer.Status() < 400 {
					tx.Commit()
				} else {
					tx.Rollback()
				}
			}()
		}

		// Handle the request
		route.Handler(ctx)
	}
}

func (r *Registry) checkAuth(ctx request.Context) bool {
	ginCtx := ctx.GetGinContext()
	if ginCtx == nil {
		return false
	}

	// Check if auth middleware has already been run
	claimsRaw, exists := ginCtx.Get("user_claims")
	if !exists {
		// Run auth middleware
		r.auth.HTTPMiddleware()(ginCtx)
		if ginCtx.IsAborted() {
			return false
		}
		claimsRaw, exists = ginCtx.Get("user_claims")
		if !exists {
			return false
		}
	}

	// Extract user claims and set user info
	if claims, ok := claimsRaw.(*auth.UserClaims); ok {
		ginCtx.Set(string(request.UserIDKey), claims.UserID)
		ginCtx.Set(string(request.UserEmailKey), claims.Email)
		ginCtx.Set("client_device_id", claims.ClientDeviceID)

		// Update the request's user info
		if httpCtx, ok := ctx.(*request.HttpCtx); ok {
			httpCtx.SetUserInfo(claims.UserID, claims.Email)
			// Note: ClientDeviceID is already set via gin request above
		}
		return true
	}
	return false
}

// containsSearchPath checks if the path contains /search
func containsSearchPath(path string) bool {
	return strings.Contains(path, "/search")
}
