package gocontroller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestBindJSONRejectsOversizedBody(t *testing.T) {
	type dto struct {
		Name string `json:"name" validate:"required"`
	}

	router := NewRouter()
	router.SetMaxBodyBytes(8)
	router.POST("/users", func(ctx *Context) error {
		_, err := ParseDTO[dto](ctx)
		if err != nil {
			return err
		}
		return ctx.Created(map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alex"}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 got %d", w.Code)
	}
}

func TestBindJSONRejectsMultipleJSONValues(t *testing.T) {
	type dto struct {
		Name string `json:"name" validate:"required"`
	}

	router := NewRouter()
	router.POST("/users", func(ctx *Context) error {
		_, err := ParseDTO[dto](ctx)
		if err != nil {
			return err
		}
		return ctx.Created(map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alex"} {"name":"second"}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

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
	ctx := newContext(w, req, nil, DefaultValidator(), DefaultMaxBodyBytes)

	if err := ctx.BadRequest("oops"); err != nil {
		t.Fatalf("bad request helper: %v", err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	router := NewRouter()
	router.Use(RequestID())
	router.GET("/rid", func(ctx *Context) error {
		return ctx.OK(map[string]string{"request_id": ctx.RequestID()})
	})

	req := httptest.NewRequest(http.MethodGet, "/rid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if got := w.Header().Get(RequestIDHeader); got == "" {
		t.Fatalf("expected %s header to be set", RequestIDHeader)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	router := NewRouter()
	router.Use(RequestID(), Recovery(RecoveryConfig{}))
	router.GET("/panic", func(ctx *Context) error {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["success"] != false {
		t.Fatalf("expected success=false, got %v", payload["success"])
	}
	errObj, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in payload")
	}
	if errObj["code"] != "internal_panic" {
		t.Fatalf("expected internal_panic code, got %v", errObj["code"])
	}
	if errObj["trace_id"] == "" {
		t.Fatalf("expected trace_id to be present")
	}
}

func TestAPIErrorEnvelope(t *testing.T) {
	router := NewRouter()
	router.Use(RequestID())
	router.GET("/err", func(ctx *Context) error {
		return &APIError{
			StatusCode: http.StatusUnprocessableEntity,
			Code:       "unprocessable",
			Message:    "bad payload",
			Details: map[string]any{
				"field": "email",
			},
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 got %d", w.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	errObj, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in payload")
	}
	if errObj["code"] != "unprocessable" {
		t.Fatalf("expected unprocessable code, got %v", errObj["code"])
	}
}

func TestMethodNotAllowedReturns405(t *testing.T) {
	router := NewRouter()
	router.GET("/users", func(ctx *Context) error { return ctx.OK(map[string]any{"ok": true}) })

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 got %d", w.Code)
	}
	if allow := w.Header().Get("Allow"); !strings.Contains(allow, http.MethodGet) {
		t.Fatalf("expected Allow header to include GET, got %q", allow)
	}
}

func TestWildcardRoute(t *testing.T) {
	router := NewRouter()
	router.GET("/assets/*", func(ctx *Context) error {
		return ctx.OK(map[string]string{"path": ctx.Param("*")})
	})

	req := httptest.NewRequest(http.MethodGet, "/assets/img/icons/logo.svg", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var payload map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["path"] != "img/icons/logo.svg" {
		t.Fatalf("unexpected wildcard path: %q", payload["path"])
	}
}

func TestCORSMiddlewarePreflight(t *testing.T) {
	router := NewRouter()
	router.Use(CORS(CORSConfig{AllowOrigins: []string{"https://app.example.com"}}))
	router.GET("/x", func(ctx *Context) error { return ctx.OK(map[string]any{"ok": true}) })

	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("expected explicit CORS origin, got %q", got)
	}
}

func TestCORSMiddlewareDefaultDoesNotAllowAnyOrigin(t *testing.T) {
	router := NewRouter()
	router.Use(CORS(CORSConfig{}))
	router.GET("/x", func(ctx *Context) error { return ctx.OK(map[string]any{"ok": true}) })

	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no default CORS origin, got %q", got)
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	router := NewRouter()
	router.Use(SecurityHeaders())
	router.GET("/x", func(ctx *Context) error { return ctx.OK(map[string]any{"ok": true}) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("expected X-Frame-Options DENY")
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options nosniff")
	}
	if w.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("expected Content-Security-Policy header")
	}
}

func TestNewHTTPServerUsesTimeoutDefaults(t *testing.T) {
	app := &App{Router: NewRouter(), Container: NewContainer()}
	srv := app.NewHTTPServer(ServerOptions{})

	if srv.ReadHeaderTimeout <= 0 {
		t.Fatalf("expected ReadHeaderTimeout default")
	}
	if srv.ReadTimeout <= 0 {
		t.Fatalf("expected ReadTimeout default")
	}
	if srv.WriteTimeout <= 0 {
		t.Fatalf("expected WriteTimeout default")
	}
	if srv.IdleTimeout <= 0 {
		t.Fatalf("expected IdleTimeout default")
	}
	if srv.MaxHeaderBytes <= 0 {
		t.Fatalf("expected MaxHeaderBytes default")
	}
}

func TestCSRFMiddlewareSetsTokenAndRejectsMissingSubmission(t *testing.T) {
	router := NewRouter()
	router.Use(CSRF(CSRFConfig{Secret: []byte("01234567890123456789012345678901")}))
	router.GET("/form", func(ctx *Context) error {
		return ctx.OK(map[string]string{"csrf": ctx.CSRFToken()})
	})
	router.POST("/form", func(ctx *Context) error {
		return ctx.OK(map[string]string{"ok": "true"})
	})

	getReq := httptest.NewRequest(http.MethodGet, "/form", nil)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", getW.Code)
	}
	cookie := getW.Result().Cookies()[0]

	postReq := httptest.NewRequest(http.MethodPost, "/form", nil)
	postReq.AddCookie(cookie)
	postW := httptest.NewRecorder()
	router.ServeHTTP(postW, postReq)
	if postW.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", postW.Code)
	}
}

func TestSaveFileSanitizesUploadedFilename(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "../../evil.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("payload")); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	ctx := newContext(w, req, nil, DefaultValidator(), DefaultMaxBodyBytes)

	file, err := ctx.BindFile("file")
	if err != nil {
		t.Fatalf("bind file: %v", err)
	}
	dir := t.TempDir()
	path, err := SaveFile(dir, file)
	if err != nil {
		t.Fatalf("save file: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("expected saved file to stay in %q, got %q", dir, path)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil.txt")); !os.IsNotExist(err) {
		t.Fatalf("unexpected file outside upload dir")
	}
}

func TestModuleNameMiddleware(t *testing.T) {
	router := NewRouter()
	router.GET("/m", func(ctx *Context) error {
		v, _ := ctx.Get(moduleNameContextKey)
		name, _ := v.(string)
		return ctx.OK(map[string]string{"module": name})
	}, ModuleName("URLModule"))

	req := httptest.NewRequest(http.MethodGet, "/m", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var payload map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["module"] != "URLModule" {
		t.Fatalf("expected module URLModule got %q", payload["module"])
	}
}

func TestCustomErrorHandlerOverride(t *testing.T) {
	router := NewRouter()
	router.SetErrorHandler(func(ctx *Context, err error) {
		_ = ctx.JSON(http.StatusTeapot, map[string]any{
			"custom": true,
			"error":  err.Error(),
		})
	})
	router.GET("/err", func(ctx *Context) error {
		return fmt.Errorf("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTeapot {
		t.Fatalf("expected 418 got %d", w.Code)
	}
}

func TestRequireAndMustContextValue(t *testing.T) {
	type user struct{ ID string }
	key := "auth_user"

	router := NewRouter()
	router.GET("/me", func(ctx *Context) error {
		u, err := MustContextValue[*user](ctx, key, "")
		if err != nil {
			return err
		}
		return ctx.OK(map[string]string{"id": u.ID})
	}, RequireContextValue(key, "Unauthorized"))

	unauthReq := httptest.NewRequest(http.MethodGet, "/me", nil)
	unauthW := httptest.NewRecorder()
	router.ServeHTTP(unauthW, unauthReq)
	if unauthW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", unauthW.Code)
	}

	authReq := httptest.NewRequest(http.MethodGet, "/me", nil)
	authReq = authReq.WithContext(context.WithValue(authReq.Context(), key, &user{ID: "u1"}))
	authW := httptest.NewRecorder()
	router.ServeHTTP(authW, authReq)
	if authW.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", authW.Code)
	}
}
