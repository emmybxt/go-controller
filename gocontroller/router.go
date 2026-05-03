package gocontroller

import (
	"errors"
	"net/http"
	"strings"
)

type route struct {
	method     string
	path       string
	parts      []string
	handler    HandlerFunc
	middleware []Middleware
}

type ErrorHandlerFunc func(*Context, error)

// Router provides method-based routing and middleware composition.
type Router struct {
	globalMiddleware []Middleware
	routes           []route
	validator        Validator
	errorHandler     ErrorHandlerFunc
	maxBodyBytes     int64
}

func NewRouter() *Router {
	return &Router{
		validator:    DefaultValidator(),
		maxBodyBytes: DefaultMaxBodyBytes,
	}
}

// SetErrorHandler overrides route-handler error rendering.
func (r *Router) SetErrorHandler(h ErrorHandlerFunc) {
	r.errorHandler = h
}

// SetValidator overrides validation engine used by Context.BindJSON for this router.
func (r *Router) SetValidator(v Validator) {
	if v == nil {
		return
	}
	r.validator = v
}

// Validator returns the validation engine currently attached to this router.
func (r *Router) Validator() Validator {
	if r.validator == nil {
		return DefaultValidator()
	}
	return r.validator
}

// SetMaxBodyBytes sets the maximum request body size used by Context.BindJSON.
// Values <= 0 disable the framework-level body limit.
func (r *Router) SetMaxBodyBytes(n int64) {
	r.maxBodyBytes = n
}

// MaxBodyBytes returns the configured request body limit for JSON binding.
func (r *Router) MaxBodyBytes() int64 {
	return r.maxBodyBytes
}

func (r *Router) Use(middleware ...Middleware) {
	r.globalMiddleware = append(r.globalMiddleware, middleware...)
}

func (r *Router) Group(prefix string, middleware ...Middleware) *RouteGroup {
	return &RouteGroup{
		router:     r,
		prefix:     cleanPath(prefix),
		middleware: middleware,
	}
}

func (r *Router) GET(path string, handler HandlerFunc, middleware ...Middleware) {
	r.add(http.MethodGet, path, handler, middleware...)
}

func (r *Router) POST(path string, handler HandlerFunc, middleware ...Middleware) {
	r.add(http.MethodPost, path, handler, middleware...)
}

func (r *Router) PUT(path string, handler HandlerFunc, middleware ...Middleware) {
	r.add(http.MethodPut, path, handler, middleware...)
}

func (r *Router) DELETE(path string, handler HandlerFunc, middleware ...Middleware) {
	r.add(http.MethodDelete, path, handler, middleware...)
}

func (r *Router) PATCH(path string, handler HandlerFunc, middleware ...Middleware) {
	r.add(http.MethodPatch, path, handler, middleware...)
}

func (r *Router) OPTIONS(path string, handler HandlerFunc, middleware ...Middleware) {
	r.add(http.MethodOptions, path, handler, middleware...)
}

func (r *Router) add(method, path string, handler HandlerFunc, middleware ...Middleware) {
	p := cleanPath(path)
	r.routes = append(r.routes, route{
		method:     method,
		path:       p,
		parts:      splitPath(p),
		handler:    handler,
		middleware: middleware,
	})
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	allowedMethods := map[string]struct{}{}
	var firstPathMatch *route
	var firstParams map[string]string
	for _, rt := range r.routes {
		params, ok := matchPath(rt.parts, splitPath(req.URL.Path))
		if !ok {
			continue
		}
		if firstPathMatch == nil {
			rtCopy := rt
			firstPathMatch = &rtCopy
			firstParams = params
		}
		if req.Method != rt.method {
			allowedMethods[rt.method] = struct{}{}
			continue
		}

		ctx := newContext(w, req, params, r.Validator(), r.MaxBodyBytes())
		final := chain(rt.handler, append(r.globalMiddleware, rt.middleware...))
		if err := final(ctx); err != nil {
			r.handleError(ctx, err)
		}
		return
	}
	if req.Method == http.MethodOptions && firstPathMatch != nil {
		ctx := newContext(w, req, firstParams, r.Validator(), r.MaxBodyBytes())
		noop := func(*Context) error { return nil }
		final := chain(noop, append(r.globalMiddleware, firstPathMatch.middleware...))
		if err := final(ctx); err != nil {
			r.handleError(ctx, err)
		}
		return
	}
	if len(allowedMethods) > 0 {
		allow := make([]string, 0, len(allowedMethods))
		for method := range allowedMethods {
			allow = append(allow, method)
		}
		w.Header().Set("Allow", strings.Join(allow, ", "))
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = jsonErrorDetailed(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil, w.Header().Get(RequestIDHeader))
		return
	}
	w.WriteHeader(http.StatusNotFound)
	_ = jsonError(w, http.StatusNotFound, "not found")
}

