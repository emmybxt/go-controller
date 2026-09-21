package gocontroller_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	gc "github.com/emmybxt/go-controller/v2/gocontroller"
)

type captureAdapter struct {
	routes []gc.Route
	calls  int
}

func (a *captureAdapter) Register(routes []gc.Route) error {
	a.routes = routes
	a.calls++
	return nil
}

type testController struct{ meta gc.ControllerMetadata }

func (c *testController) ControllerMetadata() gc.ControllerMetadata { return c.meta }

type greeting interface{ Greet() string }
type service struct{ message string }

func (s *service) Greet() string { return s.message }

type otherService struct{}

func (*otherService) Greet() string { return "other" }

type dependencyA struct{}
type dependencyB struct{}

func TestMountInjectsSharedProvidersAndComposesRoutes(t *testing.T) {
	created := 0
	shared := &gc.Module{Name: "Services", Providers: []any{
		func() *service { created++; return &service{"hello"} },
	}}
	var received []greeting
	newController := func(s greeting) *testController {
		received = append(received, s)
		return &testController{gc.ControllerMetadata{
			Prefix: "books", Middleware: []any{"controller"},
			Routes: []gc.Route{gc.GET("/:id", s.Greet, "route")},
		}}
	}
	first := &gc.Module{Name: "First", Prefix: "/v1", Imports: []*gc.Module{shared}, Controllers: []any{newController}, Middleware: []any{"child"}}
	second := &gc.Module{Name: "Second", Prefix: "/v2", Imports: []*gc.Module{shared}, Controllers: []any{newController}}
	// Spare capacity catches accidental append mutations between sibling modules.
	middleware := make([]any, 1, 10)
	middleware[0] = "root"
	root := &gc.Module{Prefix: "/api/", Imports: []*gc.Module{first, second}, Middleware: middleware}
	adapter := &captureAdapter{}
	if err := gc.Mount(adapter, root); err != nil {
		t.Fatal(err)
	}
	if created != 1 || len(received) != 2 || received[0] != received[1] {
		t.Fatalf("providers not shared: created=%d received=%v", created, received)
	}
	if len(adapter.routes) != 2 {
		t.Fatalf("routes: %+v", adapter.routes)
	}
	for i, path := range []string{"/api/v1/books/:id", "/api/v2/books/:id"} {
		if adapter.routes[i].Path != path {
			t.Errorf("path: got %q want %q", adapter.routes[i].Path, path)
		}
		if adapter.routes[i].Handler.(func() string)() != "hello" {
			t.Error("bound service handler failed")
		}
	}
	if !reflect.DeepEqual(adapter.routes[0].Middleware, []any{"root", "child", "controller", "route"}) {
		t.Error(adapter.routes[0].Middleware)
	}
	if !reflect.DeepEqual(adapter.routes[1].Middleware, []any{"root", "controller", "route"}) {
		t.Error(adapter.routes[1].Middleware)
	}
	if len(root.Middleware) != 1 || root.Middleware[0] != "root" {
		t.Error("module mutated")
	}
}

func TestMountResolvesProvidersBeforeAnyControllerRegardlessOfDeclarationOrder(t *testing.T) {
	a := &captureAdapter{}
	root := &gc.Module{
		Providers: []any{func(s *service) greeting { return s }, func() *service { return &service{"resolved"} }},
		Imports: []*gc.Module{{Controllers: []any{func(s greeting) *testController {
			return &testController{gc.ControllerMetadata{Routes: []gc.Route{gc.GET("/", s.Greet)}}}
		}}}},
	}
	if err := gc.Mount(a, root); err != nil {
		t.Fatal(err)
	}
	if a.routes[0].Handler.(func() string)() != "resolved" {
		t.Fatal("wrong provider")
	}
}

