package gocontroller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const DefaultMaxBodyBytes int64 = 1 << 20

// Context carries request/response helpers for handlers and middleware.
type Context struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	Params         map[string]string
	Values         map[string]any
	validator      Validator
	maxBodyBytes   int64
}

func newContext(w http.ResponseWriter, r *http.Request, params map[string]string, v Validator, maxBodyBytes int64) *Context {
	return &Context{
		ResponseWriter: w,
		Request:        r,
		Params:         params,
		Values:         map[string]any{},
		validator:      v,
		maxBodyBytes:   maxBodyBytes,
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

	body := c.Request.Body
	if c.maxBodyBytes > 0 {
		body = http.MaxBytesReader(c.ResponseWriter, body, c.maxBodyBytes)
	}

	decoder := json.NewDecoder(body)
	if err := decoder.Decode(dto); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return fmt.Errorf("decode request body: %w", err)
		}
		return &APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "invalid_json",
			Message:    "invalid JSON request body",
			Cause:      err,
		}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return BadRequestError("request body must contain a single JSON value")
		}
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

func newContextWithClaims(parent context.Context, key jwtContextKey, claims JWTClaims) context.Context {
	return context.WithValue(parent, key, claims)
}
