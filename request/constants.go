package request

type ContextKey string

func (c ContextKey) String() string {
	return string(c)
}

func (c ContextKey) RequestID() ContextKey {
	return "request_id"
}

func (c ContextKey) UserID() ContextKey {
	return "user_id"
}

func (c ContextKey) UserEmail() ContextKey {
	return "user_email"
}

func (c ContextKey) DB() ContextKey {
	return "db"
}

func (c ContextKey) GRPCCtx() ContextKey {
	return "grpc_ctx"
}

const (
	RequestIDKey ContextKey = "request_id"
	UserIDKey    ContextKey = "user_id"
	UserEmailKey ContextKey = "user_email"
	DBKey        ContextKey = "db"
	GRPCCtxKey   ContextKey = "grpc_ctx"
)

// TransactionKey is used to store transaction in request
// Exported to allow ORM package to use the same key for compatibility
type TransactionKey struct{}

// QueryKey is used to store Query helper in request context
// Replaces TransactionKey for new Query-based transaction management
type QueryKey struct{}
