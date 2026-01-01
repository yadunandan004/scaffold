package framework

import (
	"fmt"
	"github.com/yadunandan004/scaffold/store/postgres"
	"reflect"
	"strings"

	"github.com/google/uuid"

	"github.com/yadunandan004/scaffold/orm"
	injContext "github.com/yadunandan004/scaffold/request"
)

// Context is an alias for the injector request
type Context = injContext.Context

// ReadOnlyRepository - Interface for read-only operations
type ReadOnlyRepository[T BaseReadModel] interface {
	GetByID(ctx Context, id uuid.UUID) (*T, error)
	Search(ctx Context, req *SearchRequest) ([]*T, error)
}

// InsertRepository - Interface for read + insert operations
type InsertRepository[T BaseInsertModel] interface {
	ReadOnlyRepository[T]
	Create(ctx Context, entity *T) error
	CreateMultiple(ctx Context, entities []*T) error
}

// UpdateRepository - Interface for read + insert + update operations
type UpdateRepository[T BaseUpdateModel] interface {
	InsertRepository[T]
	Update(ctx Context, entity *T) error
	UpdateMultiple(ctx Context, entities []*T) error
}

// DeleteRepository - Interface for read + insert + update + delete operations
type DeleteRepository[T BaseDeleteModel] interface {
	UpdateRepository[T]
	Delete(ctx Context, entity *T) error
	DeleteMultiple(ctx Context, entities []*T) error
}

// BaseRepository - Full CRUD operations
type BaseRepository[T BaseCompleteModel] interface {
	DeleteRepository[T]
	Upsert(ctx Context, entity *T) error
}

// PostgresReadOnlyRepository - Implementation for read-only operations using custom ORM
type PostgresReadOnlyRepository[T BaseReadModel] struct{}

func NewPostgresReadOnlyRepository[T BaseReadModel]() *PostgresReadOnlyRepository[T] {
	return &PostgresReadOnlyRepository[T]{}
}

func getExecutor[T any](ctx Context) interface{} {
	tx := injContext.GetTransaction[T](ctx)
	if tx != nil {
		return tx
	}

	db := postgres.GetDB()
	if db == nil || db.DB == nil {
		return nil
	}
	return orm.NewDB[T](db.DB)
}

func (r *PostgresReadOnlyRepository[T]) GetByID(ctx Context, id uuid.UUID) (*T, error) {
	var entity T
	executor := getExecutor[T](ctx)
	if executor == nil {
		return nil, fmt.Errorf("no database connection available")
	}

	if tx, ok := executor.(*orm.Transaction[T]); ok {
		query := ctx.GetPgTxn()
		err := tx.FindByPK(query, &entity, id)
		return &entity, err
	}

	db, ok := executor.(*orm.DB[T])
	if !ok || db == nil {
		return nil, fmt.Errorf("invalid database executor")
	}
	err := db.FindByPK(ctx.GetCtx(), &entity, id)
	return &entity, err
}

func (r *PostgresReadOnlyRepository[T]) Search(ctx Context, req *SearchRequest) ([]*T, error) {
	var entity T
	tableName := entity.TableName()

	selectClause := "*"
	if req.HasColumns() {
		selectClause = strings.Join(req.GetColumns(), ", ")
	}

	whereClause, args := BuildWhereClause(req.Filters)
	orderByClause := BuildOrderByClause(req.Sort)
	paginationClause := BuildPaginationClause(req.Page, req.Take)
	query := fmt.Sprintf("SELECT %s FROM %s %s%s%s", selectClause, tableName, whereClause, orderByClause, paginationClause)

	executor := getExecutor[T](ctx)
	if executor == nil {
		return nil, fmt.Errorf("no database connection available")
	}

	if tx, ok := executor.(*orm.Transaction[T]); ok {
		q := ctx.GetPgTxn()
		return tx.FindByQuery(q, query, args...)
	}

	db, ok := executor.(*orm.DB[T])
	if !ok || db == nil {
		return nil, fmt.Errorf("invalid database executor")
	}
	return db.FindByQuery(ctx.GetCtx(), query, args...)
}

// PostgresInsertRepository - Implementation for read + insert operations using custom ORM
type PostgresInsertRepository[T BaseInsertModel] struct {
	PostgresReadOnlyRepository[T]
}

func NewPostgresInsertRepository[T BaseInsertModel]() *PostgresInsertRepository[T] {
	return &PostgresInsertRepository[T]{}
}

