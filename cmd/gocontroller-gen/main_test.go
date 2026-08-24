package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseControllersFindsCrossFileRoutesAndAllSupportedVerbs(t *testing.T) {
	dir := t.TempDir()
	writeGeneratorFixture(t, dir, "a_routes.go", `package fixture

// @Get("/:id")
func (c *LinkController) GetByID(ctx any) error { return nil }

// @Patch("/workspace/stats")
func (c *LinkController) PatchWorkspace(ctx any) error { return nil }

// @Post("/")
func (c *LinkController) Create(ctx any) error { return nil }

// @Put("/:id")
func (c *LinkController) Replace(ctx any) error { return nil }

// @Delete("/:id")
func (c *LinkController) Delete(ctx any) error { return nil }

// @Options("/:id")
func (c *LinkController) Options(ctx any) error { return nil }
`)
	writeGeneratorFixture(t, dir, "z_controller.go", `package fixture

// @Controller("/links")
type LinkController struct{}
`)

	packageName, controllers, err := parseControllers(dir)
	if err != nil {
		t.Fatalf("parse controllers: %v", err)
	}
	if packageName != "fixture" || len(controllers) != 1 {
		t.Fatalf("unexpected package/controllers: package=%q controllers=%#v", packageName, controllers)
	}

	routes := controllers[0].Routes
	if len(routes) != 6 {
		t.Fatalf("expected all six routes, got %#v", routes)
	}
	if routes[0].Path != "/workspace/stats" || routes[0].Method != "PATCH" {
		t.Fatalf("static route must be ordered before parameter routes, got %#v", routes)
	}

	generated := generateCode(packageName, controllers)
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		if !strings.Contains(generated, "gocontroller."+method+"(") {
			t.Fatalf("generated metadata is missing %s route:\n%s", method, generated)
		}
	}
}

func TestParseControllersIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	writeGeneratorFixture(t, dir, "b_routes.go", `package fixture

// @Get("/beta")
func (c *FixtureController) Beta(ctx any) error { return nil }
`)
	writeGeneratorFixture(t, dir, "a_routes.go", `package fixture

// @Get("/alpha")
func (c *FixtureController) Alpha(ctx any) error { return nil }
`)
	writeGeneratorFixture(t, dir, "z_controller.go", `package fixture

// @Controller("")
type FixtureController struct{}
`)

	var expected string
	for iteration := 0; iteration < 25; iteration++ {
		packageName, controllers, err := parseControllers(dir)
		if err != nil {
			t.Fatalf("parse controllers: %v", err)
		}
		generated := generateCode(packageName, controllers)
		if iteration == 0 {
			expected = generated
			continue
		}
		if generated != expected {
			t.Fatal("generator output changed between identical runs")
		}
	}
}

func writeGeneratorFixture(t *testing.T, dir, name, source string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}
