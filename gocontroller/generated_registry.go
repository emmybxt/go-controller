package gocontroller

import (
	"reflect"
	"sync"
)

var (
	generatedControllerRegistryMu sync.RWMutex
	generatedControllerRegistry   = map[reflect.Type]ControllerMetadata{}
)

// RegisterGeneratedControllerMetadata is used by generated code.
func RegisterGeneratedControllerMetadata(controllerPtr any, meta ControllerMetadata) {
	t := reflect.TypeOf(controllerPtr)
	if t == nil {
		return
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	generatedControllerRegistryMu.Lock()
	defer generatedControllerRegistryMu.Unlock()
	generatedControllerRegistry[t] = meta
}

func lookupGeneratedControllerMetadata(controller any) (ControllerMetadata, bool) {
	t := reflect.TypeOf(controller)
	if t == nil {
		return ControllerMetadata{}, false
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	generatedControllerRegistryMu.RLock()
	defer generatedControllerRegistryMu.RUnlock()
	meta, ok := generatedControllerRegistry[t]
	return meta, ok
}