func (r *PostgresInsertRepository[T]) Create(ctx Context, entity *T) error {
	if err := (*entity).PreInsert(ctx); err != nil {
		return err
	}

	executor := getExecutor[T](ctx)
	if executor == nil {
		return fmt.Errorf("no database connection available")
	}

	var err error
	if tx, ok := executor.(*orm.Transaction[T]); ok {
		query := ctx.GetPgTxn()
		err = tx.Create(query, entity)
	} else {
		db, ok := executor.(*orm.DB[T])
		if !ok || db == nil {
			return fmt.Errorf("invalid database executor")
		}
		err = db.Create(ctx.GetCtx(), entity)
	}

	if err != nil {
		return err
	}

	if (*entity).SaveTracker() {
		tracker := (*entity).MapToTracker(ctx)
		if tracker != nil {
			tableName := (*entity).TrackerTableName()
			if err := r.createTracker(ctx, tracker, tableName); err != nil {
				return fmt.Errorf("failed to create tracker: %w", err)
			}
		}
	}

	return (*entity).PostInsert(ctx)
}

func (r *PostgresInsertRepository[T]) CreateMultiple(ctx Context, entities []*T) error {
	if len(entities) == 0 {
		return nil
	}

	// Pre-insert hooks
	for _, entity := range entities {
		if err := (*entity).PreInsert(ctx); err != nil {
			return fmt.Errorf("pre-insert failed: %w", err)
		}
	}

	executor := getExecutor[T](ctx)
	if executor == nil {
		return fmt.Errorf("no database connection available")
	}

	var err error
	if tx, ok := executor.(*orm.Transaction[T]); ok {
		query := ctx.GetPgTxn()
		err = tx.CreateMultiple(query, entities)
	} else {
		db, ok := executor.(*orm.DB[T])
		if !ok || db == nil {
			return fmt.Errorf("invalid database executor")
		}
		err = db.CreateMultiple(ctx.GetCtx(), entities)
	}

	if err != nil {
		return err
	}

	for _, entity := range entities {
		if err := (*entity).PostInsert(ctx); err != nil {
			return fmt.Errorf("post-insert failed: %w", err)
		}
	}

	return nil
}

func (r *PostgresInsertRepository[T]) createTracker(ctx Context, tracker interface{}, tableName string) error {
	if tracker == nil {
		return nil
	}

	db := postgres.GetDB()
	if db == nil {
		return fmt.Errorf("no database connection")
	}
	val := reflect.ValueOf(tracker).Elem()
	typ := val.Type()

	var columns []string
	var placeholders []string
	var values []interface{}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if dbTag := field.Tag.Get("db"); dbTag != "" && dbTag != "-" {
			columns = append(columns, dbTag)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
			values = append(values, val.Field(i).Interface())
		}
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	_, err := db.ExecContext(ctx.GetRequestContext().GetCtx(), query, values...)
	return err
}

// PostgresUpdateRepository - Implementation for read + insert + update operations
type PostgresUpdateRepository[T BaseUpdateModel] struct {
	PostgresInsertRepository[T]
}

func NewPostgresUpdateRepository[T BaseUpdateModel]() *PostgresUpdateRepository[T] {
	return &PostgresUpdateRepository[T]{}
}

func (r *PostgresUpdateRepository[T]) Update(ctx Context, entity *T) error {
	if err := (*entity).PreUpdate(ctx); err != nil {
		return err
	}

	executor := getExecutor[T](ctx)
	if executor == nil {
		return fmt.Errorf("no database connection available")
	}

	var err error
	if tx, ok := executor.(*orm.Transaction[T]); ok {
		query := ctx.GetPgTxn()
		err = tx.Update(query, entity)
	} else {
		db, ok := executor.(*orm.DB[T])
		if !ok || db == nil {
			return fmt.Errorf("invalid database executor")
		}
		err = db.Update(ctx.GetCtx(), entity)
	}

	if err != nil {
		return err
	}

	if (*entity).SaveTracker() {
		tracker := (*entity).MapToTracker(ctx)
		if tracker != nil {
			tableName := (*entity).TrackerTableName()
			if err := r.createTracker(ctx, tracker, tableName); err != nil {
				return fmt.Errorf("failed to create tracker: %w", err)
			}
		}
	}

	return (*entity).PostUpdate(ctx)
}

