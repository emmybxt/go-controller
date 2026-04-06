package gocontroller

import (
	"fmt"
	"reflect"
	"strings"
)

// DecoratedController declares controller metadata for auto route binding.
type DecoratedController interface {
	ControllerMetadata() ControllerMetadata
}

// ControllerMetadata is decorator-like route metadata for a controller.
type ControllerMetadata struct {
	Prefix     string
	Middleware []Middleware
	Routes     []RouteMetadata
}

// RouteMetadata describes a single route in a controller.
type RouteMetadata struct {
	Method     string
	Path       string
	Handler    string
	Middleware []Middleware
}

func GET(path, handler string, middleware ...Middleware) RouteMetadata {
	return RouteMetadata{Method: "GET", Path: path, Handler: handler, Middleware: middleware}
}

func POST(path, handler string, middleware ...Middleware) RouteMetadata {
	return RouteMetadata{Method: "POST", Path: path, Handler: handler, Middleware: middleware}
}

func PUT(path, handler string, middleware ...Middleware) RouteMetadata {
	return RouteMetadata{Method: "PUT", Path: path, Handler: handler, Middleware: middleware}
}

func DELETE(path, handler string, middleware ...Middleware) RouteMetadata {
	return RouteMetadata{Method: "DELETE", Path: path, Handler: handler, Middleware: middleware}
}

func registerDecoratedController(group *RouteGroup, instance any, meta ControllerMetadata) error {
	controllerGroup := group.Group(meta.Prefix, meta.Middleware...)
	for _, route := range meta.Routes {
		handler, err := bindControllerMethod(instance, route.Handler)
		if err != nil {
			return err
		}

		switch strings.ToUpper(route.Method) {
		case "GET":
			controllerGroup.GET(route.Path, handler, route.Middleware...)
		case "POST":
			controllerGroup.POST(route.Path, handler, route.Middleware...)
		case "PUT":
			controllerGroup.PUT(route.Path, handler, route.Middleware...)
		case "DELETE":
			controllerGroup.DELETE(route.Path, handler, route.Middleware...)
		default:
			return fmt.Errorf("unsupported route method %q for path %q", route.Method, route.Path)
		}
	}
	return nil
}

func bindControllerMethod(instance any, methodName string) (HandlerFunc, error) {
	if methodName == "" {
		return nil, fmt.Errorf("handler method name cannot be empty")
	}

	method := reflect.ValueOf(instance).MethodByName(methodName)
	if !method.IsValid() {
		return nil, fmt.Errorf("method %q not found on controller", methodName)
	}

	t := method.Type()
	if t.NumIn() != 1 || t.In(0) != reflect.TypeOf(&Context{}) {
		return nil, fmt.Errorf("method %q must have signature func(*gocontroller.Context) error", methodName)
	}
	if t.NumOut() != 1 || !t.Out(0).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return nil, fmt.Errorf("method %q must have signature func(*gocontroller.Context) error", methodName)
	}

	return func(ctx *Context) error {
		result := method.Call([]reflect.Value{reflect.ValueOf(ctx)})
		if result[0].IsNil() {
			return nil
		}
		return result[0].Interface().(error)
	}, nil
}
