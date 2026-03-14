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
	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/notification"
	"github.com/korjavin/dutyassistant/internal/scheduler"
	"github.com/korjavin/dutyassistant/internal/service"
	"github.com/korjavin/dutyassistant/internal/store/sqlite"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
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

	// Initialize legacy dependencies for feature parity during migration
	adminID := parseInt64(getEnv("ADMIN_ID", "0"), 0)
	dishGroupID := parseInt64(getEnv("DISH_GROUP", "0"), 0)

	sched := scheduler.NewScheduler(dbStore)
	var llmClient *llm.Client // Setup standard empty LLM for simplicity unless configured

	telegramHandlers := handlers.New(dbStore, sched, dishGroupID, llmClient)
	telegramHandlers.AdminID = adminID

	// In a real app we'd integrate cron jobs here instead of bot.go or do DI to bot.go
	tgBot := bot.NewBotWithLegacy(botAPI, repo, dutyService, choreService, ratingService, telegramHandlers)
	go tgBot.Start()

	// Setup background jobs
	berlinLoc, _ := time.LoadLocation("Europe/Berlin")
	c := cron.New(cron.WithLocation(berlinLoc))

	c.AddFunc("0 11 * * *", func() {
		log.Println("[CRON] Daily duty assignment")
		duty, err := dutyService.AutoAssignDuty(context.Background(), time.Now())
		if err == nil && duty != nil {
			if duty.User != nil && duty.User.TelegramUserID != 0 {
				msg := fmt.Sprintf("You are on duty today, %s!", duty.User.FirstName)
				dm := tgbotapi.NewMessage(duty.User.TelegramUserID, msg)
				dm.ParseMode = tgbotapi.ModeHTML
				botAPI.Send(dm)
			}
			if dishGroupID != 0 {
				msg := fmt.Sprintf("Duty today is assigned to %s", duty.User.FirstName)
				grp := tgbotapi.NewMessage(dishGroupID, msg)
				grp.ParseMode = tgbotapi.ModeHTML
				botAPI.Send(grp)
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
				msg := tgbotapi.NewMessage(dishGroupID, fmt.Sprintf("🏆 Month-end ratings announced! %s takes 1st place!", winners[0].ParticipantName))
				botAPI.Send(msg)
			}
		}
	})

	c.AddFunc("50 20 * * *", func() {
		log.Println("[CRON] Daily rating reminder")
		if adminID != 0 {
			msg := tgbotapi.NewMessage(adminID, "It's 20:50! Time to rate today's duty performance! (Interactive flow starting soon)")
			botAPI.Send(msg)
		}
	})

	c.AddFunc("0 16 * * *", func() {
		log.Println("[CRON] Daily chore summary")
		if dishGroupID != 0 {
			err := notification.SendDailyChoreSummary(context.Background(), botAPI, dbStore, dishGroupID, true, "Europe/Berlin")
			if err != nil {
				log.Printf("Error sending chore summary: %v", err)
			}
		}
	})

	c.AddFunc("10 21 * * 0", func() {
		log.Println("[CRON] Weekly stats")
		if dishGroupID != 0 {
			err := notification.SendWeeklyChoreStats(context.Background(), botAPI, dbStore, dishGroupID)
			if err != nil {
				log.Printf("Error sending weekly stats: %v", err)
			}
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
