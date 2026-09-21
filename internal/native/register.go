// Package native validates and converts native functions once, at startup.
package native

import (
	"fmt"
	"reflect"

	"github.com/emmybxt/go-controller/v2/gocontroller"
)

type preparedRoute[H, M any] struct {
	route      gocontroller.Route
	handler    H
	middleware []M
	after      []M
}

// Register validates the whole batch before calling add. No request-time
// reflection or context conversion is involved.
func Register[H, M any](routes []gocontroller.Route, add func(string, string, H, []M, []M)) error {
	prepared := make([]preparedRoute[H, M], len(routes))
	for i, route := range routes {
		item, err := prepare[H, M](route)
		if err != nil {
			return fmt.Errorf("%s %s: %w", route.Method, route.Path, err)
		}
		prepared[i] = item
	}
	for _, item := range prepared {
		if err := registerOne(item, add); err != nil {
			return err
		}
	}
	return nil
}

func prepare[H, M any](route gocontroller.Route) (preparedRoute[H, M], error) {
	item := preparedRoute[H, M]{route: route}
	handler, err := convert[H](route.Handler)
	if err != nil {
		return item, fmt.Errorf("handler: %w", err)
	}
	item.handler = handler
	before, after := splitMiddleware(route.Middleware, false)
	item.middleware, err = convertMiddleware[M](before)
	if err != nil {
		return item, err
	}
	item.after, err = convertMiddleware[M](after)
	if err != nil {
		return item, fmt.Errorf("after %w", err)
	}
	return item, nil
}

func convert[T any](value any) (T, error) {
	var zero T
	v := reflect.ValueOf(value)
	t := reflect.TypeFor[T]()
	if !v.IsValid() || v.Kind() != reflect.Func {
		return zero, fmt.Errorf("expected %s, got %T", t, value)
	}
	if v.IsNil() || !v.Type().ConvertibleTo(t) {
		return zero, fmt.Errorf("expected non-nil %s, got %T", t, value)
	}
	return v.Convert(t).Interface().(T), nil
}

func registerOne[H, M any](item preparedRoute[H, M], add func(string, string, H, []M, []M)) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("register %s %s: %v", item.route.Method, item.route.Path, recovered)
		}
	}()
	add(item.route.Method, item.route.Path, item.handler, item.middleware, item.after)
	return nil
}

func splitMiddleware(values []any, downstream bool) (before, after []any) {
	for _, value := range values {
		if group, ok := value.(gocontroller.MiddlewareGroup); ok {
			b, a := splitMiddleware(group.Before, downstream)
			before = append(before, b...)
			after = append(after, a...)
			_, a = splitMiddleware(group.After, true)
			after = append(after, a...)
		} else if downstream {
			after = append(after, value)
		} else {
			before = append(before, value)
		}
	}
	return before, after
}

func convertMiddleware[M any](values []any) ([]M, error) {
	result := make([]M, len(values))
	for i, value := range values {
		converted, err := convert[M](value)
		if err != nil {
			return nil, fmt.Errorf("middleware %d: %w", i+1, err)
		}
		result[i] = converted
	}
	return result, nil
}
