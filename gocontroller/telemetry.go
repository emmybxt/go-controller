package gocontroller

import (
	"context"
	"time"
)

type telemetryContextKey string

const telemetrySpanKey telemetryContextKey = "gocontroller.telemetry.span"

// Span represents a single operation trace span.
type Span interface {
	SetAttribute(key string, value any)
	SetStatus(code StatusCode, description string)
	End()
}

// StatusCode represents span status.
type StatusCode int

const (
	StatusUnset StatusCode = iota
	StatusOk
	StatusError
)

// Tracer creates spans for distributed tracing.
type Tracer interface {
	Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
}

// SpanOption configures span creation.
type SpanOption func(*SpanConfig)

// SpanConfig holds span creation options.
type SpanConfig struct {
	Attributes map[string]any
	Kind       SpanKind
}

// SpanKind represents the type of span.
type SpanKind int

const (
	SpanKindInternal SpanKind = iota
	SpanKindServer
	SpanKindClient
)

// WithAttributes adds attributes to a span.
func WithAttributes(attrs map[string]any) SpanOption {
	return func(cfg *SpanConfig) {
		if cfg.Attributes == nil {
			cfg.Attributes = map[string]any{}
		}
		for k, v := range attrs {
			cfg.Attributes[k] = v
		}
	}
}

// WithSpanKind sets the span kind.
func WithSpanKind(kind SpanKind) SpanOption {
	return func(cfg *SpanConfig) {
		cfg.Kind = kind
	}
}

// NoOpTracer is a tracer that does nothing.
type NoOpTracer struct{}

func (t NoOpTracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	return ctx, NoOpSpan{}
}

// NoOpSpan is a span that does nothing.
type NoOpSpan struct{}

func (s NoOpSpan) SetAttribute(key string, value any)            {}
func (s NoOpSpan) SetStatus(code StatusCode, description string) {}
func (s NoOpSpan) End()                                          {}

// TelemetryConfig configures the tracing middleware.
type TelemetryConfig struct {
	Tracer         Tracer
	ServiceName    string
	ServiceVersion string
	IncludeHeaders []string
	IncludeQuery   bool
	SkipPaths      []string
}

func (c *TelemetryConfig) applyDefaults() {
	if c.Tracer == nil {
		c.Tracer = NoOpTracer{}
	}
	if c.ServiceName == "" {
		c.ServiceName = "gocontroller"
	}
	skip := map[string]struct{}{"/healthz": {}, "/ready": {}, "/favicon.ico": {}}
	for _, p := range c.SkipPaths {
		skip[p] = struct{}{}
	}
	c.SkipPaths = nil
	for p := range skip {
		c.SkipPaths = append(c.SkipPaths, p)
	}
}

// Tracing returns a middleware that creates a span for each request.
func Tracing(cfg TelemetryConfig) Middleware {
	cfg.applyDefaults()

	skipSet := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skipSet[p] = struct{}{}
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			if _, skip := skipSet[ctx.Request.URL.Path]; skip {
				return next(ctx)
			}

			spanName := ctx.Request.Method + " " + ctx.Request.URL.Path

			attrs := map[string]any{
				"http.method":     ctx.Request.Method,
				"http.url":        ctx.Request.URL.Path,
				"http.host":       ctx.Request.Host,
				"http.user_agent": ctx.Request.UserAgent(),
				"http.scheme":     ctx.Request.URL.Scheme,
			}

			if cfg.IncludeQuery {
				attrs["http.query"] = ctx.Request.URL.RawQuery
			}

			for _, h := range cfg.IncludeHeaders {
				if v := ctx.Request.Header.Get(h); v != "" {
					attrs["http.request.header."+h] = v
				}
			}

			newCtx, span := cfg.Tracer.Start(ctx.Request.Context(), spanName, WithAttributes(attrs), WithSpanKind(SpanKindServer))
			ctx.Request = ctx.Request.WithContext(newCtx)
			ctx.Values[string(telemetrySpanKey)] = span

			defer span.End()

			err := next(ctx)

			status := StatusOk
			if err != nil {
				status = StatusError
				span.SetAttribute("error.message", err.Error())
			}
			span.SetStatus(status, "")

			return err
		}
	}
}

// SpanFromContext retrieves the current trace span from context.
func SpanFromContext(ctx *Context) Span {
	v, ok := ctx.Values[string(telemetrySpanKey)]
	if !ok {
		return NoOpSpan{}
	}
	s, ok := v.(Span)
	if !ok {
		return NoOpSpan{}
	}
	return s
}

// TraceID returns the current trace ID if available.
func (c *Context) TraceID() string {
	v, ok := c.Values["gocontroller.trace_id"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// RecordMetric records a custom metric attribute on the current span.
func (c *Context) RecordMetric(key string, value any) {
	span := SpanFromContext(c)
	span.SetAttribute(key, value)
}

// NewTimerSpan creates a child span for timing a specific operation.
func NewTimerSpan(ctx context.Context, tracer Tracer, name string) func() {
	if tracer == nil {
		return func() {}
	}
	_, span := tracer.Start(ctx, name)
	start := time.Now()
	return func() {
		span.SetAttribute("duration_ms", float64(time.Since(start).Milliseconds()))
		span.End()
	}
}
