package gocontroller

import "net/http"

// AdaptHTTPMiddleware adapts a standard net/http middleware into gocontroller middleware.
func AdaptHTTPMiddleware(httpMW func(http.Handler) http.Handler) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			called := false
			var nextErr error

			bridge := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				ctx.Request = r
				nextErr = next(ctx)
			})

			httpMW(bridge).ServeHTTP(ctx.ResponseWriter, ctx.Request)
			if !called {
				return nil
			}
			return nextErr
		}
	}
}
