package http

import (
	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/http/handlers"
	"github.com/korjavin/dutyassistant/internal/http/middleware"
	"github.com/korjavin/dutyassistant/internal/store"
)

// NewServer creates and configures a new Gin HTTP server.
// It sets up the router, registers middleware, and defines all API routes.
//
// dutySecret is the shared HMAC secret for the /who endpoint used by EchoBridge.
// If empty, the /who endpoint will respond with 503 Service Unavailable.
func NewServer(s store.Store, botToken, dutySecret string) *gin.Engine {
	// Set Gin to release mode for production.
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Use structured logging and recovery middleware.
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Add security headers
	router.Use(middleware.SecurityHeaders())

	// Note on CORS: The frontend is served directly from the same domain
	// as the API under the `/api/v1` path, making it same-origin.
	// Therefore, cross-origin access is not required, and we rely
	// on the browser's default same-origin policy to secure the API.

	// Cache Control Middleware
	cacheControlMiddleware := func(c *gin.Context) {
		// Only cache CSS for a long time since we use cache-busting in index.html for it.
		// ES6 modules without fingerprints should not be cached.
		path := c.Request.URL.Path
		if len(path) > 4 && path[:4] == "/css" {
			c.Header("Cache-Control", "public, max-age=86400")
		} else {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		c.Next()
	}

	// Serve static files from web directory
	staticRoutes := router.Group("/")
	staticRoutes.Use(cacheControlMiddleware)
	{
		staticRoutes.Static("/css", "./web/css")
		staticRoutes.Static("/js", "./web/js")
		staticRoutes.Static("/vendor", "./web/vendor")
	}

	// Serve index.html without caching
	indexHandler := func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.File("./web/index.html")
	}
	router.GET("/", indexHandler)
	router.GET("/index.html", indexHandler)

	// Create an instance of the authentication middleware.
	authMiddleware := middleware.Authenticate(s, botToken)
	optionalAuthMiddleware := middleware.OptionalAuth(s, botToken)
	adminRequiredMiddleware := middleware.AdminRequired()

	// Group all API routes under /api/v1.
	api := router.Group("/api/v1")

	// Create rate limiters
	apiLimiter := middleware.NewAPILimiter()
	authLimiter := middleware.NewAuthLimiter()

	api.Use(middleware.RateLimitMiddleware(apiLimiter))
	{
		// Public endpoints with optional auth (return limited data if not authenticated).
		api.GET("/schedule/:year/:month", optionalAuthMiddleware, handlers.GetSchedule(s))
		api.GET("/prognosis/:year/:month", handlers.GetPrognosis(s))
		api.GET("/users", optionalAuthMiddleware, handlers.GetUsers(s))
		api.GET("/chores/active", optionalAuthMiddleware, handlers.GetActiveChores(s))

		// Endpoints requiring user authentication (via Telegram Web App).
		authenticated := api.Group("/")
		authenticated.Use(middleware.RateLimitMiddleware(authLimiter), authMiddleware)
		{
			authenticated.POST("/duties/volunteer", handlers.VolunteerForDuty(s))
		}

		// Endpoints requiring administrator privileges.
		admin := api.Group("/")
		admin.Use(middleware.RateLimitMiddleware(authLimiter), authMiddleware, adminRequiredMiddleware)
		{
			admin.POST("/duties", handlers.AdminAssignDuty(s))
			admin.PUT("/duties/:date", handlers.AdminModifyDuty(s))
			admin.DELETE("/duties/:date", handlers.AdminDeleteDuty(s))
		}
	}

	// Machine-to-machine endpoint: GET /who
	// Uses HMAC-SHA256 auth instead of Telegram Web App auth.
	hmacMiddleware := middleware.HMACAuth(dutySecret)
	router.GET("/who", hmacMiddleware, handlers.GetWho(s))

	return router
}
