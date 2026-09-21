# go-controller v2

A controller registration and dependency injection library for existing Go web applications.

Create your Gin, Echo, or Fiber router, describe your modules, then call `gocontroller.Mount`. Routes are registered directly on that router. Your handlers receive its native context, and the framework owns middleware execution, binding, responses, errors, and server lifecycle.

**Version 2.0.0** uses the `/v2` module path and requires Go 1.25 or newer. Existing v1 tags retain the previous standalone framework. See [migration notes](MIGRATING.md).

## Quick start: Gin

Install the release:

```sh
go get github.com/emmybxt/go-controller/v2@v2.0.0
```

```go
package main

import (
    "log"
    "net/http"

    ginadapter "github.com/emmybxt/go-controller/v2/adapters/gin"
    "github.com/emmybxt/go-controller/v2/gocontroller"
    "github.com/gin-gonic/gin"
)

type BookService struct{}

func NewBookService() *BookService { return &BookService{} }
func (s *BookService) Title(id string) string { return "Book " + id }

type BookController struct { service *BookService }

func NewBookController(service *BookService) *BookController {
    return &BookController{service: service}
}

func (c *BookController) ControllerMetadata() gocontroller.ControllerMetadata {
    return gocontroller.ControllerMetadata{
        Prefix: "/books",
        Routes: []gocontroller.Route{
            gocontroller.GET("/:id", c.Get),
        },
    }
}

func (c *BookController) Get(ctx *gin.Context) {
    ctx.JSON(http.StatusOK, gin.H{
        "title": c.service.Title(ctx.Param("id")),
    })
}

func main() {
    router := gin.Default()
    err := gocontroller.Mount(ginadapter.New(router), &gocontroller.Module{
        Name:        "LibraryModule",
        Prefix:      "/api",
        Providers:   []any{NewBookService},
        Controllers: []any{NewBookController},
    })
    if err != nil {
        log.Fatal(err)
    }
    log.Fatal(router.Run(":8080"))
}
```

`GET /api/books/1` now runs through Gin. You can also register ordinary routes before or after mounting and mount into a native router group: `ginadapter.New(router.Group("/v1"))`.

## Framework adapters

Import the adapter that matches your framework's major version. All adapter paths below are relative to `github.com/emmybxt/go-controller/v2/`.

| Adapter | Tested framework | Native handler | Native middleware |
| --- | --- | --- | --- |
| `adapters/gin` | Gin 1.12.0 | `func(*gin.Context)` | `gin.HandlerFunc` |
| `adapters/echo` | Echo 5.3.1 | `func(*echo.Context) error` | `echo.MiddlewareFunc` |
| `adapters/echo-v4` | Echo 4.15.4 | `func(echo.Context) error` | `echo.MiddlewareFunc` |
| `adapters/fiber` | Fiber 3.5.0 | `func(fiber.Ctx) error` | `fiber.Handler` |
| `adapters/fiber-v2` | Fiber 2.52.15 | `func(*fiber.Ctx) error` | `fiber.Handler` |

Each adapter accepts the native engine/app or a native group. Named function types with the same underlying native function signature are supported. Fiber v3's additional compatibility handler shapes are deliberately outside this adapter's interface; use its native `fiber.Handler` signature.

The module configuration is the same across frameworks. Controller methods and middleware must use the selected framework's types; changing adapters alone does not convert Gin handlers into Echo handlers. Services can stay framework independent.

The core `gocontroller` package imports only the standard library. Framework dependencies live in adapter packages; your binary compiles the adapters you import. The repository uses one Go module and one release version for the core and adapters.

