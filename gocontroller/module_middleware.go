package gocontroller

const moduleNameContextKey = "gocontroller.module_name"

// ModuleName annotates request context with a module name for logging/metrics.
func ModuleName(name string) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			if name != "" {
				ctx.Set(moduleNameContextKey, name)
			}
			return next(ctx)
		}
	}
}
