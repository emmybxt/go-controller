package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const declarationHeader = `package example
import gc "github.com/emmybxt/go-controller/v2/gocontroller"
var accounts = gc.Controllers("/accounts").UseBefore(auth()).UseAfter(audit())
type Accounts struct{}
`

func TestDeclarationsBindFollowingMethodsAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "z-controller.go", declarationHeader)
	var methods strings.Builder
	methods.WriteString("package example\n")
	for i, verb := range routeVerbs {
		receiver := "*Accounts"
		if i%2 == 0 {
			receiver = "Accounts"
		}
		methods.WriteString("var _ = accounts." + strings.ToUpper(verb) + "(\"/" + verb + "\").UseBefore(external.Factory()).UseAfter(audit())\nfunc (a " + receiver + ") " + verb + "(ctx *NativeContext) {}\n")
	}
	writeSource(t, dir, "a-methods.go", methods.String())
	if err := run(dir, "routes.gen.go", false); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(filepath.Join(dir, "routes.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "return accounts.MustBind(") {
		t.Fatal(string(output))
	}
	if strings.Contains(string(output), "external") || strings.Contains(string(output), "audit()") {
		t.Fatal("middleware factories were copied into generated code")
	}
	for _, verb := range routeVerbs {
		if !strings.Contains(string(output), "gocontroller."+strings.ToUpper(verb)+"(\"/"+verb+"\", c."+verb+")") {
			t.Fatal(string(output))
		}
	}
	for range 3 {
		if err := run(dir, "routes.gen.go", true); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDeclarationErrors(t *testing.T) {
	cases := []struct{ name, source, message string }{
		{"unbound", "", "has no routes"},
		{"missing method", `var _ = accounts.GET("/")`, "immediately precede"},
		{"free function", "var _ = accounts.GET(\"/\")\nfunc Get() {}", "immediately precede"},
		{"intervening declaration", "var _ = accounts.GET(\"/\")\nvar x = 1\nfunc (*Accounts) Get() {}", "immediately precede"},
		{"grouped", "var (_ = accounts.GET(\"/\"))\nfunc (*Accounts) Get() {}", "ungrouped"},
		{"named", "var route = accounts.GET(\"/\")\nfunc (*Accounts) Get() {}", "var _"},
		{"dynamic path", "var _ = accounts.GET(path)\nfunc (*Accounts) Get() {}", "string literal"},
		{"missing path", "var _ = accounts.GET()\nfunc (*Accounts) Get() {}", "quoted path"},
		{"unknown verb", "var _ = accounts.Get(\"/\")\nfunc (*Accounts) Get() {}", "supported route"},
		{"unknown modifier", "var _ = accounts.GET(\"/\").Named(\"x\")\nfunc (*Accounts) Get() {}", "unsupported declaration modifier"},
		{"duplicate", "var _ = accounts.GET(\"/\")\nfunc (*Accounts) Get() {}\nvar _ = accounts.GET(\"/\")\nfunc (*Accounts) Other() {}", "duplicate route"},
		{"mixed annotations", "var _ = accounts.GET(\"/\")\n// @Get(\"/other\")\nfunc (*Accounts) Get() {}", "mixes route declarations"},
		{"multiple receivers", "var _ = accounts.GET(\"/\")\nfunc (*Accounts) Get() {}\ntype Other struct{}\nvar _ = accounts.POST(\"/\")\nfunc (*Other) Post() {}", "cannot describe both"},
		{"multiple owners", "var other = gc.Controllers(\"/other\")\nvar _ = accounts.GET(\"/\")\nfunc (*Accounts) Get() {}\nvar _ = other.POST(\"/\")\nfunc (*Accounts) Post() {}", "one declaration variable"},
		{"handwritten", "var _ = accounts.GET(\"/\")\nfunc (*Accounts) Get() {}\nfunc (*Accounts) ControllerMetadata() {}", "already implements"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeSource(t, dir, "controller.go", declarationHeader+tc.source)
			_, _, err := parseControllers(dir)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("got %v want %s", err, tc.message)
			}
		})
	}
}

func TestDeclarationNameDoesNotShadowGeneratedReceiver(t *testing.T) {
	dir := t.TempDir()
	source := strings.ReplaceAll(declarationHeader, "accounts", "c") + "var _ = c.GET(\"/\")\nfunc (*Accounts) Get() {}"
	writeSource(t, dir, "controller.go", source)
	if err := run(dir, "routes.gen.go", false); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(filepath.Join(dir, "routes.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"func (controller *Accounts)", "return c.MustBind(", "controller.Get"} {
		if !strings.Contains(string(output), fragment) {
			t.Fatal(string(output))
		}
	}
}
