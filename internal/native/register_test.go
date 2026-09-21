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
		func(_, _ string, h handler, _ []middleware) { registered = h })
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
		err := Register([]gc.Route{route}, func(_, _ string, _ handler, _ []middleware) { t.Fatal("registered invalid handler") })
		if err == nil || !strings.Contains(err.Error(), "non-nil") {
			t.Fatalf("got %v", err)
		}
	}
}
