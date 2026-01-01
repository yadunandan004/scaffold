package request

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yadunandan004/scaffold/orm"
	"github.com/yadunandan004/scaffold/store/postgres"
)

// Use the TransactionKey from request package for compatibility with bus and other packages
// This ensures transactions created by ORM are visible to GetPgTxn() calls

type TxOptions struct {
	ReadOnly bool
}

// BeginTransactionForModel returns a Transaction[T] helper
// It reuses existing transaction from request if present, or creates a new one
func BeginTransactionForModel[T any](ctx Context, opts ...TxOptions) (*orm.Transaction[T], error) {
	reqCtx := ctx.GetCtx()

	// Check if Query already exists in context
	if val := reqCtx.Value(QueryKey{}); val != nil {
		if _, ok := val.(*orm.Query); ok {
			return orm.NewTransaction[T](), nil
		}
	}

	// No existing transaction, create a new one
	db := postgres.GetDB()
	if db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	txOpts := &sql.TxOptions{}
	if len(opts) > 0 && opts[0].ReadOnly {
		txOpts.ReadOnly = true
	}

	sqlTx, err := db.BeginTx(reqCtx, txOpts)
	if err != nil {
		return nil, err
	}

	// Create Query wrapper around transaction
	query := &orm.Query{
		Ctx:     reqCtx,
		Txn:     sqlTx,
		Scanner: &orm.RawScanner{},
	}

	// Store Query in context
	newCtx := context.WithValue(reqCtx, QueryKey{}, query)
	ctx.SetCtx(newCtx)

	// Return stateless helper
	return orm.NewTransaction[T](), nil
}

// GetTransaction returns a Transaction[T] helper if a transaction exists in request
func GetTransaction[T any](ctx Context) *orm.Transaction[T] {
	if GetQuery(ctx) != nil {
		return orm.NewTransaction[T]()
	}
	return nil
}

// BeginTransaction creates a transaction and returns a Query helper
// Stores the Query in context for subsequent operations
func BeginTransaction(ctx Context, opts ...TxOptions) (*orm.Query, error) {
	reqCtx := ctx.GetCtx()

	// Check if Query already exists in context
	if val := reqCtx.Value(QueryKey{}); val != nil {
		if q, ok := val.(*orm.Query); ok {
			return q, nil
		}
	}

	// No existing transaction, create a new one
	db := postgres.GetDB()
	if db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	txOpts := &sql.TxOptions{}
	if len(opts) > 0 && opts[0].ReadOnly {
		txOpts.ReadOnly = true
	}
	sqlTx, err := db.BeginTx(reqCtx, txOpts)
	if err != nil {
		return nil, err
	}

	// Create Query wrapper around transaction
	query := &orm.Query{
		Ctx:     reqCtx,
		Txn:     sqlTx,
		Scanner: &orm.RawScanner{},
	}

	// Store Query in context (not raw sql.Tx anymore!)
	newCtx := context.WithValue(reqCtx, QueryKey{}, query)
	ctx.SetCtx(newCtx)

	return query, nil
}

// GetRawTransaction retrieves the underlying sql.Tx from Query in context
// Returns nil if no transaction exists
func GetRawTransaction(ctx Context) *sql.Tx {
	query := GetQuery(ctx)
	if query == nil {
		return nil
	}
	return query.Txn
}

// GetQuery returns the Query helper from context
// Returns nil if no query/transaction exists
func GetQuery(ctx Context) *orm.Query {
	if val := ctx.GetCtx().Value(QueryKey{}); val != nil {
		if q, ok := val.(*orm.Query); ok {
			return q
		}
	}
	return nil
}

// CommitRawTransaction commits a raw transaction from request
func CommitRawTransaction(ctx Context) error {
	tx := GetRawTransaction(ctx)
	if tx == nil {
		return fmt.Errorf("no transaction in request")
	}
	return tx.Commit()
}

// RollbackRawTransaction rolls back a raw transaction from request
func RollbackRawTransaction(ctx Context) error {
	tx := GetRawTransaction(ctx)
	if tx == nil {
		return fmt.Errorf("no transaction in request")
	}
	return tx.Rollback()
}

