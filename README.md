# gocontroller

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/emmybxt/go-controller/blob/master/LICENSE)

`gocontroller` is a lightweight Go library for building APIs with a familiar controller/module pattern inspired by NestJS and Express, while staying idiomatic Go.

It gives you:

- Controller-oriented route organization
- Route, group, and module middleware composition
- Nest-style module graph (`imports`, `providers`, `controllers`)
- Reflection-based dependency injection
- DTO-style request parsing and validation
- Annotation-driven route metadata with `go generate`
- `net/http` compatibility so you can mount into Gin, Echo, Fiber adapters

## Why this library exists

Go frameworks are powerful, but many teams want a predictable architecture where:

- routes are declared near controller methods
- dependencies are constructor-injected
- feature modules are explicit
- request DTOs are validated consistently

`gocontroller` focuses on architecture and composition so your business code stays clean.

## Installation

```bash
go get github.com/emmybxt/go-controller
```

## Quick Start (5 minutes)

```go
package main

import (
    "net/http"

    "github.com/emmybxt/go-controller/gocontroller"
)

type HealthController struct{}

func (c *HealthController) RegisterRoutes(r *gocontroller.RouteGroup) {
    r.GET("/health", c.Health)
}

func (c *HealthController) Health(ctx *gocontroller.Context) error {
    return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func NewHealthController() *HealthController { return &HealthController{} }

func main() {
    app, err := gocontroller.NewApp(&gocontroller.Module{
        Name:        "AppModule",
        Prefix:      "/api",
        Controllers: []any{NewHealthController},
    })
    if err != nil {
        panic(err)
    }

    _ = app.Listen(":8080")
}
```

## Core Concepts

### 1) Controllers

Two supported styles:

1. Classic interface style:

```go
func (c *UserController) RegisterRoutes(r *gocontroller.RouteGroup) {
    r.GET("/users/:id", c.GetByID)
    r.POST("/users", c.Create, AuthMiddleware())
}
```

2. Metadata style:

```go
func (c *UserController) ControllerMetadata() gocontroller.ControllerMetadata {
    return gocontroller.ControllerMetadata{
        Prefix: "/users",
        Routes: []gocontroller.RouteMetadata{
            gocontroller.GET("/:id", "GetByID"),
            gocontroller.POST("/", "Create", AuthMiddleware()),
        },
    }
}
```

### 2) Modules

Modules let you group providers/controllers and compose feature boundaries.

```go
userModule := &gocontroller.Module{
    Name:        "UserModule",
    Prefix:      "/users",
    Providers:   []any{NewUserService, NewUserRepo},
    Controllers: []any{NewUserController},
}

appModule := &gocontroller.Module{
    Name:    "AppModule",
    Prefix:  "/api",
    Imports: []*gocontroller.Module{userModule},
}
```

### 3) Dependency Injection

Register providers as:

- concrete instances
- constructor functions returning `T`
- constructor functions returning `(T, error)`

Dependencies are resolved recursively from constructor parameters.

```go
func NewUserService(repo *UserRepo) *UserService { ... }
func NewUserController(svc *UserService) *UserController { ... }
```

### 4) DTO Validation

Use `ParseDTO[T]` or `ctx.BindJSON(&dto)`.

```go
type CreateUserDTO struct {
    Name  string `json:"name" validate:"required,min=2,max=50"`
    Email string `json:"email" validate:"required,email"`
}

func (c *UserController) Create(ctx *gocontroller.Context) error {
    dto, err := gocontroller.ParseDTO[CreateUserDTO](ctx)
    if err != nil {
        return err // handled as 400 when validation fails
    }
    return ctx.JSON(http.StatusCreated, dto)
}
```

Validation is powered by `go-playground/validator/v10`, so you can use its broad built-in tag set (for the pinned version in this module), including tags like:

- `required`, `min`, `max`, `len`
- `email`, `url`, `uri`, `hostname`
- `uuid`, `uuid4`, `ip`, `ipv4`, `ipv6`
- `oneof`, `startswith`, `endswith`, `contains`
- `gt`, `gte`, `lt`, `lte`
- `datetime`
- `dive` for slices/maps

You can combine tags exactly as in validator syntax, e.g. `validate:"required,oneof=admin user,lowercase"`.

### 4.1) Pluggable Validator Engine

Validation is fully swappable via `gocontroller.Validator`.

```go
type Validator interface {
    Validate(any) error
}
```

Per-app override:

```go
app.SetValidator(myValidator)
```

Global default override:

```go
gocontroller.SetDefaultValidator(myValidator)
```

Function adapter:

```go
app.SetValidator(gocontroller.ValidatorFunc(func(v any) error {
    // call ozzo/json-schema/custom rules
    return nil
}))
```

Default engine is `go-playground/validator/v10` wrapped by `NewGoPlaygroundValidator()`.

### 5) Middleware

Attach middleware at multiple levels:

