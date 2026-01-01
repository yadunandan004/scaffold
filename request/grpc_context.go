package request

import (
	"context"
	"database/sql"
	"github.com/yadunandan004/scaffold/auth"
	"github.com/yadunandan004/scaffold/store/postgres"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/yadunandan004/scaffold/orm"
)

// GRPCCtx represents the gRPC API request
type GRPCCtx struct {
	BaseCtx
	ctx      context.Context
	metadata metadata.MD
}

// Ensure GRPCCtx satisfies the Context interface
var (
	_ Context        = (*GRPCCtx)(nil)
	_ RequestContext = (*GRPCCtx)(nil)
)

// NewApiContextForGRPC creates a new GRPCCtx instance
func NewApiContextForGRPC(ctx context.Context) Context {
	grpcCtx := &GRPCCtx{
		BaseCtx: BaseCtx{
			xid: uuid.New(), // Generate XID
		},
		ctx: ctx,
	}
	var claims *auth.UserClaims
	if rawClaims, ok := ctx.Value("user_claims").(*auth.UserClaims); ok {
		claims = rawClaims
	}
	if claims == nil {
		return nil
	}
	grpcCtx.user = &Principal{
		ID:             claims.UserID,
		Email:          claims.Email,
		ClientDeviceID: claims.ClientDeviceID,
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		grpcCtx.metadata = md
		if traceIDs := md.Get("traceid"); len(traceIDs) > 0 {
			grpcCtx.traceID = traceIDs[0]
		}
	}
	return grpcCtx
}

// GetRequestContext returns the request request (self) which implements RequestContext interface
func (ctx *GRPCCtx) GetRequestContext() RequestContext {
	return ctx
}

// GetCtx implements RequestContext.GetCtx() - returns the actual request.Context
func (ctx *GRPCCtx) GetCtx() context.Context {
	return ctx.ctx
}

// SetCtx updates the request (for transaction storage)
func (ctx *GRPCCtx) SetCtx(newCtx context.Context) {
	ctx.ctx = newCtx
}

// GetUserInfo returns the user information from request
func (ctx *GRPCCtx) GetUserInfo() *Principal {
	return ctx.user
}

// SetUserInfo sets user information in the request
func (ctx *GRPCCtx) SetUserInfo(userID uuid.UUID, email string, displayName ...string) {
	if ctx.user == nil {
		ctx.user = &Principal{}
	}
	ctx.user.ID = userID
	ctx.user.Email = email
	if len(displayName) > 0 {
		ctx.user.DisplayName = displayName[0]
	}
}

// GetMetadata returns the gRPC metadata
func (ctx *GRPCCtx) GetMetadata() metadata.MD {
	return ctx.metadata
}

// RequestContext methods - minimal implementation for gRPC request
func (ctx *GRPCCtx) JSON(code int, obj interface{}) {
	// Not applicable for gRPC - would use stream.Send() instead
}

func (ctx *GRPCCtx) ShouldBindJSON(obj interface{}) error {
	// Not applicable for gRPC - message unmarshaling is handled by gRPC framework
	return nil
}

func (ctx *GRPCCtx) ShouldBindQuery(obj interface{}) error {
	// Not applicable for gRPC
	return nil
}

func (ctx *GRPCCtx) Param(key string) string {
	// Not applicable for gRPC
	return ""
}

func (ctx *GRPCCtx) Query(key string) string {
	// Not applicable for gRPC
	return ""
}

func (ctx *GRPCCtx) Header(key string) string {
	// gRPC metadata is similar to headers
	if values := ctx.metadata.Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}

func (ctx *GRPCCtx) Status(code int) {
	// Not applicable for gRPC - would use grpc.Status instead
}

func (ctx *GRPCCtx) Abort() {
	// Not applicable for gRPC - would cancel request instead
}

func (ctx *GRPCCtx) AbortWithStatus(code int) {
	// Not applicable for gRPC - would return error with status code
}

// SetPathParams sets path parameters (no-op for gRPC)
func (ctx *GRPCCtx) SetPathParams(params map[string]string) {
	// Not applicable for gRPC
}

// GetPathParam returns a path parameter by name (no-op for gRPC)
func (ctx *GRPCCtx) GetPathParam(name string) string {
	// Not applicable for gRPC
	return ""
}

// GetGinContext returns the gin request (nil for gRPC)
func (ctx *GRPCCtx) GetGinContext() *gin.Context {
	// Not applicable for gRPC
	return nil
}

// XID returns the request ID (XID)
func (ctx *GRPCCtx) XID() uuid.UUID {
	return ctx.xid
}

// TraceID returns the trace ID
func (ctx *GRPCCtx) TraceID() string {
	return ctx.traceID
}

// GRPCUnaryInterceptor creates a unary server interceptor that injects GRPCCtx
func GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		defer func() {
			log.Printf("Closing grpc request with method: %s\n", info.FullMethod)
		}()
		grpcCtx := NewApiContextForGRPC(ctx)

		// Add to request
		newCtx := context.WithValue(ctx, GRPCCtxKey, grpcCtx)

		// Call handler with new request
		return handler(newCtx, req)
	}
}

// GRPCStreamInterceptor creates a stream server interceptor that injects GRPCCtx
func GRPCStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Create GRPCCtx
		grpcCtx := NewApiContextForGRPC(ss.Context())

		// Create wrapped stream with new request
		wrapped := &wrappedServerStream{
			ServerStream: ss,
			ctx:          context.WithValue(ss.Context(), GRPCCtxKey, grpcCtx),
		}

		// Call handler with wrapped stream
		return handler(srv, wrapped)
	}
}

// wrappedServerStream wraps grpc.ServerStream to override request
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// GetPgTxn returns the PostgreSQL Query object from context
func (ctx *GRPCCtx) GetPgTxn() *orm.Query {
	if val := ctx.ctx.Value(QueryKey{}); val != nil {
		if q, ok := val.(*orm.Query); ok {
			return q
		}
	}
	return nil
}

// GetPgDB returns the raw PostgreSQL database connection
func (ctx *GRPCCtx) GetPgDB() *sql.DB {
	db := postgres.GetDB()
	if db == nil {
		return nil
	}
	return db.DB
}

// CloseTxn commits or rolls back the transaction based on the error
func (ctx *GRPCCtx) CloseTxn(err error) error {
	query := ctx.GetPgTxn()
	if query == nil {
		return nil
	}
	if err != nil {
		return query.Rollback()
	}
	return query.Commit()
}

// GetGRPCCtx extracts GRPCCtx from request
func GetGRPCCtx(ctx context.Context) (Context, bool) {
	grpcCtx, ok := ctx.Value(GRPCCtxKey).(*GRPCCtx)
	return grpcCtx, ok
}
