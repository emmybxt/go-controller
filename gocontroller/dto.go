package gocontroller

// ParseDTO decodes JSON body into a typed DTO and runs validation.
func ParseDTO[T any](ctx *Context) (T, error) {
	var dto T
	err := ctx.BindJSON(&dto)
	return dto, err
}
