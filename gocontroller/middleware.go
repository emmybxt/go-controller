package gocontroller

import "errors"

var ErrNotFound = errors.New("route not found")

type HandlerFunc func(*Context) error

type Middleware func(HandlerFunc) HandlerFunc

func chain(handler HandlerFunc, middleware []Middleware) HandlerFunc {
	wrapped := handler
	for i := len(middleware) - 1; i >= 0; i-- {
		wrapped = middleware[i](wrapped)
	}
	return wrapped
}
