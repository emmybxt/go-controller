// Package fiberadapter mounts controllers directly on Fiber v3 apps and groups.
package fiberadapter

import (
	"fmt"

	"github.com/emmybxt/go-controller/v2/gocontroller"
	"github.com/emmybxt/go-controller/v2/internal/native"
	"github.com/gofiber/fiber/v3"
)

type adapter struct{ router fiber.Router }

func New(router fiber.Router) gocontroller.Adapter { return &adapter{router: router} }

func (a *adapter) Register(routes []gocontroller.Route) error {
	if native.IsNil(a.router) {
		return fmt.Errorf("fiber router is nil")
	}
	return native.Register(routes, func(method, path string, handler fiber.Handler, middleware []fiber.Handler) {
		handlers := make([]any, 0, len(middleware)+1)
		for _, middleware := range middleware {
			handlers = append(handlers, middleware)
		}
		handlers = append(handlers, handler)
		a.router.Add([]string{method}, path, handlers[0], handlers[1:]...)
	})
}
