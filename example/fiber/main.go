package main

import (
	"log"

	fiberadapter "github.com/emmybxt/go-controller/v2/adapters/fiber"
	"github.com/emmybxt/go-controller/v2/example/internal/books"
	"github.com/emmybxt/go-controller/v2/gocontroller"
	"github.com/gofiber/fiber/v3"
)

type BookController struct{ service *books.Service }

func NewBookController(service *books.Service) *BookController {
	return &BookController{service: service}
}

func (c *BookController) ControllerMetadata() gocontroller.ControllerMetadata {
	return gocontroller.ControllerMetadata{
		Prefix: "/books",
		Routes: []gocontroller.Route{gocontroller.GET("/:id", c.Get)},
	}
}

func (c *BookController) Get(ctx fiber.Ctx) error {
	book, found := c.service.Find(ctx.Params("id"))
	if !found {
		return fiber.NewError(fiber.StatusNotFound, "book not found")
	}
	return ctx.JSON(book)
}

func main() {
	router := fiber.New()
	err := gocontroller.Mount(fiberadapter.New(router), &gocontroller.Module{
		Name: "LibraryModule", Prefix: "/api",
		Providers: []any{books.NewService}, Controllers: []any{NewBookController},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(router.Listen(":8080"))
}
