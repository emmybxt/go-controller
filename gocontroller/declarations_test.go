package gocontroller

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestDeclarationBindingPreservesMiddlewareAndCopiesRoutes(t *testing.T) {
	calls := 0
	factory := func() any { calls++; return "custom" }
	d := Controllers("/accounts").UseBefore("controller").UseAfter("audit")
	d.GET("/:id", factory()).UseBefore("auth").UseAfter("route-audit")
	d.POST("")
	meta := d.MustBind(POST("", func() {}), GET("/:id", func() {}))
	if meta.Prefix != "/accounts" || meta.Routes[0].Method != "POST" {
		t.Fatal(meta)
	}
	want := []any{"custom", "auth", UseAfter("route-audit")}
	if !reflect.DeepEqual(meta.Routes[1].Middleware, want) {
		t.Fatal(meta.Routes[1].Middleware)
	}
	meta.Routes[1].Middleware[0] = "changed"
	meta.Middleware[0] = "changed"
	again := d.MustBind(GET("/:id", func() {}), POST("", func() {}))
	if calls != 1 || again.Middleware[0] != "controller" || !reflect.DeepEqual(again.Routes[0].Middleware, want) {
		t.Fatalf("calls=%d metadata=%+v", calls, again)
	}
}

func TestStaleDeclarationBindingsFailAtStartup(t *testing.T) {
	for _, tc := range []struct {
		name     string
		setup    func(*ControllerDeclaration)
		bindings []Route
	}{
		{"missing", func(d *ControllerDeclaration) { d.GET("/") }, nil},
		{"changed", func(d *ControllerDeclaration) { d.GET("/") }, []Route{GET("/old", func() {})}},
		{"duplicate declaration", func(d *ControllerDeclaration) { d.GET("/"); d.GET("/") }, []Route{GET("/", func() {})}},
		{"duplicate binding", func(d *ControllerDeclaration) { d.GET("/"); d.POST("/") }, []Route{GET("/", func() {}), GET("/", func() {})}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Controllers("/accounts")
			tc.setup(d)
			defer func() {
				if value := recover(); value == nil || !strings.Contains(fmt.Sprint(value), "run go generate") {
					t.Fatalf("got panic %v", value)
				}
			}()
			d.MustBind(tc.bindings...)
		})
	}
}

func TestDeclarationAllVerbs(t *testing.T) {
	d := Controllers("")
	methods := []func(string, ...any) *RouteDeclaration{d.GET, d.POST, d.PUT, d.PATCH, d.DELETE, d.HEAD, d.OPTIONS, d.CONNECT, d.TRACE}
	verbs := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE"}
	bindings := make([]Route, len(verbs))
	for i, method := range methods {
		method("/")
		bindings[i] = Route{Method: verbs[i], Path: "/", Handler: func() {}}
	}
	if got := d.MustBind(bindings...); len(got.Routes) != len(verbs) {
		t.Fatal(got)
	}
}
