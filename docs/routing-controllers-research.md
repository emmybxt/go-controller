# Routing Controllers research

Reviewed 2026-09-21 against the upstream `develop` documentation/source. These findings describe upstream behavior; the proposed Go API below is a recommendation, not an upstream guarantee.

## What upstream provides

- `UseBefore` and `UseAfter` attach native Express/Koa middleware to a controller or an individual action. Users can supply existing middleware, custom functions, or middleware classes. Classes may use dependency injection. Global middleware is marked with `Middleware`, choosing `before` or `after`. [Middleware documentation](https://github.com/typestack/routing-controllers#using-middlewares)
- The decorators retain each supplied middleware and mark whether it belongs after the action. They do not themselves execute it. [UseBefore source](https://github.com/typestack/routing-controllers/blob/develop/src/decorator/UseBefore.ts), [UseAfter source](https://github.com/typestack/routing-controllers/blob/develop/src/decorator/UseAfter.ts)
- Express combines controller middleware before action middleware, then registers the chain as route guard, before middleware, built-in body/auth/upload middleware, action, after middleware. Normal successful responses advance the chain; buffer/stream responses do not always do so. Errors advance through `next(error)`. Therefore ordinary after middleware is not guaranteed cleanup and is not a response transformer. [ExpressDriver registration, success and error handling](https://github.com/typestack/routing-controllers/blob/develop/src/driver/express/ExpressDriver.ts)
- Global middleware is filtered by before/after type and sorted by descending priority. Interceptors combine global, controller, and action scopes. After the action resolves successfully, interceptors transform its result sequentially before the driver writes the response; failures go to the error handler. [RoutingControllers source](https://github.com/typestack/routing-controllers/blob/develop/src/RoutingControllers.ts)
- `Authorized` delegates roles and request information to an application checker. `CurrentUser` delegates user lookup to another application checker. Custom parameter decorators supply a resolver and optional required constraint. These are extension points, not built-in authentication or database policy. [Authorization and custom parameter documentation](https://github.com/typestack/routing-controllers#using-authorization-features)
- A required current user rejects missing users with an authorization error. The parameter handler also performs normalization, transformation and validation for supported input parameters. [ActionParameterHandler source](https://github.com/typestack/routing-controllers/blob/develop/src/ActionParameterHandler.ts)

Koa middleware usage was checked in official documentation, but its driver source could not be retrieved in this session; do not infer identical edge-case behavior from the Express implementation.

## Constraints for a native Go plugin

Go source files permit top-level declarations, not bare expression statements. A declaration such as `var _ = accounts.GET("/:accountId")` can be valid Go and carry generator metadata; the bare call above a method cannot. Automatic binding to the next method is a generator convention we would introduce. [Go specification](https://go.dev/ref/spec#Source_file_organization)

Native middleware differs across adapters:

- Gin's `Next` runs remaining handlers; `Abort` prevents pending handlers. An after callback must respect aborts. [Gin context source](https://github.com/gin-gonic/gin/blob/master/context.go)
- Fiber advances only when the handler calls `Next`; appending middleware after a normal terminal route does not make it execute. Its native middleware already supports post-processing around `Next`. [Fiber routing documentation](https://docs.gofiber.io/guide/routing/#middleware)
- Echo middleware has the shape `func(HandlerFunc) HandlerFunc`; it wraps the next handler. Keep that native contract. [Echo source](https://github.com/labstack/echo/blob/master/echo.go)

Recommendation: explicitly define whether `UseAfter` means native downstream middleware or a success-after hook. If it means a hook after an ordinary handler returns, implement adapter-specific behavior and document errors, aborts, and continuation. Do not advertise it as a guaranteed finally hook, and do not silently invoke unrelated later Fiber routes. Keep native `UseBefore` wrappers available for recovery, timing, error handling and cleanup.

The selected Go design is success-after behavior with native middleware types: Gin retains its chain and abort behavior; Echo enters an after-middleware chain only after a nil handler error; Fiber bridges a successful handler into its after chain, with a request-local gate against repeated execution when a handler calls `Next`, and a terminal handler to prevent unrelated route fallthrough. This is a Go-specific adaptation, not exact upstream parity. Verify ordinary success, explicit `Next`, nested middleware order, abort/denial, returned errors, and repeated requests across every supported adapter version. In Gin, an attached context error alone is distinct from aborting; document that native distinction.

## Practical priorities

1. Add controller/route `UseBefore`, `UseAfter`, and `Use` convenience calls. Accept users' existing native middleware. Preserve existing annotations and direct metadata so this remains an additive release.
2. Add valid-Go declaration generation with clear diagnostics for an unattached declaration, unsupported receiver, or duplicate route. Generation should retain the real method signature and native context.
3. Demonstrate `CurrentUser`, tenant and loaded-record access as typed application resolvers over the native request context. Authentication middleware resolves once and stores request-local data. A library helper is worthwhile only if it removes actual repeated code; global mutable user state must never be used.
4. Consider reusable authorization middleware factories and optional typed input binding/validation later. Keep role meaning, user loading, validation policy and status decisions under application control.
5. Defer automatic serialization/interceptors, parameter reflection, controller inheritance and an ORM-style current-record decorator. Native handlers already write responses; intercepting returned content would require a separate handler contract and expand the plugin into a framework.

An additive API preserving v2 behavior is a minor release (for example v2.1.0), subject to the repository's actual current version. Publishing is separate from preparing and testing the change.
