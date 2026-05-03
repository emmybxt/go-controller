package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/emmybxt/go-controller/gocontroller"
)

type SomethingService struct {
	ready bool
}

func NewSomethingService() *SomethingService {
	return &SomethingService{}
}

func (s *SomethingService) OnModuleInit(ctx context.Context) error {
	log.Println("[SomethingService] initializing...")
	s.ready = true
	return nil
}

func (s *SomethingService) OnModuleDestroy(ctx context.Context) error {
	log.Println("[SomethingService] shutting down...")
	s.ready = false
	return nil
}

func (s *SomethingService) BuildMessage(id string) string {
	return fmt.Sprintf("resource %s loaded", id)
}

type CreateSomethingDTO struct {
	Name  string `json:"name" validate:"required,min=2,max=32"`
	Email string `json:"email" validate:"required,email"`
}

// @Controller("")
// @Summary("Something API")
type SomethingController struct {
	service *SomethingService
}

func NewSomethingController(service *SomethingService) *SomethingController {
	return &SomethingController{service: service}
}

// @Get("/hello")
// @Summary("Returns a hello message")
func (c *SomethingController) Hello(ctx *gocontroller.Context) error {
	return ctx.JSON(http.StatusOK, map[string]any{
		"message": "Hello, World!",
	})
}

// @Get("/something/:id")
// @Summary("Get a resource by ID")
func (c *SomethingController) GetSomething(ctx *gocontroller.Context) error {
	id := ctx.Param("id")
	return ctx.JSON(http.StatusOK, map[string]any{
		"id":      id,
		"message": c.service.BuildMessage(id),
	})
}

// @Post("/something")
// @Summary("Create a new resource")
// @Use(ExampleAPIKeyMiddleware())
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

// @Post("/upload")
// @Summary("Upload a file")
// @Use(ExampleAPIKeyMiddleware())
func (c *SomethingController) Upload(ctx *gocontroller.Context) error {
	file, err := ctx.BindFile("file")
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"filename": file.Name,
		"size":     file.Header.Size,
	})
}

// @Get("/items")
// @Summary("List items with pagination")
func (c *SomethingController) ListItems(ctx *gocontroller.Context) error {
	pag := gocontroller.ParsePaginationFromContext(ctx)
	items := []map[string]any{
		{"id": 1, "name": "item 1"},
		{"id": 2, "name": "item 2"},
	}
	pag.SetTotal(42)

	ctx.ResponseWriter.Header().Set("Link", pag.LinkHeader("/api/items"))
	return ctx.JSON(http.StatusOK, pag.JSONResponse(items))
}

// @Get("/protected")
// @Summary("JWT protected endpoint")
func (c *SomethingController) Protected(ctx *gocontroller.Context) error {
	claims, ok := gocontroller.JWTClaimsFromContext(ctx)
	if !ok {
		return gocontroller.UnauthorizedError("missing claims")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"subject": claims.Subject(),
		"claims":  claims,
	})
}

// @Get("/session")
// @Summary("Session demo")
func (c *SomethingController) SessionDemo(ctx *gocontroller.Context) error {
	session := gocontroller.GetSession(ctx)
	count, _ := session.Get("visit_count")
	n := 1
	if v, ok := count.(float64); ok {
		n = int(v) + 1
	}
	session.Set("visit_count", n)

	return ctx.JSON(http.StatusOK, map[string]any{
		"visits": n,
	})
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

func ExampleAPIKeyMiddleware() gocontroller.Middleware {
	return func(next gocontroller.HandlerFunc) gocontroller.HandlerFunc {
		return func(ctx *gocontroller.Context) error {
			apiKey := os.Getenv("GOCONTROLLER_EXAMPLE_API_KEY")
			if apiKey == "" {
				return gocontroller.NewHTTPError(http.StatusInternalServerError, "example api key is not configured")
			}
			return APIKeyMiddleware(apiKey)(next)(ctx)
		}
	}
}

//go:generate go run ../cmd/gocontroller-gen -dir . -out zz_gocontroller_routes.gen.go
//go:generate go run ../cmd/oapi-gen -dir . -out openapi.json -title "Something API" -version "1.0.0" -host "localhost:8080" -scheme "http"

func main() {
	root := &gocontroller.Module{
		Name:      "SomethingModule",
		Prefix:    "/api",
		Providers: []any{NewSomethingService},
		Controllers: []any{
			NewSomethingController,
		},
	}

	app, err := gocontroller.NewApp(root)
	if err != nil {
		log.Fatal(err)
	}

	app.Router.Use(
		gocontroller.RequestID(),
		gocontroller.Recovery(gocontroller.RecoveryConfig{}),
		gocontroller.SecurityHeaders(),
		gocontroller.RequestLogger(),
	)

	rl := gocontroller.NewRateLimiter(gocontroller.RateLimitConfig{
		Rate:  100,
		Burst: 50,
	})
	app.Router.Use(rl.RateLimit())

	app.Router.Use(gocontroller.SessionMiddleware(gocontroller.SessionConfig{
		Secure:   false,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}))

	app.Router.Use(gocontroller.Idempotency(gocontroller.IdempotencyConfig{
		Store: gocontroller.NewIdempotencyStore(0, 0),
	}))

	app.Health.RegisterReadiness("service", func() gocontroller.HealthStatus {
		return gocontroller.HealthStatus{Status: "ok"}
	})

	app.Router.Use(gocontroller.Tracing(gocontroller.TelemetryConfig{
		ServiceName: "something-api",
	}))

	eventBus := gocontroller.NewEventBus()
	eventBus.On("request.start", func(ctx context.Context, event gocontroller.Event) error {
		log.Printf("[event] %s started", event.Name)
		return nil
	})
	app.Router.Use(gocontroller.EventMiddleware(eventBus))

	scheduler := gocontroller.NewScheduler()
	_ = scheduler.AddTask("heartbeat", "*/30 * * * *", func(ctx context.Context) error {
		log.Println("[cron] heartbeat tick")
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go scheduler.Start(ctx)

	app.Router.Use(gocontroller.SchedulerMiddleware(scheduler))

	log.Println("server started on :8080")
	if err := app.Run(ctx, gocontroller.ServerOptions{Addr: ":8083"}); err != nil {
		log.Fatal(err)
	}
}
