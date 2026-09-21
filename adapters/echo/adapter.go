// Package echoadapter mounts controllers directly on Echo v5 engines and groups.
package echoadapter

import (
	"fmt"

	"github.com/emmybxt/go-controller/v2/gocontroller"
	"github.com/emmybxt/go-controller/v2/internal/native"
	"github.com/labstack/echo/v5"
)

// Router is implemented by *echo.Echo and *echo.Group.
type Router interface {
	Add(string, string, echo.HandlerFunc, ...echo.MiddlewareFunc) echo.RouteInfo
}

type adapter struct{ router Router }

func New(router Router) gocontroller.Adapter { return &adapter{router: router} }

func (a *adapter) Register(routes []gocontroller.Route) error {
	if native.IsNil(a.router) {
		return fmt.Errorf("echo router is nil")
	}
	return native.Register(routes, func(method, path string, handler echo.HandlerFunc, middleware, after []echo.MiddlewareFunc) {
		a.router.Add(method, path, withAfter(handler, after), middleware...)
	})
}
