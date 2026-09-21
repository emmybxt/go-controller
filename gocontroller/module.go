package gocontroller

import (
	"fmt"
	"reflect"
	"strings"
)

// Module groups controllers and providers. Imported prefixes and middleware
// nest beneath their parent. Providers are shared singletons within one Mount.
type Module struct {
	Name        string
	Prefix      string
	Providers   []any
	Controllers []any
	Imports     []*Module
	Middleware  []any
}

// Adapter registers a complete batch of routes on a host router. Implementations
// must validate all handler and middleware types before registering any routes.
// Routing, request execution, and errors remain the host framework's concern.
type Adapter interface {
	Register([]Route) error
}

// Mount resolves constructors, composes route metadata, and registers the result
// on an existing router. Call it before serving requests. A native registration
// failure can leave earlier routes mounted; discard the router on any error.
func Mount(adapter Adapter, root *Module) error {
	if isNil(reflect.ValueOf(adapter)) {
		return fmt.Errorf("adapter is nil")
	}
	c := newContainer()
	if err := collectProviders(root, c, make(map[*Module]bool), make(map[*Module]bool)); err != nil {
		return err
	}
	routes, err := collectRoutes(root, c, "", nil)
	if err != nil {
		return err
	}
	if err := validateRoutes(routes); err != nil {
		return err
	}
	return adapter.Register(routes)
}

func collectProviders(mod *Module, c *container, visiting, visited map[*Module]bool) error {
	if mod == nil {
		return fmt.Errorf("module is nil")
	}
	if visiting[mod] {
		return fmt.Errorf("circular module import at %q", mod.Name)
	}
	if visited[mod] {
		return nil
	}
	visiting[mod] = true
	for _, imported := range mod.Imports {
		if err := collectProviders(imported, c, visiting, visited); err != nil {
			return fmt.Errorf("module %q import: %w", mod.Name, err)
		}
	}
	for _, def := range mod.Providers {
		if err := c.provide(def); err != nil {
			return fmt.Errorf("module %q provider: %w", mod.Name, err)
		}
	}
	delete(visiting, mod)
	visited[mod] = true
	return nil
}

func collectRoutes(mod *Module, c *container, prefix string, middleware []any) ([]Route, error) {
	prefix = joinPath(prefix, mod.Prefix)
	middleware = combineMiddleware(middleware, mod.Middleware)
	var routes []Route
	for _, def := range mod.Controllers {
		controller, err := c.controller(def)
		if err != nil {
			return nil, fmt.Errorf("module %q controller: %w", mod.Name, err)
		}
		routes = append(routes, controllerRoutes(controller.ControllerMetadata(), prefix, middleware)...)
	}
	for _, imported := range mod.Imports {
		childRoutes, err := collectRoutes(imported, c, prefix, middleware)
		if err != nil {
			return nil, err
		}
		routes = append(routes, childRoutes...)
	}
	return routes, nil
}

func controllerRoutes(meta ControllerMetadata, prefix string, middleware []any) []Route {
	prefix = joinPath(prefix, meta.Prefix)
	middleware = combineMiddleware(middleware, meta.Middleware)
	routes := make([]Route, len(meta.Routes))
	for i, route := range meta.Routes {
		route.Path = joinPath(prefix, route.Path)
		route.Method = strings.ToUpper(route.Method)
		route.Middleware = combineMiddleware(middleware, route.Middleware)
		routes[i] = route
	}
	return routes
}

func validateRoutes(routes []Route) error {
	seen := make(map[string]bool)
	for _, route := range routes {
		if !validMethod(route.Method) {
			return fmt.Errorf("invalid HTTP method %q for %s", route.Method, route.Path)
		}
		key := route.Method + " " + route.Path
		if seen[key] {
			return fmt.Errorf("duplicate route %s", key)
		}
		seen[key] = true
	}
	return nil
}

func validMethod(method string) bool {
	if method == "" {
		return false
	}
	for _, ch := range method {
		if ch < 'A' || ch > 'Z' {
			return false
		}
	}
	return true
}
