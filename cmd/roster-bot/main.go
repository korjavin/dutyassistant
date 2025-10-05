package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	httpserver "github.com/korjavin/dutyassistant/internal/http"
	"github.com/korjavin/dutyassistant/internal/scheduler"
	"github.com/korjavin/dutyassistant/internal/store/sqlite"
	"github.com/korjavin/dutyassistant/internal/telegram"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
)

func main() {
	log.Println("Roster Bot starting...")

	// Get configuration from environment
	dbPath := getEnv("DATABASE_PATH", "/app/data/roster.db")
	telegramToken := getEnv("TELEGRAM_APITOKEN", "")
	if telegramToken == "" {
		log.Fatal("TELEGRAM_APITOKEN environment variable is required")
	}
	adminIDStr := getEnv("ADMIN_ID", "0")
	adminID := parseInt64(adminIDStr, 0)

	// Initialize database
	log.Println("Initializing database at", dbPath)
	ctx := context.Background()
	store, err := sqlite.New(ctx, dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize scheduler
	log.Println("Initializing scheduler...")
	sched := scheduler.NewScheduler(store)

	// Initialize Telegram handlers
	log.Println("Initializing Telegram handlers...")
	var telegramHandlers *handlers.Handlers
	if adminID != 0 {
		log.Printf("Admin ID configured: %d", adminID)
		telegramHandlers = handlers.NewWithAdminID(store, sched, adminID)
	} else {
		telegramHandlers = handlers.New(store, sched)
	}

	// Initialize and start Telegram bot
	log.Println("Initializing Telegram bot...")
	bot, err := telegram.NewBot(telegramToken, telegramHandlers)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram bot: %v", err)
	}

	// Start bot in background
	botCtx, botCancel := context.WithCancel(ctx)
	defer botCancel()
	go bot.Start(botCtx)

	// Initialize cron scheduler for scheduled jobs (all times in Europe/Berlin)
	log.Println("Initializing cron scheduler...")
	berlinLoc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		log.Fatalf("Failed to load Europe/Berlin timezone: %v", err)
	}
	c := cron.New(cron.WithLocation(berlinLoc))

	// Daily at 11:00 AM Berlin - Assign today's duty
	_, err = c.AddFunc("0 11 * * *", func() {
		log.Println("[CRON] Running daily duty assignment (11:00 AM Berlin)")
		duty, err := sched.AssignTodaysDuty(context.Background())
		if err != nil {
			log.Printf("[CRON] Error assigning today's duty: %v", err)
		} else if duty != nil {
			log.Printf("[CRON] Successfully assigned duty to user %d", duty.UserID)
			// TODO: Send notification to DISH_GROUP
		}
	})
	if err != nil {
		log.Fatalf("Failed to schedule daily assignment job: %v", err)
	}

	// Daily at 21:00 PM Berlin - Mark duty as completed
	_, err = c.AddFunc("0 21 * * *", func() {
		log.Println("[CRON] Running daily duty completion (21:00 PM Berlin)")
		err := sched.CompleteTodaysDuty(context.Background())
		if err != nil {
			log.Printf("[CRON] Error completing today's duty: %v", err)
		} else {
			log.Printf("[CRON] Successfully marked today's duty as completed")
		}
	})
	if err != nil {
		log.Fatalf("Failed to schedule daily completion job: %v", err)
	}

	// Sunday at 21:10 PM Berlin - Send weekly stats
	_, err = c.AddFunc("10 21 * * 0", func() {
		log.Println("[CRON] Running weekly stats (Sunday 21:10 PM Berlin)")
		// TODO: Implement weekly stats gathering and sending to DISH_GROUP
		log.Printf("[CRON] Weekly stats job executed")
	})
	if err != nil {
		log.Fatalf("Failed to schedule weekly stats job: %v", err)
	}

	// Start cron scheduler
	c.Start()
	log.Println("Cron scheduler started with 3 jobs")

	// Initialize HTTP server with Gin
	log.Println("Initializing HTTP server on :8080...")
	router := httpserver.NewServer(store, telegramToken)

	// Create HTTP server for graceful shutdown
	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Start HTTP server in background
	go func() {
		log.Println("HTTP server listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	log.Println("Roster Bot v0.1.0 initialized successfully")
	log.Println("Press Ctrl+C to shut down")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully...")

	// Stop cron scheduler
	log.Println("Stopping cron scheduler...")
	cronCtx := c.Stop()
	<-cronCtx.Done()

	// Graceful shutdown of HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Stop Telegram bot
	botCancel()

	log.Println("Roster Bot stopped")
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
