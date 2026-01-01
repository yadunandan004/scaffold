package framework

import (
	"github.com/yadunandan004/scaffold/request"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BaseReadController[T BaseReadModel] struct {
	Service ReadOnlyService[T]
}

func NewBaseReadController[T BaseReadModel](service ReadOnlyService[T]) *BaseReadController[T] {
	return &BaseReadController[T]{Service: service}
}

// HandleGetByID gets an entity by its ID
func (ctrl *BaseReadController[T]) HandleGetByID(ctx request.Context, paramName string) {
	idStr := ctx.GetRequestContext().Param(paramName)
	id, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	entity, err := ctrl.Service.GetByID(ctx, id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	ctx.JSON(http.StatusOK, entity)
}

func (ctrl *BaseReadController[T]) HandleSearch(ctx request.Context) {
	var searchReq SearchRequest
	if err := ctx.GetRequestContext().ShouldBindJSON(&searchReq); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	results, err := ctrl.Service.Search(ctx, &searchReq)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, results)
}

// BaseInsertController - Controller with read + insert operations
type BaseInsertController[T BaseInsertModel] struct {
	BaseReadController[T]
	Service InsertService[T]
}

func NewBaseInsertController[T BaseInsertModel](service InsertService[T]) *BaseInsertController[T] {
	return &BaseInsertController[T]{
		BaseReadController: BaseReadController[T]{Service: service},
		Service:            service,
	}
}

func (ctrl *BaseInsertController[T]) HandleCreate(ctx request.Context) {
	var entity T
	if err := ctx.GetRequestContext().ShouldBindJSON(&entity); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdEntity, err := ctrl.Service.Create(ctx, &entity)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, createdEntity)
}

func (ctrl *BaseInsertController[T]) HandleCreateMultiple(ctx request.Context) {
	var entities []*T
	if err := ctx.GetRequestContext().ShouldBindJSON(&entities); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdEntities, err := ctrl.Service.CreateMultiple(ctx, entities)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, createdEntities)
}

// BaseController - Full CRUD controller (keeping for backward compatibility)
type BaseController[T BaseModel] struct {
	BaseInsertController[T]
	Service BaseService[T]
}

func NewBaseController[T BaseModel](service BaseService[T]) *BaseController[T] {
	return &BaseController[T]{
		BaseInsertController: BaseInsertController[T]{
			BaseReadController: BaseReadController[T]{Service: service},
			Service:            service,
		},
		Service: service,
	}
}

func (ctrl *BaseController[T]) HandleUpdate(ctx request.Context) {
	idStr := ctx.GetRequestContext().Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var entity T
	if err := ctx.GetRequestContext().ShouldBindJSON(&entity); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedEntity, err := ctrl.Service.Update(ctx, id, &entity)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, updatedEntity)
}

func (ctrl *BaseController[T]) HandleDelete(ctx request.Context, paramName string) {
	idStr := ctx.GetRequestContext().Param(paramName)
	id, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	err = ctrl.Service.Delete(ctx, id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

func (ctrl *BaseController[T]) HandleUpdateMultiple(ctx request.Context) {
	var entities []*T
	if err := ctx.GetRequestContext().ShouldBindJSON(&entities); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedEntities, err := ctrl.Service.UpdateMultiple(ctx, entities)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, updatedEntities)
}

type DeleteMultipleRequest struct {
	IDs []string `json:"ids"`
}

func (ctrl *BaseController[T]) HandleDeleteMultiple(ctx request.Context) {
	var req DeleteMultipleRequest
	if err := ctx.GetRequestContext().ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ids := make([]uuid.UUID, len(req.IDs))
	for i, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID: " + idStr})
			return
		}
		ids[i] = id
	}

	err := ctrl.Service.DeleteMultiple(ctx, ids)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Deleted successfully", "count": len(ids)})
}
