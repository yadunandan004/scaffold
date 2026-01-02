package framework

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/yadunandan004/scaffold/logger"
	"github.com/yadunandan004/scaffold/request"
	"github.com/yadunandan004/scaffold/store/cache"
)

type ReadOnlyService[T BaseReadModel[ID], ID IDType] interface {
	GetByID(ctx request.Context, id ID) (*T, error)
	Search(ctx request.Context, req *SearchRequest) ([]*T, error)
}

type InsertService[T BaseInsertModel[ID], ID IDType] interface {
	ReadOnlyService[T, ID]
	Create(ctx request.Context, entity *T) (*T, error)
	CreateMultiple(ctx request.Context, entities []*T) ([]*T, error)
}

type BaseService[T BaseModel[ID], ID IDType] interface {
	InsertService[T, ID]
	Update(ctx request.Context, id ID, entity *T) (*T, error)
	UpdateMultiple(ctx request.Context, entities []*T) ([]*T, error)
	Delete(ctx request.Context, id ID) error
	DeleteMultiple(ctx request.Context, ids []ID) error
	Upsert(ctx request.Context, entity *T) (*T, error)
}

type ReadOnlyServiceImpl[T BaseReadModel[ID], ID IDType] struct {
	repository   ReadOnlyRepository[T, ID]
	cacheService cache.CacheService
}

func NewReadOnlyService[T BaseReadModel[ID], ID IDType](repository ReadOnlyRepository[T, ID]) *ReadOnlyServiceImpl[T, ID] {
	return &ReadOnlyServiceImpl[T, ID]{repository: repository}
}

func NewReadOnlyServiceWithCache[T BaseReadModel[ID], ID IDType](repository ReadOnlyRepository[T, ID], cacheService cache.CacheService) *ReadOnlyServiceImpl[T, ID] {
	return &ReadOnlyServiceImpl[T, ID]{
		repository:   repository,
		cacheService: cacheService,
	}
}

type InsertServiceImpl[T BaseInsertModel[ID], ID IDType] struct {
	ReadOnlyServiceImpl[T, ID]
	repository InsertRepository[T, ID]
}

func NewInsertService[T BaseInsertModel[ID], ID IDType](repository InsertRepository[T, ID]) *InsertServiceImpl[T, ID] {
	return &InsertServiceImpl[T, ID]{
		ReadOnlyServiceImpl: ReadOnlyServiceImpl[T, ID]{repository: repository},
		repository:          repository,
	}
}

func NewInsertServiceWithCache[T BaseInsertModel[ID], ID IDType](repository InsertRepository[T, ID], cacheService cache.CacheService) *InsertServiceImpl[T, ID] {
	return &InsertServiceImpl[T, ID]{
		ReadOnlyServiceImpl: ReadOnlyServiceImpl[T, ID]{
			repository:   repository,
			cacheService: cacheService,
		},
		repository: repository,
	}
}

type BaseServiceImpl[T BaseModel[ID], ID IDType] struct {
	InsertServiceImpl[T, ID]
	repository BaseRepository[T, ID]
}

func NewBaseService[T BaseModel[ID], ID IDType](repository BaseRepository[T, ID]) *BaseServiceImpl[T, ID] {
	return &BaseServiceImpl[T, ID]{
		InsertServiceImpl: InsertServiceImpl[T, ID]{
			ReadOnlyServiceImpl: ReadOnlyServiceImpl[T, ID]{repository: repository},
			repository:          repository,
		},
		repository: repository,
	}
}

func NewBaseServiceWithCache[T BaseModel[ID], ID IDType](repository BaseRepository[T, ID], cacheService cache.CacheService) *BaseServiceImpl[T, ID] {
	return &BaseServiceImpl[T, ID]{
		InsertServiceImpl: InsertServiceImpl[T, ID]{
			ReadOnlyServiceImpl: ReadOnlyServiceImpl[T, ID]{
				repository:   repository,
				cacheService: cacheService,
			},
			repository: repository,
		},
		repository: repository,
	}
}

