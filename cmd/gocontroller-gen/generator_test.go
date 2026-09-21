package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeSource(t *testing.T, dir, name, source string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateDiscoversMethodsAcrossFilesAndBindsNativeMethods(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "a-methods.go", `package example
// @Get("/:id")
func (c *Books) Get(ctx *NativeContext) {}
// @Patch("/featured")
// @Use(Authorize("books", "write"), Audit(Config{Fields: []string{"a", "b"}}))
func (c *Books) Update(ctx *NativeContext) {}
// @Get("/*path")
func (c *Books) Files(ctx *NativeContext) {}
`)
	writeSource(t, dir, "z-controller.go", "package example\n// @Controller(\"/books\")\n// @Use(ControllerMiddleware())\ntype Books struct{}\n")
	writeSource(t, dir, "ignored_test.go", "package example_test\n// @Controller(\"/ignored\")\ntype Ignored struct{}\n")
	writeSource(t, dir, "ignored.gen.go", "package example\n// @Controller(\"/ignored\")\ntype Generated struct{}\n")
	writeSource(t, dir, "excluded.go", "//go:build ignore\n\npackage excluded\n")
	pkg, controllers, err := parseControllers(dir)
	if err != nil {
		t.Fatal(err)
	}
	if pkg != "example" || len(controllers) != 1 {
		t.Fatalf("%s %+v", pkg, controllers)
	}
	routes := controllers[0].Routes
	if len(routes) != 3 {
		t.Fatal(routes)
	}
	if got := []string{routes[0].Path, routes[1].Path, routes[2].Path}; !reflect.DeepEqual(got, []string{"/featured", "/:id", "/*path"}) {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(routes[0].Middleware, []string{`Authorize("books", "write")`, `Audit(Config{Fields: []string{"a", "b"}})`}) {
		t.Fatal(routes[0].Middleware)
	}
	first, err := generateCode(pkg, controllers)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"func (c *Books) ControllerMetadata()", `gocontroller.PATCH("/featured", c.Update, Authorize(`,
		`gocontroller.GET("/:id", c.Get)`, "Middleware: []any{ControllerMiddleware()}",
		`github.com/emmybxt/go-controller/v2/gocontroller`,
	} {
		if !strings.Contains(string(first), fragment) {
			t.Fatalf("missing %q in:\n%s", fragment, first)
		}
	}
	for range 5 {
		pkg, controllers, err := parseControllers(dir)
		if err != nil {
			t.Fatal(err)
		}
		output, err := generateCode(pkg, controllers)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, output) {
			t.Fatal("generation is nondeterministic")
		}
	}
}

func TestGenerateAllHTTPVerbs(t *testing.T) {
	dir := t.TempDir()
	var source strings.Builder
	source.WriteString("package example\n// @Controller(\"\")\ntype Example struct{}\n")
	for _, verb := range routeVerbs {
		source.WriteString("// @" + verb + "(\"/" + verb + "\")\nfunc (c *Example) " + verb + "(ctx *NativeContext) {}\n")
	}
	writeSource(t, dir, "example.go", source.String())
	if err := run(dir, "routes.gen.go", false); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, "routes.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, verb := range routeVerbs {
		if !strings.Contains(string(first), "gocontroller."+strings.ToUpper(verb)+"(") {
			t.Fatal(verb)
		}
	}
	if err := run(dir, "routes.gen.go", false); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, "routes.gen.go"))
	if !bytes.Equal(first, second) {
		t.Fatal("rerun changed output")
	}
}

func TestGeneratorReturnsActionableInputErrors(t *testing.T) {
	cases := []struct{ name, source, message string }{
		{"unquoted prefix", "// @Controller(/books)\ntype Books struct{}", "prefix"},
		{"unquoted route", "// @Controller(\"\")\ntype Books struct{}\n// @Get(/books)\nfunc (*Books) Get() {}", "path"},
		{"invalid middleware", "// @Controller(\"\")\n// @Use(Authorize(,) )\ntype Books struct{}", "@Use"},
		{"non call middleware", "// @Controller(\"\")\n// @Use() + other()\ntype Books struct{}", "@Use"},
		{"metadata exists", "// @Controller(\"\")\ntype Books struct{}\nfunc (*Books) ControllerMetadata() {}", "already implements"},
		{"generic controller", "// @Controller(\"\")\ntype Books[T any] struct{}", "generic controllers"},
		{"alias", "// @Controller(\"\")\ntype Books = struct{}", "struct declaration"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeSource(t, dir, "example.go", "package example\n"+tc.source+"\n")
			_, _, err := parseControllers(dir)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("got %v want %s", err, tc.message)
			}
		})
	}
}

func TestRouteOrderingIsTransitive(t *testing.T) {
	routes := []routeInfo{
		{Path: "/a/:id"}, {Path: "/b"}, {Path: "/b/static"}, {Path: "/b/*rest"}, {Path: "/a/:id/deep"},
	}
	for _, a := range routes {
		for _, b := range routes {
			for _, c := range routes {
				if routeLess(a, b) && routeLess(b, c) && !routeLess(a, c) {
					t.Fatalf("nontransitive: %s %s %s", a.Path, b.Path, c.Path)
				}
			}
		}
	}
}

func TestCheckDoesNotRewriteStaleOutput(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "example.go", "package example\n// @Controller(\"\")\ntype Books struct{}\n")
	if err := run(dir, "routes.gen.go", false); err != nil {
		t.Fatal(err)
	}
	if err := run(dir, "routes.gen.go", true); err != nil {
		t.Fatal(err)
	}
	stale := []byte("// stale output\n")
	writeSource(t, dir, "routes.gen.go", string(stale))
	if err := run(dir, "routes.gen.go", true); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("got %v", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "routes.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stale, after) {
		t.Fatal("check rewrote the output")
	}
}
