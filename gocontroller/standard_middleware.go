package gocontroller

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
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

type CORSConfig struct {
	AllowOrigin      string
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	MaxAge           int
}

// CORS applies a basic CORS policy and handles OPTIONS preflight.
func CORS(config CORSConfig) Middleware {
	origins := append([]string{}, config.AllowOrigins...)
	if config.AllowOrigin != "" {
		origins = append(origins, config.AllowOrigin)
	}
	methods := config.AllowMethods
	if len(methods) == 0 {
		methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	headers := config.AllowHeaders
	if len(headers) == 0 {
		headers = []string{"Content-Type", "Authorization", RequestIDHeader}
	}
	maxAge := config.MaxAge
	if maxAge <= 0 {
		maxAge = 600
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			h := ctx.ResponseWriter.Header()
			requestOrigin := ctx.Request.Header.Get("Origin")
			if origin, ok := allowedCORSOrigin(requestOrigin, origins, config.AllowCredentials); ok {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
				h.Set("Access-Control-Allow-Headers", strings.Join(headers, ", "))
				h.Set("Access-Control-Max-Age", fmt.Sprintf("%d", maxAge))
				if config.AllowCredentials {
					h.Set("Access-Control-Allow-Credentials", "true")
				}
			}

			if ctx.Request.Method == http.MethodOptions {
				ctx.ResponseWriter.WriteHeader(http.StatusNoContent)
				return nil
			}
			return next(ctx)
		}
	}
}

func allowedCORSOrigin(requestOrigin string, allowed []string, credentials bool) (string, bool) {
	if len(allowed) == 0 {
		return "", false
	}
	for _, origin := range allowed {
		origin = strings.TrimSpace(origin)
		switch {
		case origin == "":
			continue
		case origin == "*" && credentials && requestOrigin != "":
			return requestOrigin, true
		case origin == "*":
			return "*", true
		case requestOrigin != "" && origin == requestOrigin:
			return requestOrigin, true
		}
	}
	return "", false
}

// SecurityHeaders adds common hardening headers to responses.
func SecurityHeaders() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			h := ctx.ResponseWriter.Header()
			h.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; frame-ancestors 'none'")
			h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-Permitted-Cross-Domain-Policies", "none")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("X-XSS-Protection", "0")
			return next(ctx)
		}
	}
}
