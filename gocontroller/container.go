package gocontroller

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type provider struct {
	value   reflect.Value
	factory reflect.Value
}

type container struct {
	providers map[reflect.Type]*provider
	resolving map[reflect.Type]bool
}

func newContainer() *container {
	return &container{
		providers: make(map[reflect.Type]*provider),
		resolving: make(map[reflect.Type]bool),
	}
}

func (c *container) provide(def any) error {
	v := reflect.ValueOf(def)
	if isNil(v) {
		return fmt.Errorf("provider is nil")
	}
	t := v.Type()
	p := &provider{value: v}
	if t.Kind() == reflect.Func {
		if err := validateConstructor(v); err != nil {
			return err
		}
		t = t.Out(0)
		p = &provider{factory: v}
	}
	if _, exists := c.providers[t]; exists {
		return fmt.Errorf("duplicate provider for %s", t)
	}
	c.providers[t] = p
	return nil
}

func validateConstructor(fn reflect.Value) error {
	t := fn.Type()
	if t.IsVariadic() {
		return fmt.Errorf("constructor %s must not be variadic", t)
	}
	if t.NumOut() != 1 && t.NumOut() != 2 {
		return fmt.Errorf("constructor %s must return T or (T, error)", t)
	}
	if t.NumOut() == 2 && t.Out(1) != reflect.TypeFor[error]() {
		return fmt.Errorf("constructor %s second return must be error", t)
	}
	return nil
}

func (c *container) resolve(t reflect.Type) (reflect.Value, error) {
	key, err := c.providerType(t)
	if err != nil {
		return reflect.Value{}, err
	}
	p := c.providers[key]
	if p.value.IsValid() {
		return p.value, nil
	}
	if c.resolving[key] {
		return reflect.Value{}, fmt.Errorf("circular provider dependency at %s", key)
	}
	c.resolving[key] = true
	defer delete(c.resolving, key)
	p.value, err = c.construct(p.factory)
	return p.value, err
}

func (c *container) providerType(t reflect.Type) (reflect.Type, error) {
	if _, ok := c.providers[t]; ok {
		return t, nil
	}
	var candidates []reflect.Type
	for candidate := range c.providers {
		if candidate.AssignableTo(t) {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no provider for %s", t)
	}
	names := make([]string, len(candidates))
	for i, candidate := range candidates {
		names[i] = candidate.String()
	}
	sort.Strings(names)
	return nil, fmt.Errorf("ambiguous providers for %s: %s; register a constructor returning the interface explicitly", t, strings.Join(names, ", "))
}

func (c *container) construct(fn reflect.Value) (reflect.Value, error) {
	args := make([]reflect.Value, fn.Type().NumIn())
	for i := range args {
		value, err := c.resolve(fn.Type().In(i))
		if err != nil {
			return reflect.Value{}, fmt.Errorf("constructor %s argument %d: %w", fn.Type(), i+1, err)
		}
		args[i] = value
	}
	results := fn.Call(args)
	if len(results) == 2 && !results[1].IsNil() {
		return reflect.Value{}, fmt.Errorf("constructor %s: %w", fn.Type(), results[1].Interface().(error))
	}
	if isNil(results[0]) {
		return reflect.Value{}, fmt.Errorf("constructor %s returned nil", fn.Type())
	}
	return results[0], nil
}

func (c *container) controller(def any) (Controller, error) {
	v := reflect.ValueOf(def)
	if isNil(v) {
		return nil, fmt.Errorf("controller is nil")
	}
	if v.Kind() == reflect.Func {
		if err := validateConstructor(v); err != nil {
			return nil, err
		}
		var err error
		v, err = c.construct(v)
		if err != nil {
			return nil, err
		}
	}
	controller, ok := v.Interface().(Controller)
	if !ok {
		return nil, fmt.Errorf("controller %s must implement ControllerMetadata(); for annotated controllers, run go generate", v.Type())
	}
	return controller, nil
}

func isNil(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	if v.Kind() == reflect.Interface && !v.IsNil() {
		return isNil(v.Elem())
	}
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
