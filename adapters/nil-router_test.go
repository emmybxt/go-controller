package adapters_test

import (
	"testing"

	echoadapter "github.com/emmybxt/go-controller/v2/adapters/echo"
	echov4adapter "github.com/emmybxt/go-controller/v2/adapters/echo-v4"
	fiberadapter "github.com/emmybxt/go-controller/v2/adapters/fiber"
	fiberv2adapter "github.com/emmybxt/go-controller/v2/adapters/fiber-v2"
	ginadapter "github.com/emmybxt/go-controller/v2/adapters/gin"
	gc "github.com/emmybxt/go-controller/v2/gocontroller"
	"github.com/gin-gonic/gin"
	fiberv2 "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v3"
	echov4 "github.com/labstack/echo/v4"
	"github.com/labstack/echo/v5"
)

func TestAdaptersRejectNilRouters(t *testing.T) {
	for _, adapter := range []gc.Adapter{
		ginadapter.New(nil), ginadapter.New((*gin.Engine)(nil)),
		echoadapter.New(nil), echoadapter.New((*echo.Echo)(nil)),
		echov4adapter.New(nil), echov4adapter.New((*echov4.Echo)(nil)),
		fiberadapter.New(nil), fiberadapter.New((*fiber.App)(nil)),
		fiberv2adapter.New(nil), fiberv2adapter.New((*fiberv2.App)(nil)),
	} {
		if err := gc.Mount(adapter, &gc.Module{}); err == nil {
			t.Fatalf("%T accepted nil router", adapter)
		}
	}
}
