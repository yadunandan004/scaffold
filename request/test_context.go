package request

import (
	"context"
	"database/sql"
	"github.com/yadunandan004/scaffold/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yadunandan004/scaffold/orm"
)

// TestContext is a test implementation of Context for unit tests
type TestContext struct {
	BaseCtx
	ctx           context.Context
	isReadOnlyTxn bool
}

// TestContextOption defines options for creating TestContext
type TestContextOption func(*TestContext)

// WithTestReadWriteTxn enables a read-write transaction for the test context
func WithTestReadWriteTxn() TestContextOption {
	return func(ctx *TestContext) {
		ctx.isReadOnlyTxn = false

		// Create a new transaction
		db := postgres.GetDB()
		if db != nil {
			if tx, err := db.Begin(); err == nil {
				ctx.ctx = context.WithValue(ctx.ctx, TransactionKey{}, tx)
			}
		}
	}
}

// NewTestContext creates a new test request with a database transaction
func NewTestContext(opts ...TestContextOption) *TestContext {
	ctx := context.Background()

	testCtx := &TestContext{
		BaseCtx: BaseCtx{
			xid:     uuid.New(),
			traceID: "test-trace-" + uuid.New().String()[:8],
			user: &Principal{
				ID:          uuid.New(),
				Email:       "test@example.com",
				DisplayName: "Test User",
			},
		},
		ctx: ctx,
	}

	// Apply options
	for _, opt := range opts {
		opt(testCtx)
	}

	return testCtx
}

// GetRequestContext returns a minimal request request for testing
func (t *TestContext) GetRequestContext() RequestContext {
	return &testRequestContext{ctx: t.ctx}
}

// XID returns the request ID
func (t *TestContext) XID() uuid.UUID {
	return t.xid
}

// TraceID returns the trace ID
func (t *TestContext) TraceID() string {
	return t.traceID
}

// GetCtx returns the underlying request
func (t *TestContext) GetCtx() context.Context {
	return t.ctx
}

// SetCtx updates the request (for transaction storage)
func (t *TestContext) SetCtx(newCtx context.Context) {
	t.ctx = newCtx
}

// GetUserInfo returns the test user information
func (t *TestContext) GetUserInfo() *Principal {
	return t.user
}

// SetUserInfo sets custom user information for testing
func (t *TestContext) SetUserInfo(userID uuid.UUID, email string, displayName string) {
	t.user = &Principal{
		ID:          userID,
		Email:       email,
		DisplayName: displayName,
	}
}

// ClearUserInfo removes user information to simulate unauthenticated requests
func (t *TestContext) ClearUserInfo() {
	t.user = nil
}

// JSON responds with JSON (no-op for test request)
func (t *TestContext) JSON(code int, obj interface{}) {
	// No-op for test request
}

// SetPathParams sets path parameters (no-op for test request)
func (t *TestContext) SetPathParams(params map[string]string) {
	// No-op for test request
}

// GetPathParam returns a path parameter by name (no-op for test request)
func (t *TestContext) GetPathParam(name string) string {
	// No-op for test request
	return ""
}

// GetGinContext returns the gin request (nil for test request)
func (t *TestContext) GetGinContext() *gin.Context {
	// No-op for test request
	return nil
}

// testRequestContext is a minimal implementation of RequestContext for tests
type testRequestContext struct {
	ctx context.Context
}

func (t *testRequestContext) GetCtx() context.Context {
	return t.ctx
}

func (t *testRequestContext) SetCtx(newCtx context.Context) {
	t.ctx = newCtx
}

// HTTP methods - not applicable for test request
func (t *testRequestContext) JSON(code int, obj interface{})        {}
func (t *testRequestContext) ShouldBindJSON(obj interface{}) error  { return nil }
func (t *testRequestContext) ShouldBindQuery(obj interface{}) error { return nil }
func (t *testRequestContext) Param(key string) string               { return "" }
func (t *testRequestContext) Query(key string) string               { return "" }
func (t *testRequestContext) Header(key string) string              { return "" }
func (t *testRequestContext) Status(code int)                       {}
func (t *testRequestContext) Abort()                                {}
func (t *testRequestContext) AbortWithStatus(code int)              {}

// GetPgTxn returns the PostgreSQL Query object from context
func (ctx *TestContext) GetPgTxn() *orm.Query {
	if val := ctx.ctx.Value(QueryKey{}); val != nil {
		if q, ok := val.(*orm.Query); ok {
			return q
		}
	}
	return nil
}

// GetPgDB returns the raw PostgreSQL database connection
func (ctx *TestContext) GetPgDB() *sql.DB {
	db := postgres.GetDB()
	if db == nil {
		return nil
	}
	return db.DB
}

// CloseTxn commits or rolls back the transaction based on the error
func (ctx *TestContext) CloseTxn(err error) error {
	query := ctx.GetPgTxn()
	if query == nil {
		return nil
	}
	if err != nil {
		return query.Rollback()
	}
	return query.Commit()
}

// Rollback rolls back the current transaction
func (ctx *TestContext) Rollback() error {
	query := ctx.GetPgTxn()
	if query == nil {
		return nil
	}
	return query.Rollback()
}

// Ensure TestContext satisfies the Context interface
var _ Context = (*TestContext)(nil)
