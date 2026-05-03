package gocontroller

import (
	"fmt"
	"reflect"
	"sync"
)

// ScopedContainer provides request-level dependency injection.
type ScopedContainer struct {
	parent *Container
	values map[reflect.Type]reflect.Value
	mu     sync.RWMutex
}

// NewScopedContainer creates a new scoped container with a parent.
func NewScopedContainer(parent *Container) *ScopedContainer {
	return &ScopedContainer{
		parent: parent,
		values: make(map[reflect.Type]reflect.Value),
	}
}

// Provide registers a value in the current scope.
func (s *ScopedContainer) Provide(value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := reflect.ValueOf(value)
	t := v.Type()
	s.values[t] = v
	return nil
}

// Resolve resolves a dependency from the scope or parent container.
func (s *ScopedContainer) Resolve(target any) error {
	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Pointer || ptr.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer")
	}
	t := ptr.Elem().Type()

	s.mu.RLock()
	if v, ok := s.values[t]; ok {
		ptr.Elem().Set(v)
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	return s.parent.Resolve(target)
}

// ScopeConfig configures scoped DI middleware.
type ScopeConfig struct {
	ScopedProviders []any
	OnCreate        func(*ScopedContainer) error
	OnDispose       func(*ScopedContainer) error
}

// ScopedDI returns a middleware that creates a request-scoped DI container.
func ScopedDI(cfg ScopeConfig) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			scope := NewScopedContainer(nil)

			if ctx != nil && ctx.Request != nil {
				if app, ok := ctx.Values["gocontroller.app"].(*App); ok && app != nil {
					scope = NewScopedContainer(app.Container)
				}
			}

			if scope.parent == nil {
				scope = NewScopedContainer(NewContainer())
			}

			for _, provider := range cfg.ScopedProviders {
				if err := scope.Provide(provider); err != nil {
					return fmt.Errorf("scoped provider: %w", err)
				}
			}

			if cfg.OnCreate != nil {
				if err := cfg.OnCreate(scope); err != nil {
					return fmt.Errorf("scope OnCreate: %w", err)
				}
			}

			ctx.Set("gocontroller.scope", scope)

			err := next(ctx)

			if cfg.OnDispose != nil {
				_ = cfg.OnDispose(scope)
			}

			return err
		}
	}
}

// GetScope retrieves the scoped container from context.
func GetScope(ctx *Context) *ScopedContainer {
	v, ok := ctx.Get("gocontroller.scope")
	if !ok {
		return nil
	}
	s, _ := v.(*ScopedContainer)
	return s
}

// ResolveScoped resolves a dependency from the request scope.
func ResolveScoped[T any](ctx *Context) (T, error) {
	var zero T
	scope := GetScope(ctx)
	if scope == nil {
		return zero, fmt.Errorf("no scoped container in context")
	}
	var target T
	if err := scope.Resolve(&target); err != nil {
		return zero, err
	}
	return target, nil
}