- app/router (global)
- module
- route group
- route

```go
r.POST("/users", c.Create, AuthMiddleware(), AuditMiddleware())
```

Middleware signature:

```go
type Middleware func(HandlerFunc) HandlerFunc
```

Built-in helpers:

- `gocontroller.RequestLogger()`
- `gocontroller.AdaptHTTPMiddleware(func(http.Handler) http.Handler)`

## Annotation + Codegen (Decorator-like)

If you prefer Nest-like annotations, use comments + generator.

### Step 1: Annotate

```go
// @Controller("/users")
type UserController struct{}

// @Get("/:id")
func (c *UserController) GetByID(ctx *gocontroller.Context) error { return nil }

// @Post("/")
// @Use(AuthMiddleware())
func (c *UserController) Create(ctx *gocontroller.Context) error { return nil }
```

### Step 2: Add `go:generate`

```go
//go:generate go run ../cmd/gocontroller-gen -dir . -out zz_gocontroller_routes.gen.go
```

### Step 3: Generate

```bash
go generate ./example
```

Generated metadata is auto-registered through `init()` and picked up by the module loader.

### Avoid "forgot to generate" in deployments

`go build` does not run `go generate` automatically in Go.

Use build wrappers that always generate first:

```bash
make build   # runs go generate ./... then go build ./...
make test    # runs go generate ./... then go test ./...
```

And enforce freshness in CI:

```bash
make verify-generated
```

## Framework Compatibility (Gin / Echo / Fiber)

Yes, it can be used with those frameworks.

`gocontroller` exposes `App.Handler() http.Handler`, so you can mount it where wrappers are available.

### Gin

```go
import "github.com/gin-gonic/gin"

ginEngine := gin.Default()
ginEngine.Any("/api/*any", gin.WrapH(app.Handler()))
```

### Echo

```go
import "github.com/labstack/echo/v4"

e := echo.New()
e.Any("/api/*", echo.WrapHandler(app.Handler()))
```

### Fiber

```go
import (
    "github.com/gofiber/adaptor/v2"
    "github.com/gofiber/fiber/v2"
)

f := fiber.New()
f.All("/api/*", adaptor.HTTPHandler(app.Handler()))
```

Notes:

- This keeps your controller/module architecture in one place.
- If you need deep native middleware/context features of each framework, use adapters selectively at the boundary.

## Web + API Composition Helpers

You can avoid manual `finalHandler` path-switch logic with:

- `gocontroller.WebAPIHandler(webHandler, apiHandler, opts)`
- `gocontroller.NotFoundHTMLOrJSON(html404Path, jsonMessage)`
- `gocontroller.ServePage(publicDir, pageFile)`

Example:

```go
final := gocontroller.WebAPIHandler(webRouter, app.Handler(), gocontroller.HybridOptions{
    WebExactPaths:              []string{"/"},
    WebPathPrefixes:            []string{"/app", "/css/", "/js/"},
    TreatSingleSegmentGETAsWeb: true,
})
```

## Context Response Helpers

Built-in response shortcuts on `*gocontroller.Context`:

- `ctx.OK(data)`
- `ctx.Created(data)`
- `ctx.NoContent()`
- `ctx.BadRequest(msg)`
- `ctx.Unauthorized(msg)`
- `ctx.Forbidden(msg)`
- `ctx.NotFound(msg)`
- `ctx.Conflict(msg)`
- `ctx.InternalError(msg)`
- `ctx.Success(status, data)` and `ctx.Fail(status, msg)` for envelope style

## API Surface

Main types/functions:

- `gocontroller.NewApp(*Module)`
- `(*App).Listen(addr)`
- `(*App).Handler()`
- `(*App).SetValidator(v)` / `(*App).Validator()`
- `Module{ Name, Prefix, Providers, Controllers, Imports, Middleware }`
- `RouteGroup.GET/POST/PUT/DELETE`
- `ParseDTO[T](ctx)`
- `NewHTTPError(status, message)`
- `ControllerMetadata`, `RouteMetadata`
- `GET/POST/PUT/DELETE` metadata helpers
- `RegisterGeneratedControllerMetadata` (used by generated code)
- `Validator`, `ValidatorFunc`, `SetDefaultValidator`, `DefaultValidator`

## Error Handling Behavior

Default behavior:

- route not found: `404`
- validation error: `400`
- explicit `NewHTTPError(...)`: mapped status
- unknown handler error: `500`

## Public Library Checklist

Before publishing:

1. Update module path in `go.mod` to your GitHub repo.
2. Add semantic tags (`v0.1.0`, `v0.2.0`, etc.).
3. Add CI (`go test ./...`, `go vet ./...`).
4. Add changelog and license.
5. Add examples for both classic and annotation styles.

## License

This project is licensed under the MIT License. See [LICENSE](https://github.com/emmybxt/go-controller/blob/master/LICENSE).
