package adapters_test

import (
	gc "github.com/emmybxt/go-controller/v2/gocontroller"
	echov4 "github.com/labstack/echo/v4"
	echov5 "github.com/labstack/echo/v5"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestBeforeAfterMiddlewareAcrossScopes(t *testing.T) {
	for _, framework := range frameworks {
		t.Run(framework.name, func(t *testing.T) {
			h := framework.setup(t)
			module := &gc.Module{Middleware: []any{gc.UseBefore(h.middleware("module")), gc.UseAfter(h.middleware("module-after"))}, Controllers: []any{&controller{gc.ControllerMetadata{
				Middleware: []any{gc.UseAfter(h.middleware("controller-after"))},
				Routes:     []gc.Route{gc.GET("/:id", h.handler, gc.UseBefore(h.middleware("route")), gc.UseAfter(h.middleware("route-after")))},
			}}}}
			if err := gc.Mount(h.adapter, module); err != nil {
				t.Fatal(err)
			}
			want := []string{"global:before", "group:before", "module:before", "route:before", "handler", "module-after:before", "controller-after:before", "route-after:before", "route-after:after", "controller-after:after", "module-after:after", "route:after", "module:after", "group:after", "global:after"}
			for range 2 {
				*h.events = nil
				assertResponse(t, h, "GET", "/host/42?filter=bar", "", 200, "42|bar|native")
				if !reflect.DeepEqual(*h.events, want) {
					t.Fatalf("got %v, want %v", *h.events, want)
				}
			}
		})
	}
}

func TestAfterMiddlewareRespectsStopsAndErrors(t *testing.T) {
	for _, framework := range frameworks {
		t.Run(framework.name, func(t *testing.T) {
			h := framework.setup(t)
			module := &gc.Module{Controllers: []any{&controller{gc.ControllerMetadata{Routes: []gc.Route{
				gc.GET("/blocked", h.handler, h.abort, gc.UseAfter(h.middleware("after"))),
				gc.GET("/failure", h.failure, gc.UseAfter(h.middleware("after"))),
				gc.GET("/after-failure", h.handler, gc.UseAfter(h.abort, h.middleware("unreachable"))),
			}}}}}
			if err := gc.Mount(h.adapter, module); err != nil {
				t.Fatal(err)
			}
			assertResponse(t, h, "GET", "/host/blocked", "", 401, "blocked")
			for _, event := range *h.events {
				if event == "handler" || event == "after:before" {
					t.Fatalf("ran after denial: %v", *h.events)
				}
			}
			*h.events = nil
			assertResponse(t, h, "GET", "/host/failure", "", 418, "native-error")
			ranAfter := false
			for _, event := range *h.events {
				if event == "after:before" {
					ranAfter = true
				}
			}
			if ranAfter != (framework.name == "gin") {
				t.Fatalf("unexpected error continuation: %v", *h.events)
			}
			// Once a response is committed, after middleware must not replace it. Only
			// inspect continuation here; the host controls status/body write behavior.
			*h.events = nil
			response := h.request(httptest.NewRequest("GET", "/host/after-failure", nil))
			response.Body.Close()
			for _, event := range *h.events {
				if event == "unreachable:before" {
					t.Fatal("after middleware did not stop chain")
				}
			}
		})
	}
}

func TestAfterMiddlewareErrorsReachHostHandler(t *testing.T) {
	for _, framework := range frameworks {
		t.Run(framework.name, func(t *testing.T) {
			h := framework.setup(t)
			// A native middleware-shaped handler continues without writing a response.
			// Echo handlers need a direct no-op because their middleware is a wrapper.
			handler := afterErrorHandler(framework.name, h)
			if err := h.adapter.Register([]gc.Route{gc.GET("/after-error", handler, gc.UseAfter(h.afterFailure))}); err != nil {
				t.Fatal(err)
			}
			assertResponse(t, h, "GET", "/host/after-error", "", 418, "native-error")
		})
	}
}

func afterErrorHandler(name string, h harness) any {
	switch name {
	case "echo-v5":
		return func(*echov5.Context) error { return nil }
	case "echo-v4":
		return func(echov4.Context) error { return nil }
	default:
		return h.middleware("handler")
	}
}
