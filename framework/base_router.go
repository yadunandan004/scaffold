package framework

import (
	"fmt"

	"github.com/yadunandan004/scaffold/request"
)

type HandlerFunc func(ctx request.Context)

type Route struct {
	Method         string
	Path           string
	Handler        HandlerFunc
	ShouldSkipAuth bool
	ShouldSkipTxn  bool
	RateLimitRPS   int
	RateLimitBurst int
}

type RouteGroup struct {
	Name      string
	BasePath  string
	RouteList []Route
}

type BaseReadRouter[T BaseReadModel[ID], ID IDType] struct {
	Name       string
	BasePath   string
	Controller *BaseReadController[T, ID]
	RouteList  []Route
}

func NewBaseReadRouter[T BaseReadModel[ID], ID IDType](tableName string, controller *BaseReadController[T, ID]) *BaseReadRouter[T, ID] {
	return &BaseReadRouter[T, ID]{
		Name:       tableName,
		BasePath:   fmt.Sprintf("/api/v1/%s", tableName),
		Controller: controller,
		RouteList:  []Route{},
	}
}

func (br *BaseReadRouter[T, ID]) InitializeDefaultRoutes() {
	br.RouteList = append(br.RouteList,
		Route{
			Method:         request.HTTPMethod.Get(),
			Path:           "/:id",
			Handler:        func(ctx request.Context) { br.Controller.HandleGetByID(ctx, "id") },
			ShouldSkipAuth: false,
			ShouldSkipTxn:  true,
		},
		Route{
			Method:         request.HTTPMethod.Post(),
			Path:           "/search",
			Handler:        br.Controller.HandleSearch,
			ShouldSkipAuth: false,
			ShouldSkipTxn:  true,
		},
	)
}

func (br *BaseReadRouter[T, ID]) AddRoute(route Route) {
	br.RouteList = append(br.RouteList, route)
}

func (br *BaseReadRouter[T, ID]) ToRouteGroup() RouteGroup {
	return RouteGroup{
		Name:      br.Name,
		BasePath:  br.BasePath,
		RouteList: br.RouteList,
	}
}

type BaseInsertRouter[T BaseInsertModel[ID], ID IDType] struct {
	BaseReadRouter[T, ID]
	Controller *BaseInsertController[T, ID]
}

func NewBaseInsertRouter[T BaseInsertModel[ID], ID IDType](tableName string, controller *BaseInsertController[T, ID]) *BaseInsertRouter[T, ID] {
	return &BaseInsertRouter[T, ID]{
		BaseReadRouter: BaseReadRouter[T, ID]{
			Name:       tableName,
			BasePath:   fmt.Sprintf("/api/v1/%s", tableName),
			Controller: &controller.BaseReadController,
			RouteList:  []Route{},
		},
		Controller: controller,
	}
}

func (br *BaseInsertRouter[T, ID]) InitializeDefaultRoutes() {
	br.BaseReadRouter.InitializeDefaultRoutes()

	br.RouteList = append(br.RouteList,
		Route{
			Method:         request.HTTPMethod.Post(),
			Path:           "",
			Handler:        br.Controller.HandleCreate,
			ShouldSkipAuth: false,
			ShouldSkipTxn:  false,
		},
		Route{
			Method:         request.HTTPMethod.Post(),
			Path:           "/bulk",
			Handler:        br.Controller.HandleCreateMultiple,
			ShouldSkipAuth: false,
			ShouldSkipTxn:  false,
		},
	)
}

type BaseRouter[T BaseModel[ID], ID IDType] struct {
	BaseInsertRouter[T, ID]
	Controller *BaseController[T, ID]
}

func NewBaseRouter[T BaseModel[ID], ID IDType](tableName string, controller *BaseController[T, ID]) *BaseRouter[T, ID] {
	return &BaseRouter[T, ID]{
		BaseInsertRouter: BaseInsertRouter[T, ID]{
			BaseReadRouter: BaseReadRouter[T, ID]{
				Name:       tableName,
				BasePath:   fmt.Sprintf("/api/v1/%s", tableName),
				Controller: &controller.BaseReadController,
				RouteList:  []Route{},
			},
			Controller: &controller.BaseInsertController,
		},
		Controller: controller,
	}
}

func (br *BaseRouter[T, ID]) InitializeDefaultRoutes() {
	br.BaseInsertRouter.InitializeDefaultRoutes()

	br.RouteList = append(br.RouteList,
		Route{
			Method:         request.HTTPMethod.Put(),
			Path:           "/:id",
			Handler:        br.Controller.HandleUpdate,
			ShouldSkipAuth: false,
			ShouldSkipTxn:  false,
		},
		Route{
			Method:         request.HTTPMethod.Delete(),
			Path:           "/:id",
			Handler:        func(ctx request.Context) { br.Controller.HandleDelete(ctx, "id") },
			ShouldSkipAuth: false,
			ShouldSkipTxn:  false,
		},
		Route{
			Method:         request.HTTPMethod.Put(),
			Path:           "/bulk",
			Handler:        br.Controller.HandleUpdateMultiple,
			ShouldSkipAuth: false,
			ShouldSkipTxn:  false,
		},
		Route{
			Method:         request.HTTPMethod.Delete(),
			Path:           "/bulk",
			Handler:        br.Controller.HandleDeleteMultiple,
			ShouldSkipAuth: false,
			ShouldSkipTxn:  false,
		},
	)
}
