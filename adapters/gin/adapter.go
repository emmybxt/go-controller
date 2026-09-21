// Package ginadapter mounts controllers directly on Gin engines and groups.
package ginadapter

import (
	"fmt"

	"github.com/emmybxt/go-controller/v2/gocontroller"
	"github.com/emmybxt/go-controller/v2/internal/native"
	"github.com/gin-gonic/gin"
)

type adapter struct{ router gin.IRoutes }

func New(router gin.IRoutes) gocontroller.Adapter { return &adapter{router: router} }

func (a *adapter) Register(routes []gocontroller.Route) error {
	if native.IsNil(a.router) {
		return fmt.Errorf("gin router is nil")
	}
	return native.Register(routes, func(method, path string, handler gin.HandlerFunc, middleware []gin.HandlerFunc) {
		a.router.Handle(method, path, append(middleware, handler)...)
	})
}
