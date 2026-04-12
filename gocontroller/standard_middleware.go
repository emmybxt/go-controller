package gocontroller

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"runtime/debug"
)

const RequestIDHeader = "X-Request-ID"
const requestIDContextKey = "gocontroller.request_id"

func (c *Context) RequestID() string {
	v, ok := c.Get(requestIDContextKey)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// RequestID injects a request id into Context and response header.
func RequestID() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			id := ctx.Request.Header.Get(RequestIDHeader)
			if id == "" {
				id = newRequestID()
			}
			ctx.Set(requestIDContextKey, id)
			ctx.ResponseWriter.Header().Set(RequestIDHeader, id)
			return next(ctx)
		}
	}
}

type RecoveryConfig struct {
	IncludeStack bool
	Logf         func(format string, args ...any)
}

// Recovery catches panics and returns a standardized 500 error response.
func Recovery(config RecoveryConfig) Middleware {
	logger := config.Logf
	if logger == nil {
		logger = log.Printf
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					if config.IncludeStack {
						logger("[gocontroller] panic recovered: %v\n%s", rec, string(debug.Stack()))
					} else {
						logger("[gocontroller] panic recovered: %v", rec)
					}
					err = &APIError{
						StatusCode: 500,
						Code:       "internal_panic",
						Message:    "Internal server error",
						Cause:      fmt.Errorf("panic: %v", rec),
					}
				}
			}()
			return next(ctx)
		}
	}
}

func newRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "gc-fallback-id"
	}
	return hex.EncodeToString(b)
}
