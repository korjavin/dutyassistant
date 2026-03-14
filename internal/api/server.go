package api

import (
	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/api/middleware"
	"github.com/korjavin/dutyassistant/internal/domain"
)

func NewServer(repo domain.Repository, dutyService domain.DutyService, choreService domain.ChoreService, ratingService domain.RatingService, telegramToken string, dutySecret string) *gin.Engine {
	r := gin.Default()

	r.GET("/who", middleware.HMACAuth(dutySecret), GetWho(repo))

	api := r.Group("/api/v1")
	{
		api.GET("/chores/active", middleware.OptionalAuth(repo, telegramToken), GetActiveChores(repo))
		api.POST("/duties/volunteer", middleware.Authenticate(repo, telegramToken), VolunteerForDuty(dutyService))
		api.GET("/schedule/:year/:month", middleware.OptionalAuth(repo, telegramToken), GetSchedule(dutyService))
		api.GET("/users", GetUsers(repo))

		adminGroup := api.Group("")
		adminGroup.Use(middleware.Authenticate(repo, telegramToken), middleware.AdminRequired())
		{
			adminGroup.POST("/duties", AdminAssignDuty(dutyService))
			adminGroup.PUT("/duties/:date", AdminModifyDuty(dutyService))
			adminGroup.DELETE("/duties/:date", AdminDeleteDuty(repo))
		}
	}

	return r
}