func (s *ReadOnlyServiceImpl[T, ID]) GetByID(ctx request.Context, id ID) (*T, error) {
	startTime := time.Now()
	logger.LogInfo(ctx, "→ ENTER: GetByID(id: %v)", id)
	defer func() {
		logger.LogInfo(ctx, "← EXIT: GetByID (duration: %v)", time.Since(startTime))
	}()

	var entity T
	if s.cacheService != nil && entity.SaveInCache() {
		cacheKey := s.getCacheKey(&entity, id)
		cached, err := s.cacheService.Get(ctx.GetRequestContext().GetCtx(), cacheKey)
		if err == nil && cached != nil {
			if data, ok := cached.(string); ok {
				if err := json.Unmarshal([]byte(data), &entity); err == nil {
					return &entity, nil
				}
			}
		}
	}

	result, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if s.cacheService != nil && (*result).SaveInCache() {
		cacheKey := s.getCacheKey(result, id)
		if data, err := json.Marshal(result); err == nil {
			_ = s.cacheService.Set(ctx.GetRequestContext().GetCtx(), cacheKey, string(data), 5*time.Minute)
		}
	}

	return result, nil
}

func (s *ReadOnlyServiceImpl[T, ID]) Search(ctx request.Context, req *SearchRequest) ([]*T, error) {
	startTime := time.Now()
	logger.LogInfo(ctx, "→ ENTER: Search(filters: %d)", len(req.Filters))
	defer func() {
		logger.LogInfo(ctx, "← EXIT: Search (duration: %v)", time.Since(startTime))
	}()

	return s.repository.Search(ctx, req)
}

func (s *ReadOnlyServiceImpl[T, ID]) getCacheKey(entity *T, id ID) string {
	return fmt.Sprintf("%s:%v", (*entity).TableName(), id)
}

func (s *InsertServiceImpl[T, ID]) Create(ctx request.Context, entity *T) (*T, error) {
	if err := s.repository.Create(ctx, entity); err != nil {
		return nil, err
	}

	if s.cacheService != nil && (*entity).SaveInCache() {
		id := (*entity).GetID()
		cacheKey := s.getCacheKey(entity, id)
		if data, err := json.Marshal(entity); err == nil {
			_ = s.cacheService.Set(ctx.GetRequestContext().GetCtx(), cacheKey, string(data), 5*time.Minute)
		}
	}

	return entity, nil
}

func (s *InsertServiceImpl[T, ID]) CreateMultiple(ctx request.Context, entities []*T) ([]*T, error) {
	if err := s.repository.CreateMultiple(ctx, entities); err != nil {
		return nil, err
	}
	return entities, nil
}

func (s *BaseServiceImpl[T, ID]) Update(ctx request.Context, id ID, entity *T) (*T, error) {
	if err := s.repository.Update(ctx, entity); err != nil {
		return nil, err
	}

	if s.cacheService != nil && (*entity).SaveInCache() {
		cacheKey := s.getCacheKey(entity, id)
		if data, err := json.Marshal(entity); err == nil {
			_ = s.cacheService.Set(ctx.GetRequestContext().GetCtx(), cacheKey, string(data), 5*time.Minute)
		}
	}

	return entity, nil
}

func (s *BaseServiceImpl[T, ID]) UpdateMultiple(ctx request.Context, entities []*T) ([]*T, error) {
	if err := s.repository.UpdateMultiple(ctx, entities); err != nil {
		return nil, err
	}
	return entities, nil
}

func (s *BaseServiceImpl[T, ID]) Delete(ctx request.Context, id ID) error {
	entity, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get entity for deletion: %w", err)
	}

	if s.cacheService != nil && (*entity).SaveInCache() {
		cacheKey := fmt.Sprintf("%T:%v", *entity, id)
		_ = s.cacheService.Delete(ctx.GetRequestContext().GetCtx(), cacheKey)
	}

	return s.repository.Delete(ctx, entity)
}

func (s *BaseServiceImpl[T, ID]) DeleteMultiple(ctx request.Context, ids []ID) error {
	if len(ids) == 0 {
		return nil
	}

	var entities []*T
	for _, id := range ids {
		entity, err := s.repository.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get entity %v for deletion: %w", id, err)
		}
		entities = append(entities, entity)
	}

	return s.repository.DeleteMultiple(ctx, entities)
}

func (s *BaseServiceImpl[T, ID]) Upsert(ctx request.Context, entity *T) (*T, error) {
	startTime := time.Now()
	logger.LogInfo(ctx, "→ ENTER: Upsert")
	defer func() {
		logger.LogInfo(ctx, "← EXIT: Upsert (duration: %v)", time.Since(startTime))
	}()

	if err := s.repository.Upsert(ctx, entity); err != nil {
		return nil, err
	}

	if s.cacheService != nil && (*entity).SaveInCache() {
		id := (*entity).GetID()
		cacheKey := s.getCacheKey(entity, id)
		if data, err := json.Marshal(entity); err == nil {
			_ = s.cacheService.Set(ctx.GetRequestContext().GetCtx(), cacheKey, string(data), 5*time.Minute)
		}
	}

	return entity, nil
}
