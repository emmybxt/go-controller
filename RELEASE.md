# Release Notes — v1.4.0

> NestJS-inspired HTTP framework for Go with controllers, modules, DI, and 30+ built-in features.

## New Features

### Security & Authentication
- **JWT middleware** — HS256/RS256 verification, `JWTClaims` helpers, `RequireJWT` guard
- **CSRF protection** — HMAC-signed, masked double-submit tokens, configurable ignore methods
- **Request signature verification** — HMAC-SHA256 webhook-style signing with timestamp replay protection
- **Encrypted cookie sessions** — AES-GCM encrypted sessions with `GetSession`, `RegenerateSession`, `DestroySession`

### Observability
- **OpenTelemetry-compatible tracing** — `Tracing()` middleware with `Span`/`Tracer` interfaces, `NoOp` defaults
- **Event bus / pub-sub** — `On`/`Once`/`Emit`/`EmitAsync`, `EventMiddleware` for request lifecycle events

### Core Framework
- **Lifecycle hooks** — `OnModuleInit` / `OnModuleDestroy` auto-wired to providers with init/destroy ordering
- **Request-scoped DI** — `ScopedDI` middleware with `ScopedContainer`, parent fallback to app container
- **Idempotency middleware** — `Idempotency-Key` header caching with TTL-based store, response replay
- **Cron scheduler** — Full cron expression parser (`*/5 * * * *`), concurrent task execution, `Start`/`Stop`

### Developer Experience
- **OpenAPI/Swagger generation** — `cmd/oapi-gen` emits OpenAPI 3.0.3 spec from `@Controller`/`@Summary` annotations
- **File upload handling** — `ctx.BindFile()`, `ctx.BindFiles()`, `ctx.BindForm()`, `SaveFile()` with path traversal protection
- **Pagination helpers** — `ParsePaginationFromContext()`, RFC 5988 `Link` header, standardized JSON response
- **Rate limiter** — Token bucket algorithm with per-IP keying, auto-eviction, `X-RateLimit-*` headers

## Bug Fixes

| Fix | Impact |
|---|---|
| Container lifecycle wiring | Factory-resolved providers now correctly trigger `OnModuleInit`/`OnModuleDestroy` hooks |
| ScopedDI parent resolution | `ScopedContainer` now correctly inherits from `app.Container` |
| Idempotency header capture | Response headers (including `Set-Cookie`) now captured at `Write()` time, not `WriteHeader()` |
| Scheduler double-close | `sync.Once` prevents panic when both `ctx.Done()` and `Stop()` fire |

## Breaking Changes

### `app.Run()` now manages lifecycle automatically
Previously `app.Run()` only started the HTTP server. It now:
1. Calls `Lifecycle.Init(ctx)` before starting
2. Calls `Health.MarkReady()` after init
3. Calls `Lifecycle.Destroy(ctx)` on shutdown

If your providers implement `Lifecycle`, `LifecycleInitOnly`, or `LifecycleDestroyOnly`, their hooks will now execute automatically.

### `app.Listen()` does NOT trigger lifecycle
`app.Listen()` only starts the HTTP server. Use `app.Run(ctx, opts)` for full lifecycle management including health checks and graceful shutdown.

## Migration Guide (v1.3.1 → v1.4.0)

```go
// Before (v1.3.1)
app.Listen(":8080")

// After (v1.4.0) — recommended
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
app.Run(ctx, gocontroller.ServerOptions{Addr: ":8080"})
```

## Complete Feature Matrix

| Category | Features |
|---|---|
| **Routing** | `:param` paths, `*` wildcards, `RouteGroup` nesting, 405 Method Not Allowed |
| **Middleware** | RequestID, Recovery, CORS, SecurityHeaders, RequestLogger, ModuleName |
| **Security** | JWT, CSRF, HMAC signatures, rate limiting, body size limits |
| **Sessions** | Encrypted cookie sessions, regeneration, destruction |
| **Validation** | Struct tag validation (go-playground/validator), generic DTO parsing |
| **DI** | Reflection-based container, interface matching, scoped DI, lifecycle hooks |
| **Observability** | Tracing (OTel-compatible), event bus, health checks (liveness/readiness) |
| **File Handling** | Multipart uploads, file saving with sanitization |
| **Pagination** | Page/limit parsing, Link headers, JSON response envelope |
| **Reliability** | Idempotency caching, cron scheduler, graceful shutdown |
| **Codegen** | `gocontroller-gen` (routes), `oapi-gen` (OpenAPI spec) |
| **Server** | Configurable timeouts, TLS support, `http.Handler` exposure |

## Stats

- **38 source files** in `gocontroller/`
- **0 external web framework dependencies** (stdlib `net/http` only)
- **2 codegen tools** (`cmd/gocontroller-gen`, `cmd/oapi-gen`)
- **1 external dependency** (go-playground/validator)
