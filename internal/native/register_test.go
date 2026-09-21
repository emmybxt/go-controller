package native

import (
	"strings"
	"testing"

	gc "github.com/emmybxt/go-controller/v2/gocontroller"
)

type handler func(string) string
type customHandler func(string) string
type middleware func(handler) handler

func TestRegisterConvertsNamedNativeFunctions(t *testing.T) {
	var registered handler
	err := Register([]gc.Route{gc.GET("/", customHandler(func(value string) string { return value + "!" }))},
		func(_, _ string, h handler, _, _ []middleware) { registered = h })
	if err != nil {
		t.Fatal(err)
	}
	if registered("native") != "native!" {
		t.Fatal("handler changed")
	}
}

func TestRegisterRejectsTypedNilFunctions(t *testing.T) {
	for _, route := range []gc.Route{
		gc.GET("/", handler(nil)),
		gc.GET("/", handler(func(value string) string { return value }), middleware(nil)),
	} {
		err := Register([]gc.Route{route}, func(_, _ string, _ handler, _, _ []middleware) { t.Fatal("registered invalid handler") })
		if err == nil || !strings.Contains(err.Error(), "non-nil") {
			t.Fatalf("got %v", err)
		}
	}
}

func TestRegisterPartitionsNestedMiddlewareGroups(t *testing.T) {
	var events []string
	mw := func(name string) middleware {
		return func(next handler) handler {
			return func(value string) string { events = append(events, name); return next(value) }
		}
	}
	route := gc.GET("/", handler(func(value string) string { events = append(events, "handler"); return value }),
		gc.UseBefore(mw("before"), gc.UseAfter(mw("nested-after"))),
		gc.UseAfter(gc.UseBefore(mw("outer-after"))),
	)
	err := Register([]gc.Route{route}, func(_, _ string, h handler, before, after []middleware) {
		if len(before) != 1 || len(after) != 2 {
			t.Fatalf("before=%d after=%d", len(before), len(after))
		}
		for _, m := range before {
			m(h)("")
		}
		for _, m := range after {
			m(func(v string) string { return v })("")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(events, ","); got != "before,handler,nested-after,outer-after" {
		t.Fatal(got)
	}
}

func TestRegisterRejectsNilAfterMiddlewareBeforeRegistration(t *testing.T) {
	err := Register([]gc.Route{gc.GET("/", handler(func(v string) string { return v }), gc.UseAfter(middleware(nil)))},
		func(_, _ string, _ handler, _, _ []middleware) { t.Fatal("registered invalid middleware") })
	if err == nil || !strings.Contains(err.Error(), "after middleware 1") {
		t.Fatal(err)
	}
}
