package adapters_test

import (
	gc "github.com/emmybxt/go-controller/v2/gocontroller"
	"net/http"
	"testing"

	fiberadapter "github.com/emmybxt/go-controller/v2/adapters/fiber"
	"github.com/gofiber/fiber/v3"
)

func setupFiber(t *testing.T) harness {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: func(c fiber.Ctx, err error) error {
		if err == nativeFailure {
			return c.Status(418).SendString("native-error")
		}
		return fiber.DefaultErrorHandler(c, err)
	}})
	var events []string
	middleware := func(name string) fiber.Handler {
		return func(c fiber.Ctx) error {
			events = append(events, name+":before")
			err := c.Next()
			events = append(events, name+":after")
			return err
		}
	}
	app.Use(func(c fiber.Ctx) error { c.Locals("request-id", "native"); return c.Next() }, middleware("global"))
	app.Get("/plain", func(c fiber.Ctx) error { return c.SendString("plain") })
	group := app.Group("/host", middleware("group"))
	return harness{
		adapter: fiberadapter.New(group), events: &events,
		request: func(req *http.Request) *http.Response {
			response, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			return response
		},
		middleware: func(name string) any { return middleware(name) },
		handler: func(c fiber.Ctx) error {
			events = append(events, "handler")
			return c.SendString(c.Params("id") + "|" + c.Query("filter") + "|" + c.Locals("request-id").(string))
		},
		create: func(c fiber.Ctx) error {
			var body struct {
				Title string `json:"title"`
			}
			if err := c.Bind().Body(&body); err != nil {
				return err
			}
			return c.Status(201).SendString(body.Title)
		},
		afterFailure: func(fiber.Ctx) error { return nativeFailure },
		failure:      func(fiber.Ctx) error { return nativeFailure },
		abort:        func(c fiber.Ctx) error { return c.Status(401).SendString("blocked") },
	}
}

func TestFiberAfterRunsOnceWithExplicitNext(t *testing.T) {
	h := setupFiber(t)
	handler := func(c fiber.Ctx) error {
		*h.events = append(*h.events, "handler")
		if err := c.SendString("ok"); err != nil {
			return err
		}
		return c.Next()
	}
	if err := h.adapter.Register([]gc.Route{gc.GET("/next", handler, gc.UseAfter(h.middleware("after")))}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		*h.events = nil
		assertResponse(t, h, "GET", "/host/next", "", 200, "ok")
		count := 0
		for _, event := range *h.events {
			if event == "after:before" {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("after ran %d times: %v", count, *h.events)
		}
	}
}
