package gocontroller

import (
	"net/http"
	"sync/atomic"
	"time"
)

// HealthStatus represents the result of a health check.
type HealthStatus struct {
	Status  string    `json:"status"`
	Checked time.Time `json:"checked_at"`
	Details any       `json:"details,omitempty"`
}

// HealthCheckerFunc returns the health status of a component.
type HealthCheckerFunc func() HealthStatus

// HealthRegistry manages multiple health checkers.
type HealthRegistry struct {
	liveness  map[string]HealthCheckerFunc
	readiness map[string]HealthCheckerFunc
	startTime time.Time
	started   atomic.Bool
}

// NewHealthRegistry creates a new health check registry.
func NewHealthRegistry() *HealthRegistry {
	return &HealthRegistry{
		liveness:  make(map[string]HealthCheckerFunc),
		readiness: make(map[string]HealthCheckerFunc),
		startTime: time.Now(),
	}
}

// RegisterLiveness adds a liveness probe check.
func (r *HealthRegistry) RegisterLiveness(name string, fn HealthCheckerFunc) {
	r.liveness[name] = fn
}

// RegisterReadiness adds a readiness probe check.
func (r *HealthRegistry) RegisterReadiness(name string, fn HealthCheckerFunc) {
	r.readiness[name] = fn
}

// MarkReady signals the application is ready to serve traffic.
func (r *HealthRegistry) MarkReady() {
	r.started.Store(true)
}

// IsReady returns whether the application is ready.
func (r *HealthRegistry) IsReady() bool {
	return r.started.Load()
}

// Uptime returns the time since the registry was created.
func (r *HealthRegistry) Uptime() time.Duration {
	return time.Since(r.startTime)
}

// HealthMiddleware returns a middleware that serves health endpoints.
func (hr *HealthRegistry) HealthMiddleware(livenessPath, readinessPath string) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			path := ctx.Request.URL.Path

			if path == livenessPath {
				return hr.handleLiveness(ctx)
			}
			if path == readinessPath {
				return hr.handleReadiness(ctx)
			}
			return next(ctx)
		}
	}
}

func (hr *HealthRegistry) handleLiveness(ctx *Context) error {
	if !hr.IsReady() {
		return ctx.JSON(http.StatusServiceUnavailable, map[string]any{
			"status":     "starting",
			"checked_at": time.Now().Format(time.RFC3339),
			"uptime":     hr.Uptime().String(),
		})
	}

	details := map[string]any{}
	status := "ok"
	for name, fn := range hr.liveness {
		result := fn()
		details[name] = result.Status
		if result.Status != "ok" {
			status = "error"
		}
	}

	code := http.StatusOK
	if status == "error" {
		code = http.StatusServiceUnavailable
	}

	return ctx.JSON(code, map[string]any{
		"status":     status,
		"checked_at": time.Now().Format(time.RFC3339),
		"uptime":     hr.Uptime().String(),
		"details":    details,
	})
}

func (hr *HealthRegistry) handleReadiness(ctx *Context) error {
	if !hr.IsReady() {
		return ctx.JSON(http.StatusServiceUnavailable, map[string]any{
			"status":     "not_ready",
			"checked_at": time.Now().Format(time.RFC3339),
			"reason":     "application is still starting",
		})
	}

	details := map[string]any{}
	status := "ok"
	for name, fn := range hr.readiness {
		result := fn()
		details[name] = result.Status
		if result.Status != "ok" {
			status = "error"
		}
	}

	code := http.StatusOK
	if status == "error" {
		code = http.StatusServiceUnavailable
	}

	return ctx.JSON(code, map[string]any{
		"status":     status,
		"checked_at": time.Now().Format(time.RFC3339),
		"details":    details,
	})
}

// LivenessHandler returns a standalone http.HandlerFunc for liveness.
func (hr *HealthRegistry) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hr.IsReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// ReadinessHandler returns a standalone http.HandlerFunc for readiness.
func (hr *HealthRegistry) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hr.IsReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
