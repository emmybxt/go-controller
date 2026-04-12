package gocontroller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testService struct{}

func (s *testService) Name() string { return "svc" }

type testController struct {
	svc *testService
}

func newTestController(svc *testService) *testController {
	return &testController{svc: svc}
}

func (c *testController) RegisterRoutes(g *RouteGroup) {
	g.GET("/hello/:id", c.hello)
	g.POST("/users", c.createUser, func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			if ctx.Request.Header.Get("x-auth") == "" {
				return NewHTTPError(http.StatusUnauthorized, "missing auth")
			}
			return next(ctx)
		}
	})
}

func (c *testController) hello(ctx *Context) error {
	return ctx.JSON(http.StatusOK, map[string]any{"id": ctx.Param("id"), "svc": c.svc.Name()})
}

func (c *testController) createUser(ctx *Context) error {
	type dto struct {
		Name string `json:"name" validate:"required,min=2"`
	}
	payload, err := ParseDTO[dto](ctx)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusCreated, map[string]string{"name": payload.Name})
}

func TestModuleDIAndValidation(t *testing.T) {
	mod := &Module{
		Prefix:      "/api",
		Providers:   []any{func() *testService { return &testService{} }},
		Controllers: []any{newTestController},
	}
	app, err := NewApp(mod)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/hello/42", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":"a"}`))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.Header.Set("x-auth", "ok")
	badW := httptest.NewRecorder()
	app.Router.ServeHTTP(badW, badReq)
	if badW.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", badW.Code)
	}

	unauthReq := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":"alex"}`))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthW := httptest.NewRecorder()
	app.Router.ServeHTTP(unauthW, unauthReq)
	if unauthW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", unauthW.Code)
	}
}

type decoratedController struct {
	svc *testService
}

func newDecoratedController(svc *testService) *decoratedController {
	return &decoratedController{svc: svc}
}

func (c *decoratedController) ControllerMetadata() ControllerMetadata {
	return ControllerMetadata{
		Prefix: "/decorated",
		Routes: []RouteMetadata{
			GET("/hello/:id", "Hello"),
			POST("/users", "CreateUser", func(next HandlerFunc) HandlerFunc {
				return func(ctx *Context) error {
					if ctx.Request.Header.Get("x-auth") == "" {
						return NewHTTPError(http.StatusUnauthorized, "missing auth")
					}
					return next(ctx)
				}
			}),
		},
	}
}

func (c *decoratedController) Hello(ctx *Context) error {
	return ctx.JSON(http.StatusOK, map[string]any{"id": ctx.Param("id"), "svc": c.svc.Name()})
}

func (c *decoratedController) CreateUser(ctx *Context) error {
	type dto struct {
		Name string `json:"name" validate:"required,min=2"`
	}
	payload, err := ParseDTO[dto](ctx)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusCreated, map[string]string{"name": payload.Name})
}

func TestDecoratedControllerMetadata(t *testing.T) {
	mod := &Module{
		Prefix:      "/api",
		Providers:   []any{func() *testService { return &testService{} }},
		Controllers: []any{newDecoratedController},
	}
	app, err := NewApp(mod)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/decorated/hello/42", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/decorated/users", strings.NewReader(`{"name":"a"}`))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.Header.Set("x-auth", "ok")
	badW := httptest.NewRecorder()
	app.Router.ServeHTTP(badW, badReq)
	if badW.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", badW.Code)
	}

	unauthReq := httptest.NewRequest(http.MethodPost, "/api/decorated/users", strings.NewReader(`{"name":"alex"}`))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthW := httptest.NewRecorder()
	app.Router.ServeHTTP(unauthW, unauthReq)
	if unauthW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", unauthW.Code)
	}
}

type generatedController struct {
	svc *testService
}

func newGeneratedController(svc *testService) *generatedController {
	return &generatedController{svc: svc}
}

func (c *generatedController) Hello(ctx *Context) error {
	return ctx.JSON(http.StatusOK, map[string]any{"id": ctx.Param("id"), "svc": c.svc.Name()})
}

func init() {
	RegisterGeneratedControllerMetadata((*generatedController)(nil), ControllerMetadata{
		Prefix: "/generated",
		Routes: []RouteMetadata{
			GET("/hello/:id", "Hello"),
		},
	})
}

func TestGeneratedControllerMetadataRegistry(t *testing.T) {
	mod := &Module{
		Prefix:      "/api",
		Providers:   []any{func() *testService { return &testService{} }},
		Controllers: []any{newGeneratedController},
	}
	app, err := NewApp(mod)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/generated/hello/77", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

func TestValidateSupportsAdvancedTags(t *testing.T) {
	type advancedDTO struct {
		ID     string   `json:"id" validate:"required,uuid4"`
		Role   string   `json:"role" validate:"oneof=admin user viewer"`
		Emails []string `json:"emails" validate:"min=1,dive,email"`
	}

	err := Validate(advancedDTO{
		ID:     "bad-id",
		Role:   "superadmin",
		Emails: []string{"not-an-email"},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

type failingValidator struct{}

func (f failingValidator) Validate(v any) error {
	return fmt.Errorf("%w: forced failure", ErrValidation)
}

func TestAppCustomValidator(t *testing.T) {
	type dto struct {
		Name string `json:"name" validate:"required"`
	}

	handler := func(ctx *Context) error {
		_, err := ParseDTO[dto](ctx)
		if err != nil {
			return err
		}
		return ctx.JSON(http.StatusCreated, map[string]string{"ok": "true"})
	}

	mod := &Module{
		Prefix: "/api",
		Controllers: []any{
			func() Controller {
				return controllerFunc(func(g *RouteGroup) {
					g.POST("/custom", handler)
				})
			},
		},
	}
	app, err := NewApp(mod)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	app.SetValidator(failingValidator{})

	req := httptest.NewRequest(http.MethodPost, "/api/custom", strings.NewReader(`{"name":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

type controllerFunc func(*RouteGroup)

func (f controllerFunc) RegisterRoutes(g *RouteGroup) { f(g) }

func TestAdaptHTTPMiddleware(t *testing.T) {
	const key = "user-id"
	httpMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key, "u-123")))
		})
	}

	var got string
	router := NewRouter()
	router.GET("/x", func(ctx *Context) error {
		v := ctx.Request.Context().Value(key)
		got, _ = v.(string)
		return ctx.OK(map[string]string{"ok": "yes"})
	}, AdaptHTTPMiddleware(httpMW))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if got != "u-123" {
		t.Fatalf("expected adapted context value, got %q", got)
	}
}

func TestWebAPIHandlerRouting(t *testing.T) {
	web := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("web")) })
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("api")) })
	handler := WebAPIHandler(web, api, HybridOptions{
		WebExactPaths:              []string{"/"},
		WebPathPrefixes:            []string{"/app", "/css/", "/js/"},
		TreatSingleSegmentGETAsWeb: true,
	})

	check := func(method, path, want string) {
		req := httptest.NewRequest(method, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		body, _ := io.ReadAll(w.Body)
		if string(body) != want {
			t.Fatalf("%s %s expected %q got %q", method, path, want, string(body))
		}
	}

	check(http.MethodGet, "/", "web")
	check(http.MethodGet, "/app/dashboard", "web")
	check(http.MethodGet, "/abc123", "web")
	check(http.MethodPost, "/url/shorten", "api")
}

func TestContextResponseHelpers(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	ctx := newContext(w, req, nil, DefaultValidator())

	if err := ctx.BadRequest("oops"); err != nil {
		t.Fatalf("bad request helper: %v", err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}
