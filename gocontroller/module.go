package gocontroller

import (
	"fmt"
	"reflect"
)

type Controller interface {
	RegisterRoutes(*RouteGroup)
}

// Module models a NestJS-like module graph.
type Module struct {
	Name        string
	Prefix      string
	Providers   []any
	Controllers []any
	Imports     []*Module
	Middleware  []Middleware
}

type App struct {
	Router    *Router
	Container *Container
	Lifecycle *LifecycleManager
	Health    *HealthRegistry
}

// SetValidator overrides the app/router validator for request DTO validation.
func (a *App) SetValidator(v Validator) {
	if a == nil || a.Router == nil {
		return
	}
	a.Router.SetValidator(v)
}

// Validator returns the validator used by the app/router.
func (a *App) Validator() Validator {
	if a == nil || a.Router == nil {
		return DefaultValidator()
	}
	return a.Router.Validator()
}

// SetMaxBodyBytes sets the maximum request body size used by Context.BindJSON.
func (a *App) SetMaxBodyBytes(n int64) {
	if a == nil || a.Router == nil {
		return
	}
	a.Router.SetMaxBodyBytes(n)
}

// MaxBodyBytes returns the configured request body limit for JSON binding.
func (a *App) MaxBodyBytes() int64 {
	if a == nil || a.Router == nil {
		return DefaultMaxBodyBytes
	}
	return a.Router.MaxBodyBytes()
}

// SetErrorHandler overrides router error rendering for this app.
func (a *App) SetErrorHandler(h ErrorHandlerFunc) {
	if a == nil || a.Router == nil {
		return
	}
	a.Router.SetErrorHandler(h)
}

func NewApp(root *Module) (*App, error) {
	router := NewRouter()
	container := NewContainer()
	lifecycle := NewLifecycleManager()
	health := NewHealthRegistry()
	seen := map[*Module]bool{}

	if err := loadModule(root, router, container, lifecycle, health, seen); err != nil {
		return nil, err
	}

	router.app = &App{Router: router, Container: container, Lifecycle: lifecycle, Health: health}
	return router.app, nil
}

func loadModule(mod *Module, router *Router, container *Container, lifecycle *LifecycleManager, health *HealthRegistry, seen map[*Module]bool) error {
	if mod == nil {
		return fmt.Errorf("module is nil")
	}
	if seen[mod] {
		return nil
	}
	seen[mod] = true

	for _, imported := range mod.Imports {
		if err := loadModule(imported, router, container, lifecycle, health, seen); err != nil {
			return err
		}
	}

	container.WithLifecycle(lifecycle, mod.Name)

	for _, p := range mod.Providers {
		if err := container.Provide(p); err != nil {
			return fmt.Errorf("module %s provider: %w", mod.Name, err)
		}
	}

	moduleMW := append([]Middleware{ModuleName(mod.Name)}, mod.Middleware...)
	group := router.Group(mod.Prefix, moduleMW...)
	for _, cdef := range mod.Controllers {
		controller, err := instantiateController(container, cdef)
		if err != nil {
			return fmt.Errorf("module %s controller: %w", mod.Name, err)
		}
		if err := registerController(group, controller); err != nil {
			return fmt.Errorf("module %s controller: %w", mod.Name, err)
		}
	}

	return nil
}

func instantiateController(container *Container, def any) (any, error) {
	if def == nil {
		return nil, fmt.Errorf("controller definition is nil")
	}

	if c, ok := def.(Controller); ok {
		return c, nil
	}
	if c, ok := def.(DecoratedController); ok {
		return c, nil
	}

	v := reflect.ValueOf(def)
	t := v.Type()
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("controller must implement Controller/DecoratedController or be constructor function")
	}
	if t.NumOut() != 1 && t.NumOut() != 2 {
		return nil, fmt.Errorf("controller constructor must return controller value or (controller value, error)")
	}

	args := make([]reflect.Value, t.NumIn())
	for i := 0; i < t.NumIn(); i++ {
		arg, err := container.resolveType(t.In(i))
		if err != nil {
			return nil, err
		}
		args[i] = arg
	}

	results := v.Call(args)
	if len(results) == 2 && !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}

	return results[0].Interface(), nil
}

func registerController(group *RouteGroup, instance any) error {
	if c, ok := instance.(Controller); ok {
		c.RegisterRoutes(group)
		return nil
	}
	if dc, ok := instance.(DecoratedController); ok {
		return registerDecoratedController(group, instance, dc.ControllerMetadata())
	}
	if meta, ok := lookupGeneratedControllerMetadata(instance); ok {
		return registerDecoratedController(group, instance, meta)
	}
	t := reflect.TypeOf(instance)
	return fmt.Errorf(
		"controller %v does not implement Controller/DecoratedController and has no generated metadata; run go generate in that package (for example: go generate ./example)",
		t,
	)
}
