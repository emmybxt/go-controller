package gocontroller

import (
	"context"
	"fmt"
	"sync"
)

// Lifecycle defines hooks for module initialization and cleanup.
type Lifecycle interface {
	OnModuleInit(ctx context.Context) error
	OnModuleDestroy(ctx context.Context) error
}

// LifecycleInitOnly implements only OnModuleInit.
type LifecycleInitOnly interface {
	OnModuleInit(ctx context.Context) error
}

// LifecycleDestroyOnly implements only OnModuleDestroy.
type LifecycleDestroyOnly interface {
	OnModuleDestroy(ctx context.Context) error
}

type lifecycleEntry struct {
	instance any
	name     string
}

// LifecycleManager tracks and executes lifecycle hooks.
type LifecycleManager struct {
	mu       sync.Mutex
	entries  []lifecycleEntry
	initDone bool
}

// NewLifecycleManager creates a new lifecycle manager.
func NewLifecycleManager() *LifecycleManager {
	return &LifecycleManager{}
}

// Register adds an instance to the lifecycle manager.
func (m *LifecycleManager) Register(instance any, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.initDone {
		panic(fmt.Sprintf("cannot register lifecycle hook %q after initialization", name))
	}
	m.entries = append(m.entries, lifecycleEntry{instance: instance, name: name})
}

// Init executes OnModuleInit hooks for all registered instances.
func (m *LifecycleManager) Init(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, entry := range m.entries {
		switch v := entry.instance.(type) {
		case Lifecycle:
			if err := v.OnModuleInit(ctx); err != nil {
				return fmt.Errorf("lifecycle %s OnModuleInit: %w", entry.name, err)
			}
		case LifecycleInitOnly:
			if err := v.OnModuleInit(ctx); err != nil {
				return fmt.Errorf("lifecycle %s OnModuleInit: %w", entry.name, err)
			}
		}
	}

	m.initDone = true
	return nil
}

// Destroy executes OnModuleDestroy hooks in reverse order.
func (m *LifecycleManager) Destroy(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for i := len(m.entries) - 1; i >= 0; i-- {
		entry := m.entries[i]
		switch v := entry.instance.(type) {
		case Lifecycle:
			if err := v.OnModuleDestroy(ctx); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("lifecycle %s OnModuleDestroy: %w", entry.name, err)
			}
		case LifecycleDestroyOnly:
			if err := v.OnModuleDestroy(ctx); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("lifecycle %s OnModuleDestroy: %w", entry.name, err)
			}
		}
	}

	return firstErr
}

// LifecycleProvider wraps a provider factory to register lifecycle hooks.
func LifecycleProvider(fn any, name string) any {
	return lifecycleProviderWrapper{fn: fn, name: name}
}

type lifecycleProviderWrapper struct {
	fn   any
	name string
}

func (w lifecycleProviderWrapper) Provider() any {
	return w.fn
}

func (w lifecycleProviderWrapper) Name() string {
	return w.name
}
