package api

import (
	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/domain"
)

func NewServer(repo domain.Repository, dutyService domain.DutyService, choreService domain.ChoreService, ratingService domain.RatingService, telegramToken string, dutySecret string) *gin.Engine {
	r := gin.Default()

	// Setting up API groups
	api := r.Group("/api/v1")
	{
		api.GET("/chores/active", GetActiveChores(choreService))
		api.POST("/duties/volunteer", VolunteerForDuty(dutyService))
		api.POST("/duties", AdminAssignDuty(dutyService))
		api.PUT("/duties/:date", AdminModifyDuty(dutyService))
		api.DELETE("/duties/:date", AdminDeleteDuty(dutyService))
		api.GET("/schedule/:year/:month", GetSchedule(dutyService))
		api.GET("/users", GetUsers(repo))
		api.GET("/who", GetWho(repo, dutySecret))
	}

	return r
}
