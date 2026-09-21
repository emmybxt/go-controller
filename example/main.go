package main

import (
	"log"
	"net/http"

	ginadapter "github.com/emmybxt/go-controller/v2/adapters/gin"
	"github.com/emmybxt/go-controller/v2/example/internal/books"
	"github.com/emmybxt/go-controller/v2/gocontroller"
	"github.com/gin-gonic/gin"
)

//go:generate go run ../cmd/gocontroller-gen -dir . -out routes.gen.go

// @Controller("/books")
type BookController struct{ service *books.Service }

func NewBookController(service *books.Service) *BookController {
	return &BookController{service: service}
}

// @Get("/:id")
func (c *BookController) Get(ctx *gin.Context) {
	book, found := c.service.Find(ctx.Param("id"))
	if !found {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "book not found"})
		return
	}
	ctx.JSON(http.StatusOK, book)
}

// @Get("/featured")
// @Use(FeaturedHeader())
func (c *BookController) Featured(ctx *gin.Context) {
	book, _ := c.service.Find("1")
	ctx.JSON(http.StatusOK, book)
}

func FeaturedHeader() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("X-Featured", "true")
		ctx.Next()
	}
}

func newRouter() (*gin.Engine, error) {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/health", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	err := gocontroller.Mount(ginadapter.New(router), &gocontroller.Module{
		Name:        "LibraryModule",
		Prefix:      "/api",
		Providers:   []any{books.NewService},
		Controllers: []any{NewBookController},
	})
	return router, err
}

func main() {
	router, err := newRouter()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(router.Run(":8080"))
}
