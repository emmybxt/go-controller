package gocontroller

// MiddlewareGroup places native middleware before or after the controller
// handler. After middleware follows the framework's continuation/error rules;
// it is not a response transformer or a guaranteed finally hook.
type MiddlewareGroup struct {
	Before []any
	After  []any
}

func UseBefore(middleware ...any) MiddlewareGroup {
	return MiddlewareGroup{Before: append([]any(nil), middleware...)}
}

func UseAfter(middleware ...any) MiddlewareGroup {
	return MiddlewareGroup{After: append([]any(nil), middleware...)}
}
