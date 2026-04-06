package gocontroller

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Context carries request/response helpers for handlers and middleware.
type Context struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	Params         map[string]string
	Values         map[string]any
	validator      Validator
}

func newContext(w http.ResponseWriter, r *http.Request, params map[string]string, v Validator) *Context {
	return &Context{
		ResponseWriter: w,
		Request:        r,
		Params:         params,
		Values:         map[string]any{},
		validator:      v,
	}
}

func (c *Context) Set(key string, value any) {
	c.Values[key] = value
}

func (c *Context) Get(key string) (any, bool) {
	v, ok := c.Values[key]
	return v, ok
}

func (c *Context) Param(name string) string {
	return c.Params[name]
}

func (c *Context) JSON(status int, body any) error {
	c.ResponseWriter.Header().Set("Content-Type", "application/json")
	c.ResponseWriter.WriteHeader(status)
	return json.NewEncoder(c.ResponseWriter).Encode(body)
}

func (c *Context) Text(status int, value string) error {
	c.ResponseWriter.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.ResponseWriter.WriteHeader(status)
	_, err := c.ResponseWriter.Write([]byte(value))
	return err
}

func (c *Context) BindJSON(dto any) error {
	defer c.Request.Body.Close()
	if err := json.NewDecoder(c.Request.Body).Decode(dto); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	validator := c.validator
	if validator == nil {
		validator = DefaultValidator()
	}
	if err := validator.Validate(dto); err != nil {
		return err
	}
	return nil
}
