package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/emmybxt/go-controller/gocontroller"
)

type SomethingService struct{}

func NewSomethingService() *SomethingService {
	return &SomethingService{}
}

func (s *SomethingService) BuildMessage(id string) string {
	return fmt.Sprintf("resource %s loaded", id)
}

type CreateSomethingDTO struct {
	Name  string `json:"name" validate:"required,min=2,max=32"`
	Email string `json:"email" validate:"required,email"`
}

// @Controller("")
type SomethingController struct {
	service *SomethingService
}

func NewSomethingController(service *SomethingService) *SomethingController {
	return &SomethingController{service: service}
}

// @Get("/hello")
func (c *SomethingController) Hello(ctx *gocontroller.Context) error {
	return ctx.JSON(http.StatusOK, map[string]any{
		"message": "Hello, World!",
	})
}

// @Get("/something/:id")
func (c *SomethingController) GetSomething(ctx *gocontroller.Context) error {
	id := ctx.Param("id")
	return ctx.JSON(http.StatusOK, map[string]any{
		"id":      id,
		"message": c.service.BuildMessage(id),
	})
}

// @Post("/something")
// @Use(APIKeyMiddleware("local-dev-key"))
func (c *SomethingController) CreateSomething(ctx *gocontroller.Context) error {
	dto, err := gocontroller.ParseDTO[CreateSomethingDTO](ctx)
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusCreated, map[string]any{
		"name":       dto.Name,
		"email":      dto.Email,
		"created_at": time.Now().Format(time.RFC3339),
	})
}

func RequestLogger() gocontroller.Middleware {
	return func(next gocontroller.HandlerFunc) gocontroller.HandlerFunc {
		return func(ctx *gocontroller.Context) error {
			start := time.Now()
			err := next(ctx)
			log.Printf("%s %s (%s)", ctx.Request.Method, ctx.Request.URL.Path, time.Since(start))
			return err
		}
	}
}

func APIKeyMiddleware(expected string) gocontroller.Middleware {
	return func(next gocontroller.HandlerFunc) gocontroller.HandlerFunc {
		return func(ctx *gocontroller.Context) error {
			if ctx.Request.Header.Get("x-api-key") != expected {
				return gocontroller.NewHTTPError(http.StatusUnauthorized, "invalid api key")
			}
			return next(ctx)
		}
	}
}

//go:generate go run ../cmd/gocontroller-gen -dir . -out zz_gocontroller_routes.gen.go

func main() {
	root := &gocontroller.Module{
		Name:      "SomethingModule",
		Prefix:    "/api",
		Providers: []any{NewSomethingService},
		Controllers: []any{
			NewSomethingController,
		},
		Middleware: []gocontroller.Middleware{RequestLogger()},
	}

	app, err := gocontroller.NewApp(root)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("server started on :8080")
	log.Fatal(app.Listen(":8080"))
}
