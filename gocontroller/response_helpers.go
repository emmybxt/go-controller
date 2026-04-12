package gocontroller

import "net/http"

// Success returns a standard success envelope: {"success": true, "data": ...}.
func (c *Context) Success(status int, data any) error {
	return c.JSON(status, map[string]any{
		"success": true,
		"data":    data,
	})
}

// Error returns a standard error envelope: {"success": false, "error": ...}.
func (c *Context) Error(status int, message string) error {
	return c.JSON(status, map[string]any{
		"success": false,
		"error":   message,
	})
}

func (c *Context) OK(data any) error { return c.JSON(http.StatusOK, data) }

func (c *Context) Created(data any) error { return c.JSON(http.StatusCreated, data) }

func (c *Context) NoContent() error {
	c.ResponseWriter.WriteHeader(http.StatusNoContent)
	return nil
}

func (c *Context) BadRequest(message string) error {
	return c.Error(http.StatusBadRequest, message)
}

func (c *Context) Unauthorized(message string) error {
	return c.Error(http.StatusUnauthorized, message)
}

func (c *Context) Forbidden(message string) error {
	return c.Error(http.StatusForbidden, message)
}

func (c *Context) NotFound(message string) error {
	return c.Error(http.StatusNotFound, message)
}

func (c *Context) Conflict(message string) error {
	return c.Error(http.StatusConflict, message)
}

func (c *Context) InternalError(message string) error {
	return c.Error(http.StatusInternalServerError, message)
}
