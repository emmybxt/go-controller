package adapters_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	gc "github.com/emmybxt/go-controller/v2/gocontroller"
)

var nativeFailure = errors.New("native failure")

type harness struct {
	adapter      gc.Adapter
	request      func(*http.Request) *http.Response
	handler      any
	create       any
	failure      any
	afterFailure any
	abort        any
	middleware   func(string) any
	events       *[]string
}

type controller struct{ meta gc.ControllerMetadata }

func (c *controller) ControllerMetadata() gc.ControllerMetadata { return c.meta }

var frameworks = []struct {
	name  string
	setup func(*testing.T) harness
}{
	{"gin", setupGin}, {"echo-v5", setupEcho}, {"echo-v4", setupEchoV4},
	{"fiber-v3", setupFiber}, {"fiber-v2", setupFiberV2},
}

func TestNativeFrameworkIntegration(t *testing.T) {
	for _, framework := range frameworks {
		t.Run(framework.name, func(t *testing.T) {
			h := framework.setup(t)
			module := &gc.Module{
				Name: "Books", Prefix: "/api", Middleware: []any{h.middleware("module")},
				Controllers: []any{&controller{gc.ControllerMetadata{
					Prefix: "/books", Middleware: []any{h.middleware("controller")},
					Routes: []gc.Route{
						gc.POST("", h.create),
						gc.GET("/blocked", h.handler, h.abort),
						gc.GET("/failure", h.failure),
						gc.GET("/:id", h.handler, h.middleware("route")),
					},
				}}},
			}
			if err := gc.Mount(h.adapter, module); err != nil {
				t.Fatal(err)
			}
			assertResponse(t, h, "GET", "/host/api/books/42?filter=bar", "", 200, "42|bar|native")
			want := []string{"global:before", "group:before", "module:before", "controller:before", "route:before", "handler", "route:after", "controller:after", "module:after", "group:after", "global:after"}
			if !reflect.DeepEqual(*h.events, want) {
				t.Fatalf("middleware order: %v", *h.events)
			}
			*h.events = nil
			assertResponse(t, h, "GET", "/host/api/books/blocked", "", 401, "blocked")
			for _, event := range *h.events {
				if event == "handler" {
					t.Fatal("abort did not stop handler")
				}
			}
			assertResponse(t, h, "POST", "/host/api/books", `{"title":"Go"}`, 201, "Go")
			assertResponse(t, h, "GET", "/host/api/books/failure", "", 418, "native-error")
			assertResponse(t, h, "GET", "/plain", "", 200, "plain")
		})
	}
}

func TestAdaptersValidateWholeBatchBeforeRegistration(t *testing.T) {
	for _, framework := range frameworks {
		for _, invalid := range []string{"handler", "middleware", "nil handler", "nil middleware", "after middleware"} {
			t.Run(framework.name+"/"+invalid, func(t *testing.T) {
				h := framework.setup(t)
				bad := gc.GET("/bad", h.handler)
				switch invalid {
				case "handler":
					bad.Handler = func(int) {}
				case "middleware":
					bad.Middleware = []any{func(int) {}}
				case "nil handler":
					bad.Handler = nil
				case "after middleware":
					bad.Middleware = []any{gc.UseAfter(func(int) {})}
				case "nil middleware":
					bad.Middleware = []any{nil}
				}
				module := &gc.Module{Controllers: []any{&controller{gc.ControllerMetadata{Routes: []gc.Route{gc.GET("/valid", h.handler), bad}}}}}
				if err := gc.Mount(h.adapter, module); err == nil || !strings.Contains(err.Error(), "/bad") {
					t.Fatalf("got %v", err)
				}
				response := h.request(httptest.NewRequest("GET", "/host/valid", nil))
				defer response.Body.Close()
				if response.StatusCode != 404 {
					t.Fatalf("valid route registered before validation finished: %d", response.StatusCode)
				}
			})
		}
	}
}

func assertResponse(t *testing.T, h harness, method, path, body string, status int, want string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	response := h.request(req)
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status || string(data) != want {
		t.Fatalf("%s %s: got %d %q, want %d %q", method, path, response.StatusCode, data, status, want)
	}
}

func httpRequester(handler http.Handler) func(*http.Request) *http.Response {
	return func(req *http.Request) *http.Response {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response.Result()
	}
}
