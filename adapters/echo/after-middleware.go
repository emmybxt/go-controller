package echoadapter

import "github.com/labstack/echo/v5"

func withAfter(handler echo.HandlerFunc, middleware []echo.MiddlewareFunc) echo.HandlerFunc {
	if len(middleware) == 0 {
		return handler
	}
	var next echo.HandlerFunc = func(*echo.Context) error { return nil }
	for i := len(middleware) - 1; i >= 0; i-- {
		next = middleware[i](next)
	}
	return func(c *echo.Context) error {
		if err := handler(c); err != nil {
			return err
		}
		return next(c)
	}
}
