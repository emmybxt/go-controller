# v2.0.0 — native framework adapters

Go-controller now wires controllers and providers into an existing web framework. It registers native routes and leaves request handling and server ownership to Gin, Echo, or Fiber.

- Native adapters: Gin v1, Echo v5/v4, Fiber v3/v2, including native router groups.
- Familiar module fields: name, prefix, providers, controllers, imports, and middleware.
- Constructor injection with singleton reuse, interface resolution, cycle detection, and actionable startup errors.
- Module, controller, and route middleware retain native execution semantics.
- A small standard-library-only core and an open adapter interface.
- Optional annotations generate directly bound native handler references, with all nine standard HTTP verbs and a read-only `-check` mode.
- Runnable Gin, Echo, and Fiber examples, integration tests, and CI configuration.

This is a breaking release with the module path `github.com/emmybxt/go-controller/v2`. The old standalone router/context/runtime and bundled application features are removed. See [MIGRATING.md](MIGRATING.md) for the complete migration requirements.

## Installation

Requires Go 1.25 or newer.

```sh
go get github.com/emmybxt/go-controller/v2@v2.0.0
```

See [README.md](README.md) for module setup, native middleware, annotation generation, and runnable examples.
