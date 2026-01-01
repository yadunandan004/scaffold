package framework

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/yadunandan004/scaffold/logger"
	"github.com/yadunandan004/scaffold/request"
	"github.com/yadunandan004/scaffold/store/cache"

	"github.com/google/uuid"
)

// ReadOnlyService - Interface for read-only service operations
type ReadOnlyService[T BaseReadModel] interface {
	GetByID(ctx request.Context, id uuid.UUID) (*T, error)
	Search(ctx request.Context, req *SearchRequest) ([]*T, error)
}

// InsertService - Interface for read + insert service operations
type InsertService[T BaseInsertModel] interface {
	ReadOnlyService[T]
	Create(ctx request.Context, entity *T) (*T, error)
	CreateMultiple(ctx request.Context, entities []*T) ([]*T, error)
}

// BaseService - Full CRUD service operations
type BaseService[T BaseModel] interface {
	InsertService[T]
	Update(ctx request.Context, id uuid.UUID, entity *T) (*T, error)
	UpdateMultiple(ctx request.Context, entities []*T) ([]*T, error)
	Delete(ctx request.Context, id uuid.UUID) error
	DeleteMultiple(ctx request.Context, ids []uuid.UUID) error
	Upsert(ctx request.Context, entity *T) (*T, error)
}

// ReadOnlyServiceImpl - Implementation for read-only service
type ReadOnlyServiceImpl[T BaseReadModel] struct {
	repository   ReadOnlyRepository[T]
	cacheService cache.CacheService
}

func NewReadOnlyService[T BaseReadModel](repository ReadOnlyRepository[T]) *ReadOnlyServiceImpl[T] {
	return &ReadOnlyServiceImpl[T]{repository: repository}
}

func NewReadOnlyServiceWithCache[T BaseReadModel](repository ReadOnlyRepository[T], cacheService cache.CacheService) *ReadOnlyServiceImpl[T] {
	return &ReadOnlyServiceImpl[T]{
		repository:   repository,
		cacheService: cacheService,
	}
}

// InsertServiceImpl - Implementation for read + insert service
type InsertServiceImpl[T BaseInsertModel] struct {
	ReadOnlyServiceImpl[T]
	repository InsertRepository[T]
}

func NewInsertService[T BaseInsertModel](repository InsertRepository[T]) *InsertServiceImpl[T] {
	return &InsertServiceImpl[T]{
		ReadOnlyServiceImpl: ReadOnlyServiceImpl[T]{repository: repository},
		repository:          repository,
	}
}

func NewInsertServiceWithCache[T BaseInsertModel](repository InsertRepository[T], cacheService cache.CacheService) *InsertServiceImpl[T] {
	return &InsertServiceImpl[T]{
		ReadOnlyServiceImpl: ReadOnlyServiceImpl[T]{
			repository:   repository,
			cacheService: cacheService,
		},
		repository: repository,
	}
}

// BaseServiceImpl - Full CRUD service implementation
type BaseServiceImpl[T BaseModel] struct {
	InsertServiceImpl[T]
	repository BaseRepository[T]
}

func NewBaseService[T BaseModel](repository BaseRepository[T]) *BaseServiceImpl[T] {
	return &BaseServiceImpl[T]{
		InsertServiceImpl: InsertServiceImpl[T]{
			ReadOnlyServiceImpl: ReadOnlyServiceImpl[T]{repository: repository},
			repository:          repository,
		},
		repository: repository,
	}
}

func NewBaseServiceWithCache[T BaseModel](repository BaseRepository[T], cacheService cache.CacheService) *BaseServiceImpl[T] {
	return &BaseServiceImpl[T]{
		InsertServiceImpl: InsertServiceImpl[T]{
			ReadOnlyServiceImpl: ReadOnlyServiceImpl[T]{
				repository:   repository,
				cacheService: cacheService,
			},
			repository: repository,
		},
		repository: repository,
	}
}

// ReadOnlyServiceImpl methods
func (s *ReadOnlyServiceImpl[T]) GetByID(ctx request.Context, id uuid.UUID) (*T, error) {
	startTime := time.Now()
	logger.LogInfo(ctx, "→ ENTER: GetByID(id: %s)", id)
	defer func() {
		logger.LogInfo(ctx, "← EXIT: GetByID (duration: %v)", time.Since(startTime))
	}()

	var entity T
	// Check if caching is enabled for this model
	if s.cacheService != nil && entity.SaveInCache() {
		// Try to get from cache first
		cacheKey := s.getCacheKey(&entity, id)
		cached, err := s.cacheService.Get(ctx.GetRequestContext().GetCtx(), cacheKey)
		if err == nil && cached != nil {
			// Unmarshal cached data
			if data, ok := cached.(string); ok {
				if err := json.Unmarshal([]byte(data), &entity); err == nil {
					return &entity, nil
				}
			}
		}
	}

	// Get from database
	result, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache the result if caching is enabled
	if s.cacheService != nil && (*result).SaveInCache() {
		cacheKey := s.getCacheKey(result, id)
		if data, err := json.Marshal(result); err == nil {
			_ = s.cacheService.Set(ctx.GetRequestContext().GetCtx(), cacheKey, string(data), 5*time.Minute)
		}
	}

	return result, nil
}