func TestMountRejectsInvalidConfigurationBeforeRegistering(t *testing.T) {
	controller := func(*service) *testController { return &testController{} }
	cycle := &gc.Module{Name: "cycle"}
	cycle.Imports = []*gc.Module{cycle}
	cases := []struct {
		name    string
		module  *gc.Module
		message string
	}{
		{"nil module", nil, "module is nil"},
		{"nil import", &gc.Module{Imports: []*gc.Module{nil}}, "module is nil"},
		{"module cycle", cycle, "circular module import"},
		{"missing dependency", &gc.Module{Controllers: []any{controller}}, "no provider"},
		{"duplicate provider", &gc.Module{Providers: []any{&service{}, func() *service { return &service{} }}}, "duplicate provider"},
		{"nil provider", &gc.Module{Providers: []any{nil}}, "provider is nil"},
		{"typed nil provider", &gc.Module{Providers: []any{(*service)(nil)}}, "provider is nil"},
		{"nil factory", &gc.Module{Providers: []any{(func() *service)(nil)}}, "provider is nil"},
		{"nil controller", &gc.Module{Controllers: []any{nil}}, "controller is nil"},
		{"typed nil controller", &gc.Module{Controllers: []any{(*testController)(nil)}}, "controller is nil"},
		{"wrong controller", &gc.Module{Controllers: []any{&service{}}}, "must implement ControllerMetadata"},
		{"zero outputs", &gc.Module{Providers: []any{func() {}}}, "must return T"},
		{"wrong second output", &gc.Module{Controllers: []any{func() (*testController, int) { return nil, 0 }}}, "second return must be error"},
		{"variadic", &gc.Module{Providers: []any{func(...string) *service { return nil }}}, "must not be variadic"},
		{"nil result", &gc.Module{Providers: []any{func() *service { return nil }}, Controllers: []any{controller}}, "returned nil"},
		{"nil interface result", &gc.Module{Providers: []any{func() greeting { return (*service)(nil) }}, Controllers: []any{func(greeting) *testController { return nil }}}, "returned nil"},
		{"nil controller result", &gc.Module{Controllers: []any{func() *testController { return nil }}}, "returned nil"},
		{"ambiguity", &gc.Module{Providers: []any{&service{}, func() *otherService { return &otherService{} }}, Controllers: []any{func(greeting) *testController { return nil }}}, "ambiguous providers"},
		{"provider cycle", &gc.Module{Providers: []any{func(*dependencyB) *dependencyA { return nil }, func(*dependencyA) *dependencyB { return nil }}, Controllers: []any{func(*dependencyA) *testController { return nil }}}, "circular provider dependency"},
		{"duplicate route", &gc.Module{Controllers: []any{&testController{gc.ControllerMetadata{Routes: []gc.Route{gc.GET("/x", func() {}), gc.GET("/x", func() {})}}}}}, "duplicate route GET /x"},
		{"bad method", &gc.Module{Controllers: []any{&testController{gc.ControllerMetadata{Routes: []gc.Route{{Method: "GET /", Path: "/"}}}}}}, "invalid HTTP method"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &captureAdapter{}
			err := gc.Mount(a, tc.module)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("got %v, want %q", err, tc.message)
			}
			if a.calls != 0 {
				t.Fatal("registered routes despite configuration error")
			}
		})
	}
}

func TestMountPreservesConstructorErrors(t *testing.T) {
	want := errors.New("database unavailable")
	for _, module := range []*gc.Module{
		{Controllers: []any{func() (*testController, error) { return nil, want }}},
		{Providers: []any{func() (*service, error) { return nil, want }}, Controllers: []any{func(*service) *testController { return nil }}},
	} {
		if err := gc.Mount(&captureAdapter{}, module); !errors.Is(err, want) {
			t.Fatalf("got %v", err)
		}
	}
}

func TestMountExplicitInterfaceBindingWins(t *testing.T) {
	a := &captureAdapter{}
	root := &gc.Module{
		Providers: []any{&otherService{}, &service{"chosen"}, func(s *service) greeting { return s }},
		Controllers: []any{func(s greeting) (*testController, error) {
			return &testController{gc.ControllerMetadata{Routes: []gc.Route{gc.GET("/", s.Greet)}}}, nil
		}},
	}
	if err := gc.Mount(a, root); err != nil {
		t.Fatal(err)
	}
	if got := a.routes[0].Handler.(func() string)(); got != "chosen" {
		t.Fatal(got)
	}
}

func TestMountRejectsNilAdapter(t *testing.T) {
	for _, adapter := range []gc.Adapter{nil, (*captureAdapter)(nil)} {
		if err := gc.Mount(adapter, &gc.Module{}); err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestMountPreservesNativePaths(t *testing.T) {
	for _, path := range []string{"", "/", "/:id?", "/files/*rest", "/with/slash/", "/:id<int>"} {
		t.Run(path, func(t *testing.T) {
			a := &captureAdapter{}
			module := &gc.Module{Prefix: "api", Controllers: []any{&testController{gc.ControllerMetadata{Routes: []gc.Route{gc.GET(path, func() {})}}}}}
			if err := gc.Mount(a, module); err != nil {
				t.Fatal(err)
			}
			if a.routes[0].Path != "/api"+path {
				t.Fatal(a.routes[0].Path)
			}
		})
	}
}
