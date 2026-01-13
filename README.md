# Scaffold

A comprehensive Go framework for building REST and gRPC APIs. Implements a 4-layer architecture pattern (Router → Controller → Service → Repository) with built-in support for dependency injection, transaction management, and context handling.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Installation](#installation)
- [Core Components](#core-components)
  - [Context Interface](#context-interface)
  - [Registry](#registry)
  - [Base Components](#base-components)
- [Usage Examples](#usage-examples)
- [Transaction Management](#transaction-management)
- [ORM](#orm)
- [Configuration](#configuration)

## Architecture Overview

Scaffold follows a clean, layered architecture:

```
┌─────────────┐
│   Router    │  ← Handles HTTP routing and middleware
└──────┬──────┘
       │
┌──────▼──────┐
│ Controller  │  ← Handles HTTP requests/responses
└──────┬──────┘
       │
┌──────▼──────┐
│   Service   │  ← Contains business logic
└──────┬──────┘
       │
┌──────▼──────┐
│ Repository  │  ← Handles database operations
└─────────────┘
```

## Installation

```bash
go get github.com/yadunandan004/scaffold
```

## Core Components

### Context Interface

The `Context` interface provides a unified way to handle both HTTP and gRPC requests with automatic transaction management, user authentication, and request tracing.

```go
import "github.com/yadunandan004/scaffold/request"

type Context interface {
    GetRequestContext() RequestContext
    GetPgTxn() *orm.Query
    GetPgDB() *sql.DB
    CloseTxn(err error) error
    GetUserInfo() *Principal
    XID() uuid.UUID
    TraceID() string
    GetCtx() context.Context
    SetCtx(ctx context.Context)
    JSON(code int, obj interface{})
}
```

#### Context Types

1. **HttpCtx** - For HTTP requests (created automatically by the registry)
2. **GRPCCtx** - For gRPC requests
3. **CustomContext** - For background jobs and custom use cases
4. **TestContext** - For unit testing

### Registry

The `Registry` manages route registration and leverages Gin's native routing with custom middleware for context creation, authentication, and transaction management.

```go
import (
    "github.com/yadunandan004/scaffold/framework"
    "github.com/yadunandan004/scaffold/auth"
)

// Create a new registry
reg := framework.NewRegistry(ginEngine, authService)

// Add route groups
reg.AddGroup(framework.RouteGroup{
    Name:     "users",
    BasePath: "/api/v1/users",
    RouteList: []framework.Route{
        {
            Method:  "GET",
            Path:    "/:id",
            Handler: handleGetUser,
            ShouldSkipAuth: false,
            ShouldSkipTxn:  false,
        },
    },
})
```

### Base Components

#### BaseRouter

Provides default CRUD routes for your resources:

```go
import "github.com/yadunandan004/scaffold/framework"

type UserController struct {
    *framework.BaseController[model.User]
}

func NewUserRouter() *framework.BaseRouter[model.User] {
    service := NewUserService()
    controller := &UserController{
        BaseController: framework.NewBaseController(service),
    }

    router := framework.NewBaseRouter("users", controller)
    router.InitializeDefaultRoutes() // Adds standard CRUD routes

    // Add custom routes
    router.AddRoute(framework.Route{
        Method:  "POST",
        Path:    "/search",
        Handler: controller.HandleSearch,
    })

    return router
}
```

Default routes provided:
- `GET /:id` - Get by ID
- `POST /` - Create
- `PUT /:id` - Update
- `DELETE /:id` - Delete
- `POST /search` - Search with filters
- `POST /bulk` - Create multiple
- `PUT /bulk` - Update multiple
- `DELETE /bulk` - Delete multiple

#### BaseController

Handles HTTP request/response with built-in error handling:

```go
type UserController struct {
    *framework.BaseController[model.User]
    userService UserService
}

func (c *UserController) HandleGetByEmail(ctx request.Context) {
    email := ctx.GetRequestContext().Query("email")

    user, err := c.userService.GetByEmail(ctx, email)
    if err != nil {
        ctx.JSON(404, gin.H{"error": "User not found"})
        return
    }

    ctx.JSON(200, user)
}
```

#### BaseService

Implements business logic with caching support:

```go
type UserService struct {
    *framework.BaseServiceImpl[model.User]
    repo UserRepository
}

func NewUserService() *UserService {
    repo := NewUserRepository()
    return &UserService{
        BaseServiceImpl: framework.NewBaseService[model.User](repo),
        repo: repo,
    }
}

// Custom business logic
func (s *UserService) GetActiveUsers(ctx request.Context) ([]*model.User, error) {
    req := &framework.SearchRequest{
        Filters: []framework.FilterPayload{
            {Field: "status", Operator: "=", Value: "active"},
        },
    }
    return s.Search(ctx, req)
}
```

#### BaseRepository

Handles database operations with automatic tracking:

```go
type UserRepository struct {
    *framework.PostgresRepository[model.User]
}

func NewUserRepository() *UserRepository {
    return &UserRepository{
        PostgresRepository: framework.NewPostgresRepository[model.User](),
    }
}

// Custom query using raw SQL
func (r *UserRepository) GetByEmail(ctx request.Context, email string) (*model.User, error) {
    var user model.User
    query := ctx.GetPgTxn()
    err := query.QueryRow(
        "SELECT id, name, email, created_at, updated_at FROM users WHERE email = $1",
        &user,
        email,
    )
    return &user, err
}
```

## Usage Examples

### Complete Example: User API

```go
import (
    "github.com/yadunandan004/scaffold/framework"
    "github.com/yadunandan004/scaffold/request"
)

// 1. Define your model
type User struct {
    framework.BaseModelImpl
    Name  string `json:"name" orm:"column:name"`
    Email string `json:"email" orm:"column:email"`
}

func (u *User) TableName() string {
    return "users"
}

// 2. Create repository
type UserRepository interface {
    framework.BaseRepository[User]
    GetByEmail(ctx request.Context, email string) (*User, error)
}

type userRepositoryImpl struct {
    *framework.PostgresRepository[User]
}

func NewUserRepository() UserRepository {
    return &userRepositoryImpl{
        PostgresRepository: framework.NewPostgresRepository[User](),
    }
}

func (r *userRepositoryImpl) GetByEmail(ctx request.Context, email string) (*User, error) {
    var user User
    query := ctx.GetPgTxn()
    err := query.QueryRow(
        "SELECT id, name, email, created_at, updated_at FROM users WHERE email = $1",
        &user,
        email,
    )
    return &user, err
}

// 3. Create service
type UserService interface {
    framework.BaseService[User]
    GetByEmail(ctx request.Context, email string) (*User, error)
}

type userServiceImpl struct {
    *framework.BaseServiceImpl[User]
    repo UserRepository
}

func NewUserService() UserService {
    repo := NewUserRepository()
    return &userServiceImpl{
        BaseServiceImpl: framework.NewBaseService[User](repo),
        repo: repo,
    }
}

func (s *userServiceImpl) GetByEmail(ctx request.Context, email string) (*User, error) {
    return s.repo.GetByEmail(ctx, email)
}

// 4. Create controller
type UserController struct {
    *framework.BaseController[User]
    service UserService
}

func NewUserController() *UserController {
    service := NewUserService()
    return &UserController{
        BaseController: framework.NewBaseController[User](service),
        service: service,
    }
}

func (c *UserController) HandleGetByEmail(ctx request.Context) {
    email := ctx.GetRequestContext().Query("email")
    user, err := c.service.GetByEmail(ctx, email)
    if err != nil {
        ctx.JSON(404, gin.H{"error": "User not found"})
        return
    }
    ctx.JSON(200, user)
}

// 5. Create routes
func GetUserRoutes() framework.RouteGroup {
    controller := NewUserController()

    return framework.RouteGroup{
        Name:     "users",
        BasePath: "/api/v1/users",
        RouteList: []framework.Route{
            {
                Method:  "GET",
                Path:    "/:id",
                Handler: controller.HandleGetByID,
                ShouldSkipAuth: false,
                ShouldSkipTxn:  false,
            },
            {
                Method:  "GET",
                Path:    "/email",
                Handler: controller.HandleGetByEmail,
                ShouldSkipAuth: false,
                ShouldSkipTxn:  true, // Read-only operation
            },
            {
                Method:  "POST",
                Path:    "",
                Handler: controller.HandleCreateFromRequest,
                ShouldSkipAuth: false,
                ShouldSkipTxn:  false, // Will create transaction
            },
        },
    }
}

// 6. Register with server
func main() {
    engine := gin.Default()
    authService := auth.NewAuthService(authConfig)

    registry := framework.NewRegistry(engine, authService)
    registry.AddGroup(GetUserRoutes())

    engine.Run(":8080")
}
```

## Transaction Management

The framework automatically manages database transactions based on route configuration:

### Automatic Transaction Handling

```go
// Routes with ShouldSkipTxn: false will automatically get transactions
{
    Method:  "POST",
    Path:    "/users",
    Handler: createUser,
    ShouldSkipTxn: false, // Transaction will be created
}

// Transactions auto-commit on 2xx/3xx responses, rollback on 4xx/5xx
```

### Manual Transaction Access

```go
func (s *UserService) TransferCredits(ctx request.Context, fromID, toID uuid.UUID, amount float64) error {
    // Transaction is already started by the framework if ShouldSkipTxn: false
    query := ctx.GetPgTxn()

    // Execute raw SQL
    _, err := query.Exec(
        "UPDATE users SET credits = credits - $1 WHERE id = $2",
        amount, fromID,
    )
    if err != nil {
        return err // Framework will rollback
    }

    _, err = query.Exec(
        "UPDATE users SET credits = credits + $1 WHERE id = $2",
        amount, toID,
    )
    if err != nil {
        return err // Framework will rollback
    }

    // Transaction will be automatically committed on success
    return nil
}
```

### Custom Context for Background Jobs

```go
func ProcessBatchJob() error {
    // Create custom context with transaction
    ctx := request.CreateCustomContext(
        request.WithPgTxn(),
        request.WithTimeout(5 * time.Minute),
        request.WithTraceID("batch-job-123"),
    )
    defer ctx.CloseTxn(nil)

    service := NewUserService()
    users, err := service.GetActiveUsers(ctx)
    if err != nil {
        return ctx.CloseTxn(err) // Will rollback
    }

    // Process users...

    return ctx.CloseTxn(nil) // Will commit
}
```

## ORM

Scaffold uses a custom lightweight ORM with reflection-based metadata caching.

### Model Definition

```go
type User struct {
    ID        uuid.UUID  `json:"id" orm:"column:id;type:uuid;pk"`
    Name      string     `json:"name" orm:"column:name;type:varchar(100)"`
    Email     string     `json:"email" orm:"column:email;type:varchar(255)"`
    CreatedAt time.Time  `json:"created_at" orm:"column:created_at;default:CURRENT_TIMESTAMP"`
    UpdatedAt time.Time  `json:"updated_at" orm:"column:updated_at;default:CURRENT_TIMESTAMP"`
    DeletedAt *time.Time `json:"-" orm:"column:deleted_at;nullable"`
}

func (u *User) TableName() string {
    return "users"
}
```

### ORM Tags

| Tag | Description |
|-----|-------------|
| `column:name` | Database column name |
| `type:varchar(100)` | Column type |
| `pk` | Primary key |
| `nullable` | Allow NULL values |
| `default:VALUE` | Default value |

### Query Methods

```go
query := ctx.GetPgTxn()

// Single row query
var user User
err := query.QueryRow("SELECT * FROM users WHERE id = $1", &user, userID)

// Multiple rows
var users []*User
err := query.QueryRows("SELECT * FROM users WHERE status = $1", &users, "active")

// Count
count, err := query.Count("SELECT COUNT(*) FROM users WHERE status = $1", "active")

// Exists check
exists, err := query.Exists("SELECT 1 FROM users WHERE email = $1", email)

// Execute (INSERT/UPDATE/DELETE)
result, err := query.Exec("UPDATE users SET status = $1 WHERE id = $2", "inactive", userID)
```

### Using Transaction Helper

```go
import "github.com/yadunandan004/scaffold/orm"

tx := orm.NewTransaction[User]()

// Create
err := tx.Create(query, &user)

// Update
err := tx.Update(query, &user)

// Delete
err := tx.Delete(query, &user)

// Batch operations
err := tx.CreateMultiple(query, users)

// Upsert with conflict handling
err := tx.Upsert(query, &user, []string{"email"})
```

## Configuration

Use `ConfigResolver` for unified configuration with precedence: config file → environment variables → defaults.

```go
import "github.com/yadunandan004/scaffold/config"

// Load from config file
resolver, err := config.NewConfigResolverFromFile("config.yaml")

// Or just from environment
resolver := config.NewConfigResolver("")

// Get values with fallback
host := resolver.GetString("database.host", "DB_HOST", "localhost")
port := resolver.GetInt("database.port", "DB_PORT", 5432)
enabled := resolver.GetBool("feature.enabled", "FEATURE_ENABLED", false)
```

### Database Configuration

```go
import "github.com/yadunandan004/scaffold/store/postgres"

// Get config from resolver
dbConfig := postgres.GetDBConfig(resolver)

// Or from environment variables
dbConfig := postgres.GetDBConfigFromEnv()

// Environment variables:
// DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
// DB_SSL_MODE, DB_SEARCH_PATH, DB_MAX_OPEN_CONNS, DB_MAX_IDLE_CONNS
```

## Model Lifecycle Hooks

Override these methods in your models for custom behavior:

```go
type User struct {
    framework.BaseModelImpl
    Name  string `json:"name" orm:"column:name"`
    Email string `json:"email" orm:"column:email"`
}

func (u *User) Validate(ctx request.Context) error {
    if u.Email == "" {
        return errors.New("email is required")
    }
    return nil
}

func (u *User) PreInsert(ctx request.Context) error {
    // Called before INSERT
    u.Email = strings.ToLower(u.Email)
    return nil
}

func (u *User) PostInsert(ctx request.Context) error {
    // Called after successful INSERT
    // e.g., send welcome email
    return nil
}

func (u *User) PreUpdate(ctx request.Context) error {
    // Called before UPDATE
    return nil
}

func (u *User) PostUpdate(ctx request.Context) error {
    // Called after successful UPDATE
    return nil
}

func (u *User) PreDelete(ctx request.Context) error {
    // Called before DELETE
    return nil
}

func (u *User) PostDelete(ctx request.Context) error {
    // Called after successful DELETE
    return nil
}
```

## Rate Limiting

Configure per-route rate limits:

```go
framework.Route{
    Method:         "POST",
    Path:           "/api/login",
    Handler:        handleLogin,
    RateLimitRPS:   10,   // 10 requests per second
    RateLimitBurst: 20,   // Allow bursts up to 20
}
```

## Testing

The package provides `TestContext` for unit testing:

```go
func TestUserService_Create(t *testing.T) {
    // Setup test database using testcontainers
    container, _ := postgres.NewMockConnection()
    defer container.Terminate(context.Background())

    ctx := request.NewTestContext()

    service := NewUserService()
    user := &User{
        Name:  "Test User",
        Email: "test@example.com",
    }

    created, err := service.Create(ctx, user)
    assert.NoError(t, err)
    assert.NotEqual(t, uuid.Nil, created.ID)
}
```

## Best Practices

1. **Use Base Components**: Leverage the base components to avoid boilerplate code
2. **Transaction Management**: Let the framework handle transactions automatically
3. **Error Handling**: Return errors from service/repository layers; framework handles rollback
4. **Caching**: Use `NewBaseServiceWithCache` for frequently accessed data
5. **Testing**: Use `TestContext` and `NewMockConnection` for tests
6. **Custom Logic**: Extend base components with your custom methods
7. **Raw SQL**: Use parameterized queries (`$1`, `$2`) to prevent SQL injection

## Performance Considerations

- Routes use Gin's radix tree routing (O(log n) lookup)
- Transactions are only created when needed
- ORM metadata is cached per model type (one-time reflection cost)
- Connection pooling with configurable limits
- Rate limiting uses token bucket algorithm

## Package Structure

```
scaffold/
├── auth/           # JWT authentication and middleware
├── config/         # Configuration resolver
├── framework/      # Base components (router, controller, service, repository)
├── logger/         # Structured logging with multiple backends
├── metrics/        # Prometheus metrics and OpenTelemetry
├── orm/            # Lightweight ORM with reflection-based scanning
├── rate_limiter/   # HTTP and gRPC rate limiting
├── request/        # Context interface and implementations
├── singleton/      # Singleton pattern helpers
└── store/
    ├── cache/      # Redis cache service
    ├── object_storage/  # S3-compatible storage
    └── postgres/   # PostgreSQL connection management
```