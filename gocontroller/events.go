package gocontroller

import (
	"context"
	"reflect"
	"sync"
)

// Event represents a domain event.
type Event struct {
	Name     string
	Payload  any
	Metadata map[string]string
}

// EventHandler processes an event.
type EventHandler func(ctx context.Context, event Event) error

// EventBus provides pub/sub event dispatching.
type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]EventHandler),
	}
}

// On registers a handler for an event name.
func (b *EventBus) On(name string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = append(b.handlers[name], handler)
}

// Once registers a handler that fires only once.
func (b *EventBus) Once(name string, handler EventHandler) {
	var onceHandler EventHandler
	onceHandler = func(ctx context.Context, event Event) error {
		err := handler(ctx, event)
		b.mu.Lock()
		defer b.mu.Unlock()
		if handlers, ok := b.handlers[name]; ok {
			for i, h := range handlers {
				if reflect.ValueOf(h).Pointer() == reflect.ValueOf(onceHandler).Pointer() {
					b.handlers[name] = append(handlers[:i], handlers[i+1:]...)
					break
				}
			}
		}
		return err
	}
	b.On(name, onceHandler)
}

// Off removes all handlers for an event name.
func (b *EventBus) Off(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.handlers, name)
}

// Emit fires an event to all registered handlers.
func (b *EventBus) Emit(ctx context.Context, event Event) error {
	b.mu.RLock()
	handlers, ok := b.handlers[event.Name]
	b.mu.RUnlock()

	if !ok {
		return nil
	}

	var firstErr error
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// EmitSync fires an event synchronously, blocking until all handlers complete.
func (b *EventBus) EmitSync(ctx context.Context, event Event) error {
	return b.Emit(ctx, event)
}

// EmitAsync fires an event asynchronously in a goroutine.
func (b *EventBus) EmitAsync(ctx context.Context, event Event) {
	go func() {
		_ = b.Emit(ctx, event)
	}()
}

// EventMiddleware returns a middleware that emits events on request lifecycle.
func EventMiddleware(bus *EventBus) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			if err := bus.Emit(ctx.Request.Context(), Event{
				Name: "request.start",
				Payload: map[string]string{
					"method": ctx.Request.Method,
					"path":   ctx.Request.URL.Path,
				},
			}); err != nil {
				return err
			}

			err := next(ctx)

			status := "success"
			if err != nil {
				status = "error"
			}

			_ = bus.Emit(ctx.Request.Context(), Event{
				Name: "request.end",
				Payload: map[string]string{
					"method": ctx.Request.Method,
					"path":   ctx.Request.URL.Path,
					"status": status,
				},
			})

			return err
		}
	}
}

// EventBusFromContext retrieves the event bus from context.
func EventBusFromContext(ctx *Context) *EventBus {
	v, ok := ctx.Get("gocontroller.event_bus")
	if !ok {
		return nil
	}
	bus, _ := v.(*EventBus)
	return bus
}
