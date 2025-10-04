package http

import (
	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/http/handlers"
	"github.com/korjavin/dutyassistant/internal/http/middleware"
	"github.com/korjavin/dutyassistant/internal/store"
)

// NewServer creates and configures a new Gin HTTP server.
// It sets up the router, registers middleware, and defines all API routes.
func NewServer(s store.Store, botToken string) *gin.Engine {
	// Set Gin to release mode for production.
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Use structured logging and recovery middleware.
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Serve static files from web directory
	router.Static("/dist", "./web/dist")
	router.Static("/js", "./web/js")
	router.StaticFile("/", "./web/index.html")
	router.StaticFile("/index.html", "./web/index.html")

	// Create an instance of the authentication middleware.
	authMiddleware := middleware.Authenticate(s, botToken)
	adminRequiredMiddleware := middleware.AdminRequired()

	// Group all API routes under /api/v1.
	api := router.Group("/api/v1")
	{
		// Public endpoints, accessible to anyone.
		api.GET("/schedule/:year/:month", handlers.GetSchedule(s))
		api.GET("/users", handlers.GetUsers(s))

		// Endpoints requiring user authentication (via Telegram Web App).
		authenticated := api.Group("/")
		authenticated.Use(authMiddleware)
		{
			authenticated.POST("/duties/volunteer", handlers.VolunteerForDuty(s))
		}

		// Endpoints requiring administrator privileges.
		admin := api.Group("/")
		admin.Use(authMiddleware, adminRequiredMiddleware)
		{
			admin.POST("/duties", handlers.AdminAssignDuty(s))
			admin.PUT("/duties/:date", handlers.AdminModifyDuty(s))
			admin.DELETE("/duties/:date", handlers.AdminDeleteDuty(s))
		}
	}

	return router
}