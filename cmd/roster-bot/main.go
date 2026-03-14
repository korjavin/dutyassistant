package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/api"
	"github.com/korjavin/dutyassistant/internal/bot"
	"github.com/korjavin/dutyassistant/internal/domain"
	"github.com/korjavin/dutyassistant/internal/service"
	"github.com/korjavin/dutyassistant/internal/store/sqlite"
	"github.com/robfig/cron/v3"
)

func main() {
	telegramToken := getEnv("TELEGRAM_APITOKEN", "")
	if telegramToken == "" {
		log.Fatal("TELEGRAM_APITOKEN environment variable must be set")
	}

	botAPI, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}
	botAPI.Debug = getEnv("GIN_MODE", "debug") == "debug"
	log.Printf("Authorized on account %s", botAPI.Self.UserName)

	dbPath := getEnv("DATABASE_PATH", "/app/data/roster.db")
	dbStore, err := sqlite.New(context.Background(), dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	repo := domain.NewStoreAdapter(dbStore)

	dutyService := service.NewDutyService(repo)
	choreService := service.NewChoreService(repo)
	ratingService := service.NewRatingService(repo)

	tgBot := bot.NewBot(botAPI, dutyService, choreService, ratingService)
	go tgBot.Start()

	// Setup background jobs
	berlinLoc, _ := time.LoadLocation("Europe/Berlin")
	c := cron.New(cron.WithLocation(berlinLoc))
	dishGroupID := parseInt64(getEnv("DISH_GROUP", "0"), 0)
	adminID := parseInt64(getEnv("ADMIN_ID", "0"), 0)

	c.AddFunc("0 11 * * *", func() {
		log.Println("[CRON] Daily duty assignment")
		duty, err := dutyService.AutoAssignDuty(context.Background(), time.Now())
		if err == nil && duty != nil {
			if duty.User != nil && duty.User.TelegramUserID != 0 {
				msg := tgbotapi.NewMessage(duty.User.TelegramUserID, fmt.Sprintf("You are on duty today!"))
				botAPI.Send(msg)
			}
			if dishGroupID != 0 {
				msg := tgbotapi.NewMessage(dishGroupID, fmt.Sprintf("Duty today: %s", duty.User.FirstName))
				botAPI.Send(msg)
			}
		}
	})

	c.AddFunc("0 21 * * *", func() {
		log.Println("[CRON] Duty completion")
		dutyService.CompleteTodaysDuty(context.Background())

		// Month-end winners announcement
		now := time.Now()
		if now.Month() != now.AddDate(0, 0, 1).Month() && dishGroupID != 0 {
			winners, _ := ratingService.GetMonthlyWinners(context.Background(), now.Year(), now.Month())
			if len(winners) > 0 {
				msg := tgbotapi.NewMessage(dishGroupID, "Month-end ratings announced!")
				botAPI.Send(msg)
			}
		}
	})

	c.AddFunc("50 20 * * *", func() {
		log.Println("[CRON] Daily rating reminder")
		if adminID != 0 {
			msg := tgbotapi.NewMessage(adminID, "Time to rate today's duty performance!")
			botAPI.Send(msg)
		}
	})

	c.AddFunc("0 16 * * *", func() {
		log.Println("[CRON] Daily chore summary")
		if dishGroupID != 0 {
			msg := tgbotapi.NewMessage(dishGroupID, "Daily chore summary report")
			botAPI.Send(msg)
		}
	})

	c.AddFunc("10 21 * * 0", func() {
		log.Println("[CRON] Weekly stats")
		if dishGroupID != 0 {
			msg := tgbotapi.NewMessage(dishGroupID, "Weekly stats report")
			botAPI.Send(msg)
		}
	})

	c.Start()

	dutySecret := getEnv("DUTY_SECRET", "")
	router := api.NewServer(repo, dutyService, choreService, ratingService, telegramToken, dutySecret)

	log.Println("HTTP server listening on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt64(s string, defaultValue int64) int64 {
	var result int64
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
		return defaultValue
	}
	return result
}