// CreateCustomContext creates a new custom request with the given options
func CreateCustomContext(opts ...ContextOption) Context {
	customCtx := &CustomContext{
		BaseCtx: BaseCtx{
			xid: uuid.New(), // Always generate XID by default
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(customCtx)
	}

	// Use provided base context or fall back to context.Background()
	baseCtx := customCtx.baseContext
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	if customCtx.timeout > 0 {
		customCtx.ctx, customCtx.cancel = context.WithTimeout(baseCtx, customCtx.timeout)
	} else {
		customCtx.ctx, customCtx.cancel = context.WithCancel(baseCtx)
	}
	return customCtx
}

// GetCtx returns the underlying request (implements TxContext)
func (c *CustomContext) GetCtx() context.Context {
	return c.ctx
}

// SetCtx sets the underlying request (implements TxContext)
func (c *CustomContext) SetCtx(ctx context.Context) {
	c.ctx = ctx
}

// GetRequestContext returns the request request (self) which implements RequestContext interface
func (c *CustomContext) GetRequestContext() RequestContext {
	return &customRequestContext{ctx: c.ctx}
}

// XID returns the request ID (XID)
func (c *CustomContext) XID() uuid.UUID {
	return c.xid
}

// TraceID returns the trace ID
func (c *CustomContext) TraceID() string {
	return c.traceID
}

// Close closes any open transactions and cancels the request
func (c *CustomContext) Close(err error) error {
	// Cancel the request
	if c.cancel != nil {
		defer c.cancel()
	}
	return nil
}

// customRequestContext is a minimal implementation of RequestContext for custom contexts
type customRequestContext struct {
	ctx context.Context
}

func (c *customRequestContext) GetCtx() context.Context {
	return c.ctx
}

func (c *customRequestContext) SetCtx(newCtx context.Context) {
	c.ctx = newCtx
}

// HTTP methods - not applicable for custom request
func (c *customRequestContext) JSON(code int, obj interface{})        {}
func (c *customRequestContext) ShouldBindJSON(obj interface{}) error  { return nil }
func (c *customRequestContext) ShouldBindQuery(obj interface{}) error { return nil }
func (c *customRequestContext) Param(key string) string               { return "" }
func (c *customRequestContext) Query(key string) string               { return "" }
func (c *customRequestContext) Header(key string) string              { return "" }
func (c *customRequestContext) Status(code int)                       {}
func (c *customRequestContext) Abort()                                {}
func (c *customRequestContext) AbortWithStatus(code int)              {}

// GetUserInfo returns the user information
func (c *CustomContext) GetUserInfo() *Principal {
	return c.user
}

// JSON responds with JSON (no-op for custom request)
func (c *CustomContext) JSON(code int, obj interface{}) {
}

// SetPathParams sets path parameters (no-op for custom request)
func (c *CustomContext) SetPathParams(params map[string]string) {
}

// GetPathParam returns a path parameter by name (no-op for custom request)
func (c *CustomContext) GetPathParam(name string) string {
	return ""
}

// GetGinContext returns the gin request (nil for custom request)
func (c *CustomContext) GetGinContext() *gin.Context {
	return nil
}

// GetPgTxn returns the PostgreSQL Query object from context
func (c *CustomContext) GetPgTxn() *orm.Query {
	if val := c.ctx.Value(QueryKey{}); val != nil {
		if q, ok := val.(*orm.Query); ok {
			return q
		}
	}
	return nil
}

// CloseTxn commits or rolls back the transaction based on the error
func (c *CustomContext) CloseTxn(err error) error {
	query := c.GetPgTxn()
	if query == nil {
		return nil
	}
	if err != nil {
		return query.Rollback()
	}
	return query.Commit()
}

// GetPgDB returns the raw PostgreSQL database connection
func (c *CustomContext) GetPgDB() *sql.DB {
	db := postgres.GetDB()
	if db == nil {
		return nil
	}
	return db.DB
}

// Ensure CustomContext satisfies the Context interface
var _ Context = (*CustomContext)(nil)

// NewQueryWithTxn creates a query helper from context
// Returns error if no transaction exists in context
func NewQueryWithTxn(ctx Context) (*orm.Query, error) {
	tx := GetRawTransaction(ctx)
	if tx == nil {
		return nil, fmt.Errorf("no transaction in request")
	}
	return &orm.Query{
		Ctx:     ctx.GetCtx(),
		Txn:     tx,
		Scanner: &orm.RawScanner{},
	}, nil
}
