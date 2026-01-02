package framework

import (
	"github.com/yadunandan004/scaffold/request"
	"net/http"

	"github.com/gin-gonic/gin"
)

type IDParser[ID IDType] interface {
	ParseID(s string) (ID, error)
}

type BaseReadController[T BaseReadModel[ID], ID IDType] struct {
	Service  ReadOnlyService[T, ID]
	IDParser func(string) (ID, error)
}

func NewBaseReadController[T BaseReadModel[ID], ID IDType](service ReadOnlyService[T, ID], idParser func(string) (ID, error)) *BaseReadController[T, ID] {
	return &BaseReadController[T, ID]{Service: service, IDParser: idParser}
}

func (ctrl *BaseReadController[T, ID]) HandleGetByID(ctx request.Context, paramName string) {
	idStr := ctx.GetRequestContext().Param(paramName)
	id, err := ctrl.IDParser(idStr)
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

func (ctrl *BaseReadController[T, ID]) HandleSearch(ctx request.Context) {
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

type BaseInsertController[T BaseInsertModel[ID], ID IDType] struct {
	BaseReadController[T, ID]
	Service InsertService[T, ID]
}

func NewBaseInsertController[T BaseInsertModel[ID], ID IDType](service InsertService[T, ID], idParser func(string) (ID, error)) *BaseInsertController[T, ID] {
	return &BaseInsertController[T, ID]{
		BaseReadController: BaseReadController[T, ID]{Service: service, IDParser: idParser},
		Service:            service,
	}
}

func (ctrl *BaseInsertController[T, ID]) HandleCreate(ctx request.Context) {
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

func (ctrl *BaseInsertController[T, ID]) HandleCreateMultiple(ctx request.Context) {
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

type BaseController[T BaseModel[ID], ID IDType] struct {
	BaseInsertController[T, ID]
	Service BaseService[T, ID]
}

func NewBaseController[T BaseModel[ID], ID IDType](service BaseService[T, ID], idParser func(string) (ID, error)) *BaseController[T, ID] {
	return &BaseController[T, ID]{
		BaseInsertController: BaseInsertController[T, ID]{
			BaseReadController: BaseReadController[T, ID]{Service: service, IDParser: idParser},
			Service:            service,
		},
		Service: service,
	}
}

func (ctrl *BaseController[T, ID]) HandleUpdate(ctx request.Context) {
	idStr := ctx.GetRequestContext().Param("id")
	id, err := ctrl.IDParser(idStr)
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

func (ctrl *BaseController[T, ID]) HandleDelete(ctx request.Context, paramName string) {
	idStr := ctx.GetRequestContext().Param(paramName)
	id, err := ctrl.IDParser(idStr)
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

func (ctrl *BaseController[T, ID]) HandleUpdateMultiple(ctx request.Context) {
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

func (ctrl *BaseController[T, ID]) HandleDeleteMultiple(ctx request.Context) {
	var req DeleteMultipleRequest
	if err := ctx.GetRequestContext().ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ids := make([]ID, len(req.IDs))
	for i, idStr := range req.IDs {
		id, err := ctrl.IDParser(idStr)
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
