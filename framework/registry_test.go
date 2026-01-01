package framework

import (
	"github.com/yadunandan004/scaffold/request"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRegistryWithContext(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registry := NewRegistry(engine, nil)

	// Track if handler was called
	handlerCalled := false
	var receivedCtx request.Context

	// Add a test route
	registry.AddGroup(RouteGroup{
		Name:     "test",
		BasePath: "/api",
		RouteList: []Route{
			{
				Method: "POST",
				Path:   "/test/:id",
				Handler: func(ctx request.Context) {
					handlerCalled = true
					receivedCtx = ctx
					// Use Context methods
					ctx.JSON(200, gin.H{"message": "success"})
				},
				ShouldSkipAuth: true,
				ShouldSkipTxn:  false, // Should create transaction
			},
		},
	})

	// Create request and execute using the Gin engine
	req, _ := http.NewRequest("POST", "/api/test/123", nil)
	w := httptest.NewRecorder()

	// ExecuteTemplate request through Gin engine
	engine.ServeHTTP(w, req)

	// Assert
	assert.True(t, handlerCalled)
	assert.NotNil(t, receivedCtx)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestRegistrySkipTransaction(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registry := NewRegistry(engine, nil)

	// Track transaction creation
	var hadTransaction bool

	// Add a test route with skip transaction
	registry.AddGroup(RouteGroup{
		Name:     "test",
		BasePath: "/api",
		RouteList: []Route{
			{
				Method: "GET",
				Path:   "/test",
				Handler: func(ctx request.Context) {
					// Check if transaction exists
					// Transaction is now checked via orm.GetTransaction
					hadTransaction = false
					ctx.JSON(200, gin.H{"message": "success"})
				},
				ShouldSkipAuth: true,
				ShouldSkipTxn:  true,
			},
		},
	})

	// Create request and execute using the Gin engine
	req, _ := http.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	// ExecuteTemplate request through Gin engine
	engine.ServeHTTP(w, req)

	// Assert
	assert.False(t, hadTransaction, "GET request with ShouldSkipTxn=true should not have transaction")
	assert.Equal(t, 200, w.Code)
}

func TestRegistryRouteMatching(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registry := NewRegistry(engine, nil)

	// Track which handler was called
	var calledRoute string

	// Add multiple test routes
	registry.AddGroup(RouteGroup{
		Name:     "users",
		BasePath: "/api/users",
		RouteList: []Route{
			{
				Method: "GET",
				Path:   "/:id",
				Handler: func(ctx request.Context) {
					calledRoute = "get-user"
					id := ctx.GetRequestContext().Param("id")
					ctx.JSON(200, gin.H{"route": "get-user", "id": id})
				},
				ShouldSkipAuth: true,
				ShouldSkipTxn:  true,
			},
			{
				Method: "POST",
				Path:   "",
				Handler: func(ctx request.Context) {
					calledRoute = "create-user"
					ctx.JSON(200, gin.H{"route": "create-user"})
				},
				ShouldSkipAuth: true,
				ShouldSkipTxn:  true,
			},
			{
				Method: "PUT",
				Path:   "/:id",
				Handler: func(ctx request.Context) {
					calledRoute = "update-user"
					ctx.JSON(200, gin.H{"route": "update-user"})
				},
				ShouldSkipAuth: true,
				ShouldSkipTxn:  true,
			},
		},
	})

	// Test GET /api/users/123
	req1, _ := http.NewRequest("GET", "/api/users/123", nil)
	w1 := httptest.NewRecorder()
	engine.ServeHTTP(w1, req1)
	assert.Equal(t, "get-user", calledRoute)
	assert.Equal(t, 200, w1.Code)
	assert.Contains(t, w1.Body.String(), `"id":"123"`)

	// Test POST /api/users
	req2, _ := http.NewRequest("POST", "/api/users", nil)
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, req2)
	assert.Equal(t, "create-user", calledRoute)
	assert.Equal(t, 200, w2.Code)

	// Test PUT /api/users/456
	req3, _ := http.NewRequest("PUT", "/api/users/456", nil)
	w3 := httptest.NewRecorder()
	engine.ServeHTTP(w3, req3)
	assert.Equal(t, "update-user", calledRoute)
	assert.Equal(t, 200, w3.Code)
}

func TestRegistryTransactionForDifferentMethods(t *testing.T) {
	testCases := []struct {
		name              string
		method            string
		path              string
		shouldSkipTxn     bool
		expectTransaction bool
	}{
		{
			name:              "GET with transaction should be read-only",
			method:            "GET",
			path:              "/test-get",
			shouldSkipTxn:     false,
			expectTransaction: true,
		},
		{
			name:              "POST with transaction should be writable",
			method:            "POST",
			path:              "/test-post",
			shouldSkipTxn:     false,
			expectTransaction: true,
		},
		{
			name:              "GET /search with transaction should be read-only",
			method:            "GET",
			path:              "/search",
			shouldSkipTxn:     false,
			expectTransaction: true,
		},
		{
			name:              "OPTIONS should not have transaction",
			method:            "OPTIONS",
			path:              "/test-options",
			shouldSkipTxn:     false,
			expectTransaction: false,
		},
		{
			name:              "GET with skip transaction",
			method:            "GET",
			path:              "/test-skip",
			shouldSkipTxn:     true,
			expectTransaction: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup new engine for each test case
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			registry := NewRegistry(engine, nil)

			var hadTransaction bool

			// Add test route
			registry.AddGroup(RouteGroup{
				Name:     "test",
				BasePath: "/api",
				RouteList: []Route{
					{
						Method: tc.method,
						Path:   tc.path,
						Handler: func(ctx request.Context) {
							tx := request.GetTransaction[interface{}](ctx)
							hadTransaction = (tx != nil)
							ctx.JSON(200, gin.H{"ok": true})
						},
						ShouldSkipAuth: true,
						ShouldSkipTxn:  tc.shouldSkipTxn,
					},
				},
			})

			// ExecuteTemplate request
			req, _ := http.NewRequest(tc.method, "/api"+tc.path, nil)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			// Assert
			if tc.expectTransaction {
				assert.True(t, hadTransaction, "%s should have transaction", tc.name)
			} else {
				assert.False(t, hadTransaction, "%s should not have transaction", tc.name)
			}
			assert.Equal(t, 200, w.Code)
		})
	}
}
