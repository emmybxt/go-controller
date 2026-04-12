package gocontroller

import (
	"log"
	"time"
)

// RequestLogger logs method, path, status time around each request.
func RequestLogger() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			start := time.Now()
			err := next(ctx)
			module := "gocontroller"
			if v, ok := ctx.Get(moduleNameContextKey); ok {
				if name, ok := v.(string); ok && name != "" {
					module = name
				}
			}
			log.Printf("[gocontroller][%s] %s %s (%s)", module, ctx.Request.Method, ctx.Request.URL.Path, time.Since(start))
			return err
		}
	}
}
