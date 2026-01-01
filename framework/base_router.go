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
	RateLimitRPS   int // Requests per second (0 = use default)
	RateLimitBurst int // Burst size (0 = use default)
}

type RouteGroup struct {
	Name      string
	BasePath  string
	RouteList []Route
}

type BaseReadRouter[T BaseReadModel] struct {
	Name       string
	BasePath   string
	Controller *BaseReadController[T]
	RouteList  []Route
}

func NewBaseReadRouter[T BaseReadModel](tableName string, controller *BaseReadController[T]) *BaseReadRouter[T] {
	return &BaseReadRouter[T]{
		Name:       tableName,
		BasePath:   fmt.Sprintf("/api/v1/%s", tableName),
		Controller: controller,
		RouteList:  []Route{},
	}
}

func (br *BaseReadRouter[T]) InitializeDefaultRoutes() {
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

func (br *BaseReadRouter[T]) AddRoute(route Route) {
	br.RouteList = append(br.RouteList, route)
}

func (br *BaseReadRouter[T]) ToRouteGroup() RouteGroup {
	return RouteGroup{
		Name:      br.Name,
		BasePath:  br.BasePath,
		RouteList: br.RouteList,
	}
}

// BaseInsertRouter - Router for read + insert operations
type BaseInsertRouter[T BaseInsertModel] struct {
	BaseReadRouter[T]
	Controller *BaseInsertController[T]
}

func NewBaseInsertRouter[T BaseInsertModel](tableName string, controller *BaseInsertController[T]) *BaseInsertRouter[T] {
	return &BaseInsertRouter[T]{
		BaseReadRouter: BaseReadRouter[T]{
			Name:       tableName,
			BasePath:   fmt.Sprintf("/api/v1/%s", tableName),
			Controller: &controller.BaseReadController,
			RouteList:  []Route{},
		},
		Controller: controller,
	}
}

func (br *BaseInsertRouter[T]) InitializeDefaultRoutes() {
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

// BaseRouter - Full CRUD router
type BaseRouter[T BaseModel] struct {
	BaseInsertRouter[T]
	Controller *BaseController[T]
}

func NewBaseRouter[T BaseModel](tableName string, controller *BaseController[T]) *BaseRouter[T] {
	return &BaseRouter[T]{
		BaseInsertRouter: BaseInsertRouter[T]{
			BaseReadRouter: BaseReadRouter[T]{
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

func (br *BaseRouter[T]) InitializeDefaultRoutes() {
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
