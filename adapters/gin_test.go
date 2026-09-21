package adapters_test

import (
	"net/http"
	"strings"
	"testing"

	ginadapter "github.com/emmybxt/go-controller/v2/adapters/gin"
	gc "github.com/emmybxt/go-controller/v2/gocontroller"
	"github.com/gin-gonic/gin"
)

func setupGin(t *testing.T) harness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	app := gin.New()
	var events []string
	middleware := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) {
			events = append(events, name+":before")
			c.Next()
			events = append(events, name+":after")
		}
	}
	app.Use(func(c *gin.Context) {
		c.Set("request-id", "native")
		c.Next()
		if len(c.Errors) > 0 {
			c.String(418, "native-error")
		}
	}, middleware("global"))
	app.GET("/plain", func(c *gin.Context) { c.String(200, "plain") })
	group := app.Group("/host", middleware("group"))
	return harness{
		adapter: ginadapter.New(group), request: httpRequester(app), events: &events,
		middleware: func(name string) any { return middleware(name) },
		handler: func(c *gin.Context) {
			events = append(events, "handler")
			c.String(200, "%s|%s|%s", c.Param("id"), c.Query("filter"), c.GetString("request-id"))
		},
		create: func(c *gin.Context) {
			var body struct {
				Title string `json:"title"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.AbortWithStatus(400)
				return
			}
			c.String(201, "%s", body.Title)
		},
		afterFailure: func(c *gin.Context) { _ = c.Error(nativeFailure) },
		failure:      func(c *gin.Context) { _ = c.Error(nativeFailure) },
		abort:        func(c *gin.Context) { c.String(401, "blocked"); c.Abort() },
	}
}

func TestGinRegistrationConflictsReturnErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := gin.New()
	app.GET("/existing", func(*gin.Context) {})
	module := &gc.Module{Controllers: []any{&controller{gc.ControllerMetadata{Routes: []gc.Route{gc.GET("/existing", func(*gin.Context) {})}}}}}
	err := gc.Mount(ginadapter.New(app), module)
	if err == nil || !strings.Contains(err.Error(), "GET /existing") {
		t.Fatalf("got %v", err)
	}
}

func TestGinPreservesWildcardsAndNativeRouteTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := gin.New()
	module := &gc.Module{Prefix: "/api", Controllers: []any{&controller{gc.ControllerMetadata{Routes: []gc.Route{
		gc.GET("/files/*path", func(c *gin.Context) { c.String(200, "%s", c.Param("path")) }),
		gc.HEAD("/ping", func(c *gin.Context) { c.Status(http.StatusNoContent) }),
	}}}}}
	if err := gc.Mount(ginadapter.New(app), module); err != nil {
		t.Fatal(err)
	}
	if len(app.Routes()) != 2 {
		t.Fatal(app.Routes())
	}
	assertResponse(t, harness{request: httpRequester(app)}, "GET", "/api/files/a/b", "", 200, "/a/b")
	assertResponse(t, harness{request: httpRequester(app)}, "HEAD", "/api/ping", "", 204, "")
}
