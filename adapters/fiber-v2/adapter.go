// Package fiberv2adapter mounts controllers directly on Fiber v2 apps and groups.
package fiberv2adapter

import (
	"fmt"

	"github.com/emmybxt/go-controller/v2/gocontroller"
	"github.com/emmybxt/go-controller/v2/internal/native"
	"github.com/gofiber/fiber/v2"
)

type adapter struct{ router fiber.Router }

func New(router fiber.Router) gocontroller.Adapter { return &adapter{router: router} }

func (a *adapter) Register(routes []gocontroller.Route) error {
	if native.IsNil(a.router) {
		return fmt.Errorf("fiber v2 router is nil")
	}
	return native.Register(routes, func(method, path string, handler fiber.Handler, middleware, after []fiber.Handler) {
		a.router.Add(method, path, append(middleware, withAfter(handler, after)...)...)
	})
}
