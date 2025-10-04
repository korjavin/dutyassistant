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
