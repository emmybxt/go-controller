package gocontroller

import "fmt"

// RequireContextValue ensures a request context value exists before handler execution.
// Useful for auth checks after upstream middleware populates request context.
func RequireContextValue(key any, unauthorizedMessage string) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			if ctx.Request.Context().Value(key) == nil {
				if unauthorizedMessage == "" {
					unauthorizedMessage = "Unauthorized"
				}
				return UnauthorizedError(unauthorizedMessage)
			}
			return next(ctx)
		}
	}
}

// ContextValue extracts a typed request context value by key.
func ContextValue[T any](ctx *Context, key any) (T, bool) {
	var zero T
	raw := ctx.Request.Context().Value(key)
	if raw == nil {
		return zero, false
	}
	v, ok := raw.(T)
	if !ok {
		return zero, false
	}
	return v, true
}

// MustContextValue extracts a typed request context value by key or returns an unauthorized APIError.
func MustContextValue[T any](ctx *Context, key any, unauthorizedMessage string) (T, error) {
	v, ok := ContextValue[T](ctx, key)
	if ok {
		return v, nil
	}
	if unauthorizedMessage == "" {
		unauthorizedMessage = "Unauthorized"
	}
	var zero T
	return zero, &APIError{
		StatusCode: 401,
		Code:       "unauthorized",
		Message:    unauthorizedMessage,
		Details: map[string]any{
			"key": fmt.Sprintf("%v", key),
		},
	}
}
