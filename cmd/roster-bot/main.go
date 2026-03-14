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

	// In a real app we'd integrate cron jobs here instead of bot.go or do DI to bot.go
	tgBot := bot.NewBot(botAPI, dutyService, choreService, ratingService)
	go tgBot.Start()

	// Setup background jobs
	berlinLoc, _ := time.LoadLocation("Europe/Berlin")
	c := cron.New(cron.WithLocation(berlinLoc))

	c.AddFunc("0 11 * * *", func() {
		log.Println("Cron: Triggering auto assign duty...")
		dutyService.AutoAssignDuty(context.Background(), time.Now())
	})

	c.AddFunc("0 21 * * *", func() {
		log.Println("Cron: Triggering duty completion...")
		dutyService.CompleteTodaysDuty(context.Background())
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