Native semantics are documented by [Gin](https://gin-gonic.com/en/docs/routing/grouping-routes/), [Echo v5](https://github.com/labstack/echo/blob/v5.3.1/echo.go), and [Fiber v3](https://docs.gofiber.io/guide/routing/).

## Modules and providers

```go
root := &gocontroller.Module{
    Name:   "AppModule",
    Prefix: "/api",
    Imports: []*gocontroller.Module{
        {
            Name:        "BooksModule",
            Prefix:      "/v1",
            Providers:   []any{NewBookService},
            Controllers: []any{NewBookController},
        },
    },
}
```

Parent, imported-module, controller, and route prefixes compose: `/api` + `/v1` + `/books` + `/:id`. An empty route path addresses the prefix itself; `"/"` requests an explicit trailing slash. Native parameter, wildcard, and optional-segment syntax is passed through. Route matching, redirects, HEAD/OPTIONS behavior, and conflicts with existing host routes follow the host framework.

Provider definitions can be instances, `func(...) T`, or `func(...) (T, error)`. Constructor arguments are resolved recursively. Providers are lazy singletons within one `Mount` call; unused factories are not invoked. Constructors should return a non-nil result and an optional exact `error` return. Variadic constructors are unsupported.

All providers in an import graph share one scope and are registered before any controller is constructed. Provider declaration order does not matter. An exact type registration wins; an interface parameter can also resolve a single assignable concrete provider. Missing dependencies, duplicate provider types, ambiguous interface bindings, and dependency cycles return startup errors. To select an interface implementation explicitly:

```go
Providers: []any{
    NewBookService,
    func(service *BookService) BookReader { return service },
}
```

Importing the same module pointer shares its provider definitions. Its controllers are mounted under each importing path, with a new controller instance for each occurrence. Reusing a route module at the same effective path produces a duplicate-route error. Separate `Mount` calls have separate provider scopes. Use one root module with imports when services should be shared.

`Name` identifies a module in setup errors; it does not inject request values. Provider initialization, cleanup, and concurrency safety remain application responsibilities. There are no automatic lifecycle hooks or request-scoped services.

## Native middleware

Register global middleware on your framework before mounting. Module, controller, and route middleware use native functions:

```go
module.Middleware = []any{AuthMiddleware()}

// Inside ControllerMetadata:
gocontroller.POST("/", c.Create, AuditMiddleware())
```

Execution order is host/global/group → parent module → imported module → controller → route → handler. Native `Next`, abort, and returned-error behavior is preserved. For Echo, middleware has its usual `func(echo.HandlerFunc) echo.HandlerFunc` shape.

`[]any` allows the same module shape across frameworks. All official adapters validate the full batch's handler and middleware types before registration. Passing a handler name string, a nil function, or a function from the wrong framework returns an error at startup.

Call `Mount` before serving requests and stop startup on any error. Provider resolution, duplicate-route checks, and handler/middleware type validation finish before registration. Native registration failures (including invalid route patterns) can occur after earlier routes have been added; discard that router. The library does not inspect or roll back the host's existing route table. Register routes in your intended priority order, particularly with Fiber's ordered routing.

## Optional annotation generation

Keep routes beside native controller methods without manually implementing `ControllerMetadata`:

```go
//go:generate go run github.com/emmybxt/go-controller/v2/cmd/gocontroller-gen -dir . -out routes.gen.go

// @Controller("/books")
type BookController struct { service *BookService }

// @Get("/:id")
// @Use(AuthMiddleware())
func (c *BookController) Get(ctx *gin.Context) { /* native handler */ }
```

Annotation names are case-sensitive: use `@Post`, not `@post`. Run `go generate ./...` and commit the generated `.gen.go` files. The generator emits a `ControllerMetadata()` method with bound references such as `c.Get`; there is no runtime handler-name lookup or global metadata registry. Use either generated or handwritten metadata on a controller, not both.

Supported annotations: `@Controller`, `@Use`, `@Get`, `@Post`, `@Put`, `@Patch`, `@Delete`, `@Head`, `@Options`, `@Connect`, and `@Trace`. Use quoted route paths. `@Use` expressions should resolve within the same package; wrap externally imported middleware in a local helper. Nested function arguments and composite literals are supported.

Discovery handles methods declared in separate files, excludes tests/generated files and files outside the active build constraints, and produces deterministic output. Within each generated controller, static segments sort ahead of parameters, then wildcards. Native framework syntax still determines matching. Generic controllers require handwritten metadata.

CI can check an existing output without rewriting it:

```sh
go run ./cmd/gocontroller-gen -dir ./example -out routes.gen.go -check
```

## Run the examples locally

From this checkout, choose one server:

```sh
go generate ./...
go run ./example        # Gin, generated metadata
go run ./example/echo   # Echo v5, handwritten metadata
go run ./example/fiber  # Fiber v3, handwritten metadata
```

Each listens on port 8080. Request `http://localhost:8080/api/books/1`. The examples share an ordinary Go service without importing a web framework into that service.

## Custom adapters

Implement `gocontroller.Adapter` with `Register([]gocontroller.Route) error`. `Mount` passes fully composed paths, bound handlers, and ordered middleware. Validate the full batch's native function types before registering it. Register those functions directly on your host router without introducing a request dispatcher or context wrapper.

## Verification

```sh
make verify-generated
go test -race ./...
go vet ./...
go build ./...
```

The integration suite exercises all five adapters through real framework requests, including native groups, middleware ordering and aborts, request-local values, JSON binding, host error handling, route coexistence, and startup type rejection.

MIT licensed.
