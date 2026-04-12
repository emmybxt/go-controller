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

// Router provides method-based routing and middleware composition.
type Router struct {
	globalMiddleware []Middleware
	routes           []route
	validator        Validator
}

func NewRouter() *Router {
	return &Router{validator: DefaultValidator()}
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
	for _, rt := range r.routes {
		if req.Method != rt.method {
			continue
		}
		params, ok := matchPath(rt.parts, splitPath(req.URL.Path))
		if !ok {
			continue
		}

		ctx := newContext(w, req, params, r.Validator())
		final := chain(rt.handler, append(r.globalMiddleware, rt.middleware...))
		if err := final(ctx); err != nil {
			r.writeError(w, err)
		}
		return
	}
	w.WriteHeader(http.StatusNotFound)
	_ = jsonError(w, http.StatusNotFound, "not found")
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
	if errors.Is(err, ErrValidation) {
		w.WriteHeader(http.StatusBadRequest)
		_ = jsonErrorDetailed(w, http.StatusBadRequest, "validation_failed", err.Error(), nil, traceID)
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
