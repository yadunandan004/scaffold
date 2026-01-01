package request

import (
	"bytes"
	"context"
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yadunandan004/scaffold/orm"
)

// Principal represents the authenticated user information
type Principal struct {
	ID             uuid.UUID
	Email          string
	DisplayName    string
	ClientDeviceID string // UUID of the client device from JWT claims
}

func (p *Principal) GetID() uuid.UUID {
	return p.ID
}

func (p *Principal) GetEmail() string {
	return p.Email
}

// BaseCtx contains common fields for both HTTP and gRPC contexts
type BaseCtx struct {
	user    *Principal
	xid     uuid.UUID
	traceID string
}

// HttpCtx represents the HTTP API request
type HttpCtx struct {
	BaseCtx
	ginCtx     *gin.Context
	pathParams map[string]string
}

type RequestContext interface {
	GetCtx() context.Context
	SetCtx(ctx context.Context)
	JSON(code int, obj interface{})
	ShouldBindJSON(obj interface{}) error
	ShouldBindQuery(obj interface{}) error
	Param(key string) string
	Query(key string) string
	Header(key string) string
	Status(code int)
	Abort()
	AbortWithStatus(code int)
}

// ContextOption is a function that configures a CustomContext
type ContextOption func(*CustomContext)

// WithPgTxn enables PostgreSQL transaction for the request
func WithPgTxn() ContextOption {
	return func(c *CustomContext) {
		c.enablePgTxn = true
	}
}

// WithTraceID sets a specific trace ID for the request
func WithTraceID(traceID string) ContextOption {
	return func(c *CustomContext) {
		c.traceID = traceID
	}
}

// WithUserInfo sets user information for the request
func WithUserInfo(user *Principal) ContextOption {
	return func(c *CustomContext) {
		c.user = user
	}
}

// WithTimeout sets a timeout for the request
func WithTimeout(timeout time.Duration) ContextOption {
	return func(c *CustomContext) {
		c.timeout = timeout
	}
}

// WithoutTimeout disables timeout for the request
func WithoutTimeout() ContextOption {
	return func(c *CustomContext) {
		c.noTimeout = true
	}
}

// WithBaseContext sets a base context to use instead of context.Background()
func WithBaseContext(ctx context.Context) ContextOption {
	return func(c *CustomContext) {
		c.baseContext = ctx
	}
}

// CustomContext is a custom implementation of Context for specific use cases
type CustomContext struct {
	BaseCtx
	ctx               context.Context
	cancel            context.CancelFunc
	xid               uuid.UUID
	traceID           string
	enablePgTxn       bool
	enableReadOnlyTxn bool
	enableChTxn       bool
	timeout           time.Duration
	noTimeout         bool
	baseContext       context.Context // Optional base context to use instead of context.Background()
}

// bufferedResponseWriter wraps gin.ResponseWriter to capture error responses
type bufferedResponseWriter struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (w *bufferedResponseWriter) Write(data []byte) (int, error) {
	// Capture the data
	w.body.Write(data)
	// Also write to the original writer
	return w.ResponseWriter.Write(data)
}

func (w *bufferedResponseWriter) WriteString(s string) (int, error) {
	// Capture the string
	w.body.WriteString(s)
	// Also write to the original writer
	return w.ResponseWriter.WriteString(s)
}

func (w *bufferedResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bufferedResponseWriter) Status() int {
	if w.statusCode == 0 {
		return w.ResponseWriter.Status()
	}
	return w.statusCode
}

// Context defines the common interface for both HTTP and gRPC contexts
type Context interface {
	GetRequestContext() RequestContext
	GetUserInfo() *Principal
	XID() uuid.UUID
	TraceID() string
	// Context management for transactions
	GetCtx() context.Context
	SetCtx(ctx context.Context)
	// Transaction methods
	GetPgTxn() *orm.Query
	GetPgDB() *sql.DB
	CloseTxn(err error) error
	// HTTP methods - will be no-op for gRPC
	GetGinContext() *gin.Context
	SetPathParams(params map[string]string)
	GetPathParam(name string) string
	JSON(code int, obj interface{})
}

// Ensure both implementations satisfy the interface
var (
	_ Context        = (*HttpCtx)(nil)
	_ RequestContext = (*HttpCtx)(nil)
)

// HttpCtxOption defines options for creating HttpCtx
type HttpCtxOption func(*HttpCtx)
