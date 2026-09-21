package main

import (
	ginadapter "github.com/emmybxt/go-controller/v2/adapters/gin"
	gc "github.com/emmybxt/go-controller/v2/gocontroller"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

//go:generate go run ../../cmd/gocontroller-gen -dir . -out routes.gen.go

var accountsController = gc.Controllers("/accounts").UseBefore(requireUser).UseAfter(auditRequest)

type AccountService struct{}

func NewAccountService() *AccountService           { return &AccountService{} }
func (*AccountService) Find(id, user string) gin.H { return gin.H{"id": id, "owner": user} }

type AccountsController struct{ service *AccountService }

func NewAccountsController(service *AccountService) *AccountsController {
	return &AccountsController{service: service}
}

var _ = accountsController.GET("/:accountId").UseBefore(accountHeader)

func (a *AccountsController) GetAccount(c *gin.Context) {
	c.JSON(http.StatusOK, a.service.Find(c.Param("accountId"), CurrentUser(c)))
}

var _ = accountsController.POST("")

func (a *AccountsController) CreateAccount(c *gin.Context) {
	var input struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, a.service.Find(input.ID, CurrentUser(c)))
}

// CurrentUser is an application-owned resolver over the native request context.
func CurrentUser(c *gin.Context) string { return c.GetString("current-user") }

func requireUser(c *gin.Context) {
	// Demo identity only: production middleware must verify credentials.
	user := c.GetHeader("X-Demo-User")
	if user == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	c.Set("current-user", user)
	c.Next()
}

func accountHeader(c *gin.Context) { c.Header("X-Accounts", "true"); c.Next() }
func auditRequest(c *gin.Context) {
	log.Printf("%s %s user=%s status=%d", c.Request.Method, c.FullPath(), CurrentUser(c), c.Writer.Status())
	c.Next()
}

func newRouter() (*gin.Engine, error) {
	router := gin.New()
	router.Use(gin.Recovery())
	err := gc.Mount(ginadapter.New(router), &gc.Module{
		Name: "AccountsModule", Prefix: "/api", Providers: []any{NewAccountService}, Controllers: []any{NewAccountsController},
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