func (s *ReadOnlyServiceImpl[T]) Search(ctx request.Context, req *SearchRequest) ([]*T, error) {
	startTime := time.Now()
	logger.LogInfo(ctx, "→ ENTER: Search(filters: %d)", len(req.Filters))
	defer func() {
		logger.LogInfo(ctx, "← EXIT: Search (duration: %v)", time.Since(startTime))
	}()

	return s.repository.Search(ctx, req)
}

// getCacheKey generates a cache key for the entity
func (s *ReadOnlyServiceImpl[T]) getCacheKey(entity *T, id uuid.UUID) string {
	return fmt.Sprintf("%s:%s", (*entity).TableName(), id.String())
}

// InsertServiceImpl methods
func (s *InsertServiceImpl[T]) Create(ctx request.Context, entity *T) (*T, error) {

	if err := s.repository.Create(ctx, entity); err != nil {
		return nil, err
	}

	// Cache the created entity if caching is enabled
	if s.cacheService != nil && (*entity).SaveInCache() {
		// Get ID using reflection (assuming BaseModelImpl)
		if id := s.getEntityID(entity); id != uuid.Nil {
			cacheKey := s.getCacheKey(entity, id)
			if data, err := json.Marshal(entity); err == nil {
				_ = s.cacheService.Set(ctx.GetRequestContext().GetCtx(), cacheKey, string(data), 5*time.Minute)
			}
		}
	}

	return entity, nil
}

func (s *InsertServiceImpl[T]) CreateMultiple(ctx request.Context, entities []*T) ([]*T, error) {
	if err := s.repository.CreateMultiple(ctx, entities); err != nil {
		return nil, err
	}
	return entities, nil
}

// BaseServiceImpl methods (full CRUD)
func (s *BaseServiceImpl[T]) Update(ctx request.Context, id uuid.UUID, entity *T) (*T, error) {

	if err := s.repository.Update(ctx, entity); err != nil {
		return nil, err
	}

	// Update cache if caching is enabled
	if s.cacheService != nil && (*entity).SaveInCache() {
		cacheKey := s.getCacheKey(entity, id)
		if data, err := json.Marshal(entity); err == nil {
			_ = s.cacheService.Set(ctx.GetRequestContext().GetCtx(), cacheKey, string(data), 5*time.Minute)
		}
	}

	return entity, nil
}

func (s *BaseServiceImpl[T]) UpdateMultiple(ctx request.Context, entities []*T) ([]*T, error) {
	if err := s.repository.UpdateMultiple(ctx, entities); err != nil {
		return nil, err
	}
	return entities, nil
}

func (s *BaseServiceImpl[T]) Delete(ctx request.Context, id uuid.UUID) error {
	// First get the entity
	entity, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get entity for deletion: %w", err)
	}

	// Delete from cache if caching is enabled
	if s.cacheService != nil && (*entity).SaveInCache() {
		cacheKey := fmt.Sprintf("%T:%s", *entity, id.String())
		_ = s.cacheService.Delete(ctx.GetRequestContext().GetCtx(), cacheKey)
	}

	return s.repository.Delete(ctx, entity)
}

func (s *BaseServiceImpl[T]) DeleteMultiple(ctx request.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	var entities []*T
	for _, id := range ids {
		entity, err := s.repository.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get entity %s for deletion: %w", id, err)
		}
		entities = append(entities, entity)
	}

	return s.repository.DeleteMultiple(ctx, entities)
}

// getEntityID extracts the ID from the entity using the GetID interface method
func (s *InsertServiceImpl[T]) getEntityID(entity *T) uuid.UUID {
	return (*entity).GetID()
}

// BaseServiceImpl methods (full CRUD)
func (s *BaseServiceImpl[T]) Upsert(ctx request.Context, entity *T) (*T, error) {
	startTime := time.Now()
	logger.LogInfo(ctx, "→ ENTER: Upsert")
	defer func() {
		logger.LogInfo(ctx, "← EXIT: Upsert (duration: %v)", time.Since(startTime))
	}()

	if err := s.repository.Upsert(ctx, entity); err != nil {
		return nil, err
	}

	// Cache the upserted entity if caching is enabled
	if s.cacheService != nil && (*entity).SaveInCache() {
		// Get ID using reflection (assuming BaseModelImpl)
		if id := s.getEntityID(entity); id != uuid.Nil {
			cacheKey := s.getCacheKey(entity, id)
			if data, err := json.Marshal(entity); err == nil {
				_ = s.cacheService.Set(ctx.GetRequestContext().GetCtx(), cacheKey, string(data), 5*time.Minute)
			}
		}
	}

	return entity, nil
}
