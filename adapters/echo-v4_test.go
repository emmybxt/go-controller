package adapters_test

import (
	"testing"

	echoadapter "github.com/emmybxt/go-controller/v2/adapters/echo-v4"
	"github.com/labstack/echo/v4"
)

func setupEchoV4(t *testing.T) harness {
	t.Helper()
	app := echo.New()
	var events []string
	middleware := func(name string) echo.MiddlewareFunc {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				events = append(events, name+":before")
				err := next(c)
				events = append(events, name+":after")
				return err
			}
		}
	}
	defaultErrorHandler := app.HTTPErrorHandler
	app.HTTPErrorHandler = func(err error, c echo.Context) {
		if err == nativeFailure {
			_ = c.String(418, "native-error")
			return
		}
		defaultErrorHandler(err, c)
	}
	app.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("request-id", "native"); return next(c) }
	}, middleware("global"))
	app.GET("/plain", func(c echo.Context) error { return c.String(200, "plain") })
	group := app.Group("/host", middleware("group"))
	return harness{
		adapter: echoadapter.New(group), request: httpRequester(app), events: &events,
		middleware: func(name string) any { return middleware(name) },
		handler: func(c echo.Context) error {
			events = append(events, "handler")
			return c.String(200, c.Param("id")+"|"+c.QueryParam("filter")+"|"+c.Get("request-id").(string))
		},
		create: func(c echo.Context) error {
			var body struct {
				Title string `json:"title"`
			}
			if err := c.Bind(&body); err != nil {
				return err
			}
			return c.String(201, body.Title)
		},
		afterFailure: func(echo.HandlerFunc) echo.HandlerFunc { return func(echo.Context) error { return nativeFailure } },
		failure:      func(echo.Context) error { return nativeFailure },
		abort: echo.MiddlewareFunc(func(echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error { return c.String(401, "blocked") }
		}),
	}
}