func (r *PostgresUpdateRepository[T]) UpdateMultiple(ctx Context, entities []*T) error {
	if len(entities) == 0 {
		return nil
	}

	for _, entity := range entities {
		if err := (*entity).PreUpdate(ctx); err != nil {
			return err
		}
	}

	executor := getExecutor[T](ctx)
	if executor == nil {
		return fmt.Errorf("no database connection available")
	}

	var err error
	if tx, ok := executor.(*orm.Transaction[T]); ok {
		query := ctx.GetPgTxn()
		err = tx.UpdateMultiple(query, entities)
	} else {
		db, ok := executor.(*orm.DB[T])
		if !ok || db == nil {
			return fmt.Errorf("invalid database executor")
		}
		err = db.UpdateMultiple(ctx.GetCtx(), entities)
	}

	if err != nil {
		return err
	}

	for _, entity := range entities {
		if err := (*entity).PostUpdate(ctx); err != nil {
			return fmt.Errorf("post-update failed: %w", err)
		}
	}

	return nil
}

// PostgresDeleteRepository - Implementation for full CRUD minus upsert
type PostgresDeleteRepository[T BaseDeleteModel] struct {
	PostgresUpdateRepository[T]
}

func NewPostgresDeleteRepository[T BaseDeleteModel]() *PostgresDeleteRepository[T] {
	return &PostgresDeleteRepository[T]{}
}

func (r *PostgresDeleteRepository[T]) Delete(ctx Context, entity *T) error {
	if err := (*entity).PreDelete(ctx); err != nil {
		return err
	}

	executor := getExecutor[T](ctx)
	if executor == nil {
		return fmt.Errorf("no database connection available")
	}

	var err error
	if tx, ok := executor.(*orm.Transaction[T]); ok {
		query := ctx.GetPgTxn()
		err = tx.Delete(query, entity)
	} else {
		db, ok := executor.(*orm.DB[T])
		if !ok || db == nil {
			return fmt.Errorf("invalid database executor")
		}
		err = db.Delete(ctx.GetCtx(), entity)
	}

	if err != nil {
		return err
	}

	if (*entity).SaveTracker() {
		tracker := (*entity).MapToTracker(ctx)
		if tracker != nil {
			tableName := (*entity).TrackerTableName()
			if err := r.createTracker(ctx, tracker, tableName); err != nil {
				return fmt.Errorf("failed to create tracker: %w", err)
			}
		}
	}

	return (*entity).PostDelete(ctx)
}

func (r *PostgresDeleteRepository[T]) DeleteMultiple(ctx Context, entities []*T) error {
	if len(entities) == 0 {
		return nil
	}

	for _, entity := range entities {
		if err := (*entity).PreDelete(ctx); err != nil {
			return err
		}
	}

	executor := getExecutor[T](ctx)
	if executor == nil {
		return fmt.Errorf("no database connection available")
	}

	var err error
	if tx, ok := executor.(*orm.Transaction[T]); ok {
		query := ctx.GetPgTxn()
		err = tx.DeleteMultiple(query, entities)
	} else {
		db, ok := executor.(*orm.DB[T])
		if !ok || db == nil {
			return fmt.Errorf("invalid database executor")
		}
		err = db.DeleteMultiple(ctx.GetCtx(), entities)
	}

	if err != nil {
		return err
	}

	for _, entity := range entities {
		if err := (*entity).PostDelete(ctx); err != nil {
			return fmt.Errorf("post-delete failed: %w", err)
		}
	}

	return nil
}

// PostgresRepository - Full CRUD implementation with upsert
type PostgresRepository[T BaseCompleteModel] struct {
	PostgresDeleteRepository[T]
}

func NewPostgresRepository[T BaseCompleteModel]() *PostgresRepository[T] {
	return &PostgresRepository[T]{}
}

func (r *PostgresRepository[T]) Upsert(ctx Context, entity *T) error {
	conflictColumns := (*entity).OnConflict()

	executor := getExecutor[T](ctx)
	if executor == nil {
		return fmt.Errorf("no database connection available")
	}

	var err error
	if tx, ok := executor.(*orm.Transaction[T]); ok {
		query := ctx.GetPgTxn()
		err = tx.Upsert(query, entity, conflictColumns)
	} else {
		db, ok := executor.(*orm.DB[T])
		if !ok || db == nil {
			return fmt.Errorf("invalid database executor")
		}
		err = db.Upsert(ctx.GetCtx(), entity, conflictColumns)
	}

	return err
}
