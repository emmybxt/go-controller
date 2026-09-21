package main

import (
	"context"
	"log"
	"net/http"

	echoadapter "github.com/emmybxt/go-controller/v2/adapters/echo"
	"github.com/emmybxt/go-controller/v2/example/internal/books"
	"github.com/emmybxt/go-controller/v2/gocontroller"
	"github.com/labstack/echo/v5"
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

func (c *BookController) Get(ctx *echo.Context) error {
	book, found := c.service.Find(ctx.Param("id"))
	if !found {
		return echo.NewHTTPError(http.StatusNotFound, "book not found")
	}
	return ctx.JSON(http.StatusOK, book)
}

func main() {
	router := echo.New()
	err := gocontroller.Mount(echoadapter.New(router), &gocontroller.Module{
		Name: "LibraryModule", Prefix: "/api",
		Providers: []any{books.NewService}, Controllers: []any{NewBookController},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(echo.StartConfig{Address: ":8080"}.Start(context.Background(), router))
}