func (r *Router) handleError(ctx *Context, err error) {
	if r.errorHandler != nil {
		r.errorHandler(ctx, err)
		return
	}
	r.writeError(ctx.ResponseWriter, err)
}

func (r *Router) writeError(w http.ResponseWriter, err error) {
	traceID := w.Header().Get(RequestIDHeader)

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		status := apiErr.StatusCode
		if status == 0 {
			status = http.StatusInternalServerError
		}
		w.WriteHeader(status)
		_ = jsonErrorDetailed(w, status, apiErr.Code, apiErr.Message, apiErr.Details, traceID)
		return
	}

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		w.WriteHeader(httpErr.StatusCode)
		_ = jsonErrorDetailed(w, httpErr.StatusCode, "", httpErr.Message, nil, traceID)
		return
	}
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_ = jsonErrorDetailed(w, http.StatusRequestEntityTooLarge, "request_body_too_large", "request body too large", nil, traceID)
		return
	}
	if errors.Is(err, ErrValidation) {
		w.WriteHeader(http.StatusBadRequest)
		_ = jsonErrorDetailed(w, http.StatusBadRequest, "validation_failed", ErrValidation.Error(), nil, traceID)
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
	_ = jsonErrorDetailed(w, http.StatusInternalServerError, "internal_error", "internal server error", nil, traceID)
}

type RouteGroup struct {
	router     *Router
	prefix     string
	middleware []Middleware
}

func (g *RouteGroup) Group(prefix string, middleware ...Middleware) *RouteGroup {
	combined := append(append([]Middleware{}, g.middleware...), middleware...)
	return &RouteGroup{
		router:     g.router,
		prefix:     joinPath(g.prefix, prefix),
		middleware: combined,
	}
}

func (g *RouteGroup) GET(path string, handler HandlerFunc, middleware ...Middleware) {
	g.router.GET(g.combine(path), handler, append(g.middleware, middleware...)...)
}

func (g *RouteGroup) POST(path string, handler HandlerFunc, middleware ...Middleware) {
	g.router.POST(g.combine(path), handler, append(g.middleware, middleware...)...)
}

func (g *RouteGroup) PUT(path string, handler HandlerFunc, middleware ...Middleware) {
	g.router.PUT(g.combine(path), handler, append(g.middleware, middleware...)...)
}

func (g *RouteGroup) DELETE(path string, handler HandlerFunc, middleware ...Middleware) {
	g.router.DELETE(g.combine(path), handler, append(g.middleware, middleware...)...)
}

func (g *RouteGroup) PATCH(path string, handler HandlerFunc, middleware ...Middleware) {
	g.router.PATCH(g.combine(path), handler, append(g.middleware, middleware...)...)
}

func (g *RouteGroup) OPTIONS(path string, handler HandlerFunc, middleware ...Middleware) {
	g.router.OPTIONS(g.combine(path), handler, append(g.middleware, middleware...)...)
}

func (g *RouteGroup) combine(path string) string {
	return joinPath(g.prefix, path)
}

func cleanPath(path string) string {
	if path == "" || path == "/" {
		return "/"
	}
	path = "/" + strings.Trim(path, "/")
	return path
}

func joinPath(left, right string) string {
	l := strings.Trim(left, "/")
	r := strings.Trim(right, "/")

	switch {
	case l == "" && r == "":
		return "/"
	case l == "":
		return "/" + r
	case r == "":
		return "/" + l
	default:
		return "/" + l + "/" + r
	}
}

func splitPath(path string) []string {
	if path == "/" {
		return nil
	}
	return strings.Split(strings.Trim(path, "/"), "/")
}

func matchPath(pattern, actual []string) (map[string]string, bool) {
	// Support a trailing wildcard segment: /assets/* matches /assets/a/b/c.
	if len(pattern) > 0 && pattern[len(pattern)-1] == "*" {
		if len(actual) < len(pattern)-1 {
			return nil, false
		}
		params := map[string]string{}
		for i := 0; i < len(pattern)-1; i++ {
			part := pattern[i]
			candidate := actual[i]
			if strings.HasPrefix(part, ":") {
				params[strings.TrimPrefix(part, ":")] = candidate
				continue
			}
			if part != candidate {
				return nil, false
			}
		}
		params["*"] = strings.Join(actual[len(pattern)-1:], "/")
		return params, true
	}

	if len(pattern) != len(actual) {
		return nil, false
	}
	params := map[string]string{}
	for i := range pattern {
		part := pattern[i]
		candidate := actual[i]
		if strings.HasPrefix(part, ":") {
			params[strings.TrimPrefix(part, ":")] = candidate
			continue
		}
		if part != candidate {
			return nil, false
		}
	}
	return params, true
}
