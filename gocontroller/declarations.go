package gocontroller

import "fmt"

// ControllerDeclaration describes a controller for gocontroller-gen. Configure
// declarations during package initialization, before mounting controllers.
type ControllerDeclaration struct {
	prefix     string
	middleware []any
	routes     []*RouteDeclaration
}

// RouteDeclaration attaches routing information to the following method.
type RouteDeclaration struct{ route Route }

// Controllers declares a prefix for methods marked with var _ = name.GET(...).
func Controllers(prefix string) *ControllerDeclaration {
	return &ControllerDeclaration{prefix: prefix}
}

func (d *ControllerDeclaration) Use(middleware ...any) *ControllerDeclaration {
	d.middleware = append(d.middleware, middleware...)
	return d
}

func (d *ControllerDeclaration) UseBefore(middleware ...any) *ControllerDeclaration {
	return d.Use(middleware...)
}

func (d *ControllerDeclaration) UseAfter(middleware ...any) *ControllerDeclaration {
	return d.Use(UseAfter(middleware...))
}

func (d *ControllerDeclaration) GET(path string, middleware ...any) *RouteDeclaration {
	return d.declare("GET", path, middleware)
}

func (d *ControllerDeclaration) POST(path string, middleware ...any) *RouteDeclaration {
	return d.declare("POST", path, middleware)
}

func (d *ControllerDeclaration) PUT(path string, middleware ...any) *RouteDeclaration {
	return d.declare("PUT", path, middleware)
}

func (d *ControllerDeclaration) PATCH(path string, middleware ...any) *RouteDeclaration {
	return d.declare("PATCH", path, middleware)
}

func (d *ControllerDeclaration) DELETE(path string, middleware ...any) *RouteDeclaration {
	return d.declare("DELETE", path, middleware)
}

func (d *ControllerDeclaration) HEAD(path string, middleware ...any) *RouteDeclaration {
	return d.declare("HEAD", path, middleware)
}

func (d *ControllerDeclaration) OPTIONS(path string, middleware ...any) *RouteDeclaration {
	return d.declare("OPTIONS", path, middleware)
}

func (d *ControllerDeclaration) CONNECT(path string, middleware ...any) *RouteDeclaration {
	return d.declare("CONNECT", path, middleware)
}

func (d *ControllerDeclaration) TRACE(path string, middleware ...any) *RouteDeclaration {
	return d.declare("TRACE", path, middleware)
}

func (d *ControllerDeclaration) declare(method, path string, middleware []any) *RouteDeclaration {
	route := &RouteDeclaration{route: Route{
		Method: method, Path: path, Middleware: append([]any(nil), middleware...),
	}}
	d.routes = append(d.routes, route)
	return route
}

func (r *RouteDeclaration) Use(middleware ...any) *RouteDeclaration {
	r.route.Middleware = append(r.route.Middleware, middleware...)
	return r
}

func (r *RouteDeclaration) UseBefore(middleware ...any) *RouteDeclaration {
	return r.Use(middleware...)
}

func (r *RouteDeclaration) UseAfter(middleware ...any) *RouteDeclaration {
	return r.Use(UseAfter(middleware...))
}

// MustBind is used by generated metadata. It binds native handlers without
// rerunning middleware factories. Stale bindings panic during startup.
func (d *ControllerDeclaration) MustBind(bindings ...Route) ControllerMetadata {
	routes, err := d.bind(bindings)
	if err != nil {
		panic(fmt.Errorf("controller %q declarations: %w; run go generate", d.prefix, err))
	}
	return ControllerMetadata{
		Prefix: d.prefix, Middleware: append([]any(nil), d.middleware...), Routes: routes,
	}
}

func (d *ControllerDeclaration) bind(bindings []Route) ([]Route, error) {
	declarations, err := d.routeIndex()
	if err != nil {
		return nil, err
	}
	if len(bindings) != len(declarations) {
		return nil, fmt.Errorf("generated bindings do not match the declared routes")
	}
	routes := make([]Route, len(bindings))
	for i, binding := range bindings {
		key := binding.Method + " " + binding.Path
		declared, ok := declarations[key]
		if !ok {
			return nil, fmt.Errorf("no matching declaration for %s", key)
		}
		binding.Middleware = combineMiddleware(declared.Middleware, binding.Middleware)
		routes[i] = binding
		delete(declarations, key)
	}
	return routes, nil
}

func (d *ControllerDeclaration) routeIndex() (map[string]Route, error) {
	routes := make(map[string]Route, len(d.routes))
	for _, declaration := range d.routes {
		route := declaration.route
		key := route.Method + " " + route.Path
		if _, exists := routes[key]; exists {
			return nil, fmt.Errorf("duplicate declaration for %s", key)
		}
		routes[key] = route
	}
	return routes, nil
}
