# Migrating from v1 to v2

V2 is a breaking rewrite of go-controller as a plugin for existing routers. V1 remains available through its original module path and tags. V2.0.0 requires Go 1.25 or newer.

1. Change imports to `github.com/emmybxt/go-controller/v2/gocontroller` and select a native adapter.
2. Create the Gin/Echo/Fiber router in your application's `main` function.
3. Replace `NewApp(module)` with `Mount(adapter.New(router), module)` and check the returned error.
4. Change handlers to the framework's native context and return signature. Use native parsing, validation, responses, and errors.
5. Keep constructor providers and module configuration. Replace `gocontroller.Middleware` with native middleware in `[]any` fields.
6. Use `ControllerMetadata()` with `[]gocontroller.Route` and bound methods (`c.Get`, replacing `"Get"`). For annotated controllers, regenerate with the v2 generator. Remove old generated registry files before generating if they are incompatible with v2 imports.
7. Start and shut down the native server from your application.

## Behavior changes

| V1 behavior | V2 behavior |
| --- | --- |
| `NewApp`, custom Router/RouteGroup, `Handler`, `Listen`, `Run` | Existing framework router plus `Mount` |
| `*gocontroller.Context` | Native Gin/Echo/Fiber context |
| `RegisterRoutes(*RouteGroup)` or string method metadata | `ControllerMetadata()` with bound native handler functions |
| Generated global metadata registry | Generated method on each annotated controller |
| Independent imported-module prefixes | Parent and imported prefixes/middleware compose |
| Implicit provider overwrites and interface selection | Duplicate types and ambiguous interfaces fail startup |
| Container service locator / scoped DI | Constructor injection with mount-scoped singleton providers |
| Automatic provider lifecycle hooks | Application-owned resource initialization and cleanup |
| Generic HTTP wrapping behind a wildcard route | Individual routes registered on the native router |

One native controller does not run unchanged on every framework: Gin and Echo have different handler types. Share framework-independent services, then use controllers and middleware written for the framework you mount into.

## Removed standalone features

V2 removes the custom context/router/server, DTO/validation wrappers, response/error helpers, authentication helpers, JWT, sessions, CSRF, rate limiting, health registry, telemetry, event bus, cron scheduler, idempotency, uploads, pagination helpers, lifecycle manager, and request-scoped container. Use the host framework or focused application libraries for these concerns.

The old `oapi-gen` command is removed. Route annotation generation remains, but OpenAPI generation is outside this library's routing and injection scope. Existing OpenAPI documents are not automatically migrated.

Native framework behavior now governs route matching, middleware execution, error rendering, implicit HEAD/OPTIONS, and body limits. Configure those on the host before serving requests. No v1 security or server settings are silently transferred.

## Install v2.0.0

```sh
go get github.com/emmybxt/go-controller/v2@v2.0.0
```

The single root Go module versions core and adapters together. Update your imports to include `/v2` and regenerate annotated routes before building your application.
