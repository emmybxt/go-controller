package gocontroller

import (
	"fmt"
	"reflect"
)

type providerFactory struct {
	fn          reflect.Value
	isLifecycle bool
	name        string
}

type Container struct {
	instances  map[reflect.Type]reflect.Value
	factories  map[reflect.Type]providerFactory
	lifecycle  *LifecycleManager
	moduleName string
}

func NewContainer() *Container {
	return &Container{
		instances: map[reflect.Type]reflect.Value{},
		factories: map[reflect.Type]providerFactory{},
	}
}

func (c *Container) Provide(value any) error {
	return c.provideWithLifecycle(value, nil, "")
}

func (c *Container) provideWithLifecycle(value any, lifecycle *LifecycleManager, moduleName string) error {
	if value == nil {
		return fmt.Errorf("nil provider")
	}

	if lp, ok := value.(lifecycleProviderWrapper); ok {
		value = lp.Provider()
		moduleName = lp.Name()
	}

	v := reflect.ValueOf(value)
	t := v.Type()

	if t.Kind() == reflect.Func {
		if t.NumOut() != 1 && t.NumOut() != 2 {
			return fmt.Errorf("provider func must return value or (value, error)")
		}
		if t.NumOut() == 2 {
			errType := t.Out(1)
			if !errType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				return fmt.Errorf("second return value must be error")
			}
		}
		outType := t.Out(0)
		c.factories[outType] = providerFactory{fn: v, isLifecycle: lifecycle != nil, name: moduleName}
		return nil
	}

	c.instances[t] = v
	if lifecycle != nil {
		registerInstanceLifecycle(v.Interface(), lifecycle, moduleName)
	}
	return nil
}

func (c *Container) Resolve(target any) error {
	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Pointer || ptr.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer")
	}
	t := ptr.Elem().Type()
	value, err := c.resolveType(t)
	if err != nil {
		return err
	}
	ptr.Elem().Set(value)
	return nil
}

func (c *Container) MustResolve(target any) {
	if err := c.Resolve(target); err != nil {
		panic(err)
	}
}

func registerInstanceLifecycle(instance any, lifecycle *LifecycleManager, moduleName string) {
	if lifecycle == nil {
		return
	}
	switch instance.(type) {
	case Lifecycle, LifecycleInitOnly, LifecycleDestroyOnly:
		name := moduleName
		if namer, ok := instance.(interface{ ModuleName() string }); ok {
			name = namer.ModuleName()
		}
		lifecycle.Register(instance, name)
	}
}

func (c *Container) resolveType(t reflect.Type) (reflect.Value, error) {
	if v, ok := c.instances[t]; ok {
		return v, nil
	}
	if t.Kind() == reflect.Interface {
		for instType, inst := range c.instances {
			if instType.Implements(t) {
				return inst, nil
			}
		}
	}
	factory, ok := c.factories[t]
	if !ok {
		return reflect.Value{}, fmt.Errorf("no provider for type %s", t.String())
	}

	fnType := factory.fn.Type()
	args := make([]reflect.Value, fnType.NumIn())
	for i := 0; i < fnType.NumIn(); i++ {
		argType := fnType.In(i)
		argValue, err := c.resolveType(argType)
		if err != nil {
			return reflect.Value{}, err
		}
		args[i] = argValue
	}

	results := factory.fn.Call(args)
	value := results[0]
	if len(results) == 2 && !results[1].IsNil() {
		return reflect.Value{}, results[1].Interface().(error)
	}

	c.instances[t] = value
	if factory.isLifecycle {
		registerInstanceLifecycle(value.Interface(), c.lifecycle, factory.name)
	}

	return value, nil
}
