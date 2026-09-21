// Package gocontroller wires providers and controllers into an existing router.
// It does not own the HTTP server, request context, or middleware runtime.
package gocontroller

import "strings"

// Controller declares routes using bound native framework handler methods.
// Implement this directly or generate it from @Controller and @Get comments.
type Controller interface {
	ControllerMetadata() ControllerMetadata
}

type ControllerMetadata struct {
	Prefix     string
	Middleware []any
	Routes     []Route
}

// Route carries native handlers and middleware. Their types are checked by the
// selected adapter at mount time, before any routes are registered.
type Route struct {
	Method     string
	Path       string
	Handler    any
	Middleware []any
}

func GET(path string, handler any, middleware ...any) Route {
	return Route{"GET", path, handler, middleware}
}

func POST(path string, handler any, middleware ...any) Route {
	return Route{"POST", path, handler, middleware}
}

func PUT(path string, handler any, middleware ...any) Route {
	return Route{"PUT", path, handler, middleware}
}

func PATCH(path string, handler any, middleware ...any) Route {
	return Route{"PATCH", path, handler, middleware}
}

func DELETE(path string, handler any, middleware ...any) Route {
	return Route{"DELETE", path, handler, middleware}
}

func HEAD(path string, handler any, middleware ...any) Route {
	return Route{"HEAD", path, handler, middleware}
}

func OPTIONS(path string, handler any, middleware ...any) Route {
	return Route{"OPTIONS", path, handler, middleware}
}

func CONNECT(path string, handler any, middleware ...any) Route {
	return Route{"CONNECT", path, handler, middleware}
}

func TRACE(path string, handler any, middleware ...any) Route {
	return Route{"TRACE", path, handler, middleware}
}

// joinPath preserves native route syntax and an explicitly requested trailing
// slash. path.Clean would rewrite patterns owned by the host framework.
func joinPath(prefix, suffix string) string {
	if suffix == "" {
		if prefix == "" {
			return "/"
		}
		return "/" + strings.TrimLeft(prefix, "/")
	}
	return "/" + strings.TrimLeft(strings.TrimRight(prefix, "/")+"/"+strings.TrimLeft(suffix, "/"), "/")
}

func combineMiddleware(parent, child []any) []any {
	combined := make([]any, 0, len(parent)+len(child))
	combined = append(combined, parent...)
	return append(combined, child...)
}
