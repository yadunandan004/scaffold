package request

import (
	"bytes"
	"context"
	"database/sql"
	"github.com/yadunandan004/scaffold/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yadunandan004/scaffold/orm"
)

// NewApiContextForHttp creates a new HttpCtx instance and returns it as Context interface
func NewApiContextForHttp(c *gin.Context, opts ...HttpCtxOption) Context {
	ctx := &HttpCtx{
		BaseCtx: BaseCtx{
			xid: uuid.New(), // Generate XID
		},
		ginCtx: c,
	}

	// Extract TraceID from header
	if traceID := c.GetHeader("TraceID"); traceID != "" {
		ctx.traceID = traceID
	}

	// Extract user info from gin request if available
	if userID, exists := c.Get(UserIDKey.String()); exists {
		if id, ok := userID.(uuid.UUID); ok {
			if ctx.user == nil {
				ctx.user = &Principal{}
			}
			ctx.user.ID = id
		}
	}

	if email, exists := c.Get(UserEmailKey.String()); exists {
		if e, ok := email.(string); ok {
			if ctx.user == nil {
				ctx.user = &Principal{}
			}
			ctx.user.Email = e
		}
	}

	if clientDeviceID, exists := c.Get("client_device_id"); exists {
		if cid, ok := clientDeviceID.(string); ok {
			if ctx.user == nil {
				ctx.user = &Principal{}
			}
			ctx.user.ClientDeviceID = cid
		}
	}

	// Apply options
	for _, opt := range opts {
		opt(ctx)
	}

	return ctx
}

// GetRequestContext returns the request request (self) which implements RequestContext interface
func (ctx *HttpCtx) GetRequestContext() RequestContext {
	return ctx
}

// GetCtx implements RequestContext.GetCtx() - returns the actual request.Context
func (ctx *HttpCtx) GetCtx() context.Context {
	return ctx.GetContextWithValues()
}

// SetCtx implements RequestContext.SetCtx() - updates the request request
func (ctx *HttpCtx) SetCtx(newCtx context.Context) {
	ctx.ginCtx.Request = ctx.ginCtx.Request.WithContext(newCtx)
}

// GetContextWithValues returns the actual request.Context with all necessary values
func (ctx *HttpCtx) GetContextWithValues() context.Context {
	return ctx.ginCtx.Request.Context()
}

// GetRequestID returns the request ID from request
func (ctx *HttpCtx) GetRequestID() string {
	if reqID, exists := ctx.ginCtx.Get(RequestIDKey.String()); exists {
		if id, ok := reqID.(string); ok {
			return id
		}
	}
	return ""
}

// GetUserID returns the user ID from request
func (ctx *HttpCtx) GetUserID() (uuid.UUID, bool) {
	if userID, exists := ctx.ginCtx.Get(UserIDKey.String()); exists {
		if id, ok := userID.(uuid.UUID); ok {
			return id, true
		}
	}
	return uuid.Nil, false
}

// GetUserEmail returns the user email from request
func (ctx *HttpCtx) GetUserEmail() (string, bool) {
	if email, exists := ctx.ginCtx.Get(UserEmailKey.String()); exists {
		if e, ok := email.(string); ok {
			return e, true
		}
	}
	return "", false
}

// SetUserInfo sets user information in the request
func (ctx *HttpCtx) SetUserInfo(userID uuid.UUID, email string, displayName ...string) {
	ctx.ginCtx.Set(UserIDKey.String(), userID)
	ctx.ginCtx.Set(UserEmailKey.String(), email)

	// Update pkg user struct
	if ctx.user == nil {
		ctx.user = &Principal{}
	}
	ctx.user.ID = userID
	ctx.user.Email = email
	if len(displayName) > 0 {
		ctx.user.DisplayName = displayName[0]
	}
}

// GetGinContext returns the underlying gin request
func (ctx *HttpCtx) GetGinContext() *gin.Context {
	return ctx.ginCtx
}

// InstallBufferedWriter replaces the gin writer with a buffered one to capture error responses
func (g *HttpCtx) InstallBufferedWriter() {
	// Create a new buffered writer that wraps the existing one
	bw := &bufferedResponseWriter{
		ResponseWriter: g.ginCtx.Writer,
		body:           &bytes.Buffer{},
		statusCode:     0,
	}
	// Replace the writer
	g.ginCtx.Writer = bw
}

// GetUserInfo returns the user information from request
func (ctx *HttpCtx) GetUserInfo() *Principal {
	return ctx.user
}

// XID returns the request ID (XID)
func (ctx *HttpCtx) XID() uuid.UUID {
	return ctx.xid
}

// TraceID returns the trace ID
func (ctx *HttpCtx) TraceID() string {
	return ctx.traceID
}

func (ctx *HttpCtx) JSON(code int, obj interface{}) {
	ctx.ginCtx.JSON(code, obj)
}

func (ctx *HttpCtx) ShouldBindJSON(obj interface{}) error {
	return ctx.ginCtx.ShouldBindJSON(obj)
}

func (ctx *HttpCtx) ShouldBindQuery(obj interface{}) error {
	return ctx.ginCtx.ShouldBindQuery(obj)
}

func (ctx *HttpCtx) Param(key string) string {
	// Use Gin's native param extraction
	if ctx.ginCtx != nil {
		return ctx.ginCtx.Param(key)
	}
	// Fallback to pathParams if set manually (for testing)
	if ctx.pathParams != nil {
		return ctx.pathParams[key]
	}
	return ""
}

func (ctx *HttpCtx) SetPathParams(params map[string]string) {
	ctx.pathParams = params
}

func (ctx *HttpCtx) GetPathParam(name string) string {
	if ctx.pathParams != nil {
		return ctx.pathParams[name]
	}
	return ""
}

func (ctx *HttpCtx) Query(key string) string {
	return ctx.ginCtx.Query(key)
}

func (ctx *HttpCtx) Header(key string) string {
	return ctx.ginCtx.GetHeader(key)
}

func (ctx *HttpCtx) Status(code int) {
	ctx.ginCtx.Status(code)
}

func (ctx *HttpCtx) Abort() {
	ctx.ginCtx.Abort()
}

func (ctx *HttpCtx) AbortWithStatus(code int) {
	ctx.ginCtx.AbortWithStatus(code)
}

// GetPgTxn returns the PostgreSQL Query object from context
func (ctx *HttpCtx) GetPgTxn() *orm.Query {
	reqCtx := ctx.GetCtx()
	if val := reqCtx.Value(QueryKey{}); val != nil {
		if q, ok := val.(*orm.Query); ok {
			return q
		}
	}
	return nil
}

// GetPgDB returns the raw PostgreSQL database connection
func (ctx *HttpCtx) GetPgDB() *sql.DB {
	db := postgres.GetDB()
	if db == nil {
		return nil
	}
	return db.DB
}

// CloseTxn commits or rolls back the transaction based on the error
func (ctx *HttpCtx) CloseTxn(err error) error {
	query := ctx.GetPgTxn()
	if query == nil {
		return nil
	}
	if err != nil {
		return query.Rollback()
	}
	return query.Commit()
}
