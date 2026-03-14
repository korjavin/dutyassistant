package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	httpserver "github.com/korjavin/dutyassistant/internal/http"
	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/notification"
	"github.com/korjavin/dutyassistant/internal/scheduler"
	"github.com/korjavin/dutyassistant/internal/store/sqlite"
	"github.com/korjavin/dutyassistant/internal/telegram"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	slog.Info(fmt.Sprint("Roster Bot starting..."))

	// Get configuration from environment
	dbPath := getEnv("DATABASE_PATH", "/app/data/roster.db")
	telegramToken := getEnv("TELEGRAM_APITOKEN", "")
	if telegramToken == "" {
		slog.Error(fmt.Sprint("TELEGRAM_APITOKEN environment variable is required"))
		os.Exit(1)
	}
	adminIDStr := getEnv("ADMIN_ID", "0")
	adminID := parseInt64(adminIDStr, 0)
	dishGroupIDStr := getEnv("DISH_GROUP", "0")
	dishGroupID := parseInt64(dishGroupIDStr, 0)

	openaiAPIKey := getEnv("OPENAI_API_KEY", "")
	openaiURL := getEnv("OPENAI_URL", "")
	openaiTimeout := parseInt64(getEnv("OPENAI_TIMEOUT_SECONDS", "10"), 10)
	openaiModel := getEnv("OPENAI_MODEL", "gpt-4o-mini")

	var openaiTemperature *float64
	if tempStr := os.Getenv("OPENAI_TEMPERATURE"); tempStr != "" {
		if temp, err := strconv.ParseFloat(tempStr, 64); err == nil {
			openaiTemperature = &temp
		}
	}

	// Initialize database
	slog.Info(fmt.Sprint("Initializing database at", dbPath))
	ctx := context.Background()
	store, err := sqlite.New(ctx, dbPath)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to initialize database: %v", err))
		os.Exit(1)
	}

	// Initialize scheduler
	slog.Info(fmt.Sprint("Initializing scheduler..."))
	sched := scheduler.NewScheduler(store)

	// Initialize LLM client
	slog.Info(fmt.Sprint("Initializing LLM client..."))
	llmClient := llm.NewClient(openaiAPIKey, openaiURL, int(openaiTimeout), openaiModel, openaiTemperature)
	if llmClient != nil {
		model, temp, url := llmClient.Config()
		slog.Info(fmt.Sprintf("LLM Client: Enabled (Provider: OpenAI, Model: %s, Temperature: %.2f, BaseURL: %s)", model, temp, url))
	} else {
		slog.Info(fmt.Sprint("LLM Client: Disabled (OPENAI_API_KEY not set)"))
	}

	// Initialize Telegram handlers
	slog.Info(fmt.Sprint("Initializing Telegram handlers..."))
	var telegramHandlers *handlers.Handlers
	if adminID != 0 {
		slog.Info(fmt.Sprintf("Admin ID configured: %d", adminID))
		telegramHandlers = handlers.NewWithAdminID(store, sched, dishGroupID, adminID, llmClient)
	} else {
		telegramHandlers = handlers.New(store, sched, dishGroupID, llmClient)
	}

	// Initialize and start Telegram bot
	slog.Info(fmt.Sprint("Initializing Telegram bot..."))
	bot, err := telegram.NewBot(telegramToken, telegramHandlers, dishGroupID, adminID)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to initialize Telegram bot: %v", err))
		os.Exit(1)
	}
	slog.Info(fmt.Sprintf("Access control configured: GroupID=%d, OwnerID=%d", dishGroupID, adminID))

	// Start bot in background
	botCtx, botCancel := context.WithCancel(ctx)
	defer botCancel()
	go bot.Start(botCtx)

	// Launch periodic chore reminders goroutine
	go notification.StartPeriodicChoreReminders(botCtx, bot, store, getEnv("CHORE_TIMEZONE", "Europe/Berlin"))

	// Initialize cron scheduler for scheduled jobs (all times in Europe/Berlin)
	slog.Info(fmt.Sprint("Initializing cron scheduler..."))
	berlinLoc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to load Europe/Berlin timezone: %v", err))
		os.Exit(1)
	}
	c := cron.New(cron.WithLocation(berlinLoc))

	// Daily at 11:00 AM Berlin - Assign today's duty and Process Recurring Chores
	_, err = c.AddFunc("0 11 * * *", func() {
		slog.Info(fmt.Sprint("═══════════════════════════════════════════════════════════"))
		slog.Info(fmt.Sprint("Running daily duty assignment and recurring chores (11:00 AM Berlin)"), slog.String("component", "cron"))
		slog.Info(fmt.Sprintf("Current time: %s", time.Now().In(berlinLoc).Format("2006-01-02 15:04:05 MST")), slog.String("component", "cron"))

		// Process Recurring Chores
		if err := telegramHandlers.ProcessRecurringChores(context.Background()); err != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to process recurring chores: %v", err), slog.String("component", "cron"))
		}

		duty, err := sched.AssignTodaysDuty(context.Background())
		if err != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to assign today's duty: %v", err), slog.String("component", "cron"))
			return
		}

		if duty == nil {
			slog.Warn(fmt.Sprintf("WARNING: No duty was assigned (duty is nil)"), slog.String("component", "cron"))
			return
		}

		slog.Info(fmt.Sprintf("✓ Successfully assigned duty to user %d (Assignment Type: %s)", duty.UserID, duty.AssignmentType), slog.String("component", "cron"))

		if duty.User == nil {
			slog.Error(fmt.Sprintf("ERROR: Duty.User is nil - cannot send notifications!"), slog.String("component", "cron"))
			return
		}

		slog.Info(fmt.Sprintf("Duty details: UserID=%d, Name=%s, TelegramID=%d, Date=%s, Type=%s", duty.UserID, duty.User.FirstName, duty.User.TelegramUserID, duty.DutyDate.Format("2006-01-02"), duty.AssignmentType), slog.String("component", "cron"))

		// Send DM to assigned user using our notification formatter
		if duty.User.TelegramUserID != 0 {
			slog.Info(fmt.Sprintf("Preparing DM for user %s (TelegramID: %d)", duty.User.FirstName, duty.User.TelegramUserID), slog.String("component", "cron"))
			dmMsg := notification.FormatDMToAssignee(duty)

			if llmClient != nil {
				dmMsg = llmClient.RefineMessage(context.Background(), "friendly congratulatory DM to person assigned chore", dmMsg)
			}

			slog.Info(fmt.Sprintf("DM message content: %s", dmMsg), slog.String("component", "cron"))

			if err := bot.SendMessageHTML(duty.User.TelegramUserID, dmMsg); err != nil {
				slog.Error(fmt.Sprintf("ERROR: Failed to send DM to user %d: %v", duty.User.TelegramUserID, err), slog.String("component", "cron"))
				// Log failure to database
				if dbErr := store.LogNotification(context.Background(), duty.DutyDate, duty.UserID, "DM", "FAILED", err.Error()); dbErr != nil {
					slog.Error(fmt.Sprintf("ERROR: Failed to log DM failure: %v", dbErr), slog.String("component", "cron"))
				}
			} else {
				slog.Info(fmt.Sprintf("✓ Successfully sent DM notification to user %d", duty.User.TelegramUserID), slog.String("component", "cron"))
				// Log success to database
				if dbErr := store.LogNotification(context.Background(), duty.DutyDate, duty.UserID, "DM", "SUCCESS", ""); dbErr != nil {
					slog.Error(fmt.Sprintf("ERROR: Failed to log DM success: %v", dbErr), slog.String("component", "cron"))
				}
			}
		} else {
			slog.Warn(fmt.Sprintf("WARNING: User %s has TelegramUserID=0, cannot send DM", duty.User.FirstName), slog.String("component", "cron"))
			// Log skip to database
			if dbErr := store.LogNotification(context.Background(), duty.DutyDate, duty.UserID, "DM", "SKIPPED", "TelegramUserID is 0"); dbErr != nil {
				slog.Error(fmt.Sprintf("ERROR: Failed to log DM skip: %v", dbErr), slog.String("component", "cron"))
			}
		}

		// Send notification to group chat using our notification formatter
		if dishGroupID != 0 {
			slog.Info(fmt.Sprintf("Preparing group message for chat %d", dishGroupID), slog.String("component", "cron"))
			groupMsg := notification.FormatDutyAssignedMessage(duty)

			if llmClient != nil {
				groupMsg = llmClient.RefineMessage(context.Background(), "congratulate duty assignee proudly", groupMsg)
			}

			slog.Info(fmt.Sprintf("Group message content: %s", groupMsg), slog.String("component", "cron"))

			if err := bot.SendMessageHTML(dishGroupID, groupMsg); err != nil {
				slog.Error(fmt.Sprintf("ERROR: Failed to send group notification to chat %d: %v", dishGroupID, err), slog.String("component", "cron"))
				// Log failure to database
				if dbErr := store.LogNotification(context.Background(), duty.DutyDate, duty.UserID, "GROUP", "FAILED", err.Error()); dbErr != nil {
					slog.Error(fmt.Sprintf("ERROR: Failed to log group notification failure: %v", dbErr), slog.String("component", "cron"))
				}
			} else {
				slog.Info(fmt.Sprintf("✓ Successfully sent group notification to chat %d", dishGroupID), slog.String("component", "cron"))
				// Log success to database
				if dbErr := store.LogNotification(context.Background(), duty.DutyDate, duty.UserID, "GROUP", "SUCCESS", ""); dbErr != nil {
					slog.Error(fmt.Sprintf("ERROR: Failed to log group notification success: %v", dbErr), slog.String("component", "cron"))
				}
			}
		} else {
			slog.Warn(fmt.Sprintf("WARNING: DISH_GROUP not configured (dishGroupID=0), skipping group notification"), slog.String("component", "cron"))
			// Log skip to database
			if dbErr := store.LogNotification(context.Background(), duty.DutyDate, duty.UserID, "GROUP", "SKIPPED", "dishGroupID not configured"); dbErr != nil {
				slog.Error(fmt.Sprintf("ERROR: Failed to log group notification skip: %v", dbErr), slog.String("component", "cron"))
			}
		}

		slog.Info(fmt.Sprint("Daily duty assignment completed"), slog.String("component", "cron"))
		slog.Info(fmt.Sprint("═══════════════════════════════════════════════════════════"))
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule daily assignment job: %v", err))
		os.Exit(1)
	}

	// Daily at 21:00 PM Berlin - Mark duty as completed
	_, err = c.AddFunc("0 21 * * *", func() {
		slog.Info(fmt.Sprint("Running daily duty completion (21:00 PM Berlin)"), slog.String("component", "cron"))
		err := sched.CompleteTodaysDuty(context.Background())
		if err != nil {
			slog.Error(fmt.Sprintf("Error completing today's duty: %v", err), slog.String("component", "cron"))
		} else {
			slog.Info(fmt.Sprintf("Successfully marked today's duty as completed"), slog.String("component", "cron"))
		}
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule daily completion job: %v", err))
		os.Exit(1)
	}

	// Daily at 20:50 Berlin - Ask the admin to rate active participants
	_, err = c.AddFunc("50 20 * * *", func() {
		slog.Info(fmt.Sprint("Running daily participant rating reminder (20:50 Berlin)"), slog.String("component", "cron"))

		if adminID == 0 {
			slog.Info(fmt.Sprint("Participant rating reminder skipped: ADMIN_ID is not configured"), slog.String("component", "cron"))
			return
		}

		msg, ok, err := telegramHandlers.PrepareDailyRatingsReminder(adminID, adminID, time.Now().In(berlinLoc))
		if err != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to prepare participant rating reminder: %v", err), slog.String("component", "cron"))
			return
		}
		if !ok {
			slog.Info(fmt.Sprint("Participant rating reminder skipped: no active non-admin participants to rate"), slog.String("component", "cron"))
			return
		}

		if _, err := bot.API().Send(*msg); err != nil {
			telegramHandlers.SessionManager.EndSession(adminID)
			slog.Error(fmt.Sprintf("ERROR: Failed to send participant rating reminder to admin %d: %v", adminID, err), slog.String("component", "cron"))
			return
		}

		slog.Info(fmt.Sprintf("Successfully sent participant rating reminder to admin %d", adminID), slog.String("component", "cron"))
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule participant rating reminder job: %v", err))
		os.Exit(1)
	}

	// Daily at 21:00 Berlin - On the last calendar day, publish the monthly participant rating winners
	_, err = c.AddFunc("0 21 * * *", func() {
		slog.Info(fmt.Sprint("Checking month-end participant ratings announcement (21:00 Berlin)"), slog.String("component", "cron"))

		if dishGroupID == 0 {
			slog.Info(fmt.Sprint("Month-end participant ratings announcement skipped: DISH_GROUP is not configured"), slog.String("component", "cron"))
			return
		}

		msg, ok, err := telegramHandlers.BuildMonthlyRatingsWinnersAnnouncement(time.Now().In(berlinLoc))
		if err != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to build month-end participant ratings announcement: %v", err), slog.String("component", "cron"))
			return
		}
		if !ok {
			slog.Info(fmt.Sprint("Month-end participant ratings announcement skipped: today is not the last calendar day of the month"), slog.String("component", "cron"))
			return
		}

		if _, err := bot.API().Send(*msg); err != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to send month-end participant ratings announcement to group %d: %v", dishGroupID, err), slog.String("component", "cron"))
			return
		}

		slog.Info(fmt.Sprintf("Successfully sent month-end participant ratings announcement to group %d", dishGroupID), slog.String("component", "cron"))
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule month-end participant ratings announcement job: %v", err))
		os.Exit(1)
	}

	// Daily at 16:00 Berlin - Send daily chore summary
	tz := getEnv("CHORE_TIMEZONE", "Europe/Berlin")
	_, err = c.AddFunc("CRON_TZ="+tz+" 0 16 * * *", func() {
		slog.Info(fmt.Sprintf("Running daily chore summary (16:00 %s)", tz), slog.String("component", "cron"))
		err := notification.SendDailyChoreSummary(context.Background(), bot.API(), store, dishGroupID, true, getEnv("CHORE_TIMEZONE", "Europe/Berlin"))
		if err != nil {
			slog.Error(fmt.Sprintf("Error sending daily chore summary: %v", err), slog.String("component", "cron"))
		} else {
			slog.Info(fmt.Sprintf("Successfully sent daily chore summary"), slog.String("component", "cron"))
		}
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule daily chore summary job: %v", err))
		os.Exit(1)
	}

	// Sunday at 21:10 PM Berlin - Send weekly stats
	_, err = c.AddFunc("10 21 * * 0", func() {
		slog.Info(fmt.Sprint("Running weekly stats (Sunday 21:10 PM Berlin)"), slog.String("component", "cron"))
		err := notification.SendWeeklyChoreStats(context.Background(), bot.API(), store, dishGroupID)
		if err != nil {
			slog.Error(fmt.Sprintf("Error sending weekly chore stats: %v", err), slog.String("component", "cron"))
		} else {
			slog.Info(fmt.Sprintf("Successfully sent weekly chore stats"), slog.String("component", "cron"))
		}
		slog.Info(fmt.Sprintf("Weekly stats job executed"), slog.String("component", "cron"))
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule weekly stats job: %v", err))
		os.Exit(1)
	}

	// Start cron scheduler
	c.Start()
	slog.Info(fmt.Sprint("═══════════════════════════════════════════════════════════"))
	slog.Info(fmt.Sprint("Cron scheduler started with 6 jobs:"))
	slog.Info(fmt.Sprint("  1. Daily at 11:00 AM Berlin - Assign today's duty and send notifications"))
	slog.Info(fmt.Sprint("  2. Daily at 21:00 PM Berlin - Mark today's duty as completed"))
	slog.Info(fmt.Sprint("  3. Daily at 20:50 PM Berlin - Send participant rating reminder to the admin"))
	slog.Info(fmt.Sprint("  4. Daily at 21:00 PM Berlin - Publish month-end participant rating winners on the last calendar day"))
	slog.Info(fmt.Sprint("  5. Daily at 16:00 Europe/Berlin - Send daily chore summary"))
	slog.Info(fmt.Sprint("  6. Sunday at 21:10 PM Berlin - Send weekly stats"))
	slog.Info(fmt.Sprintf("Current Berlin time: %s", time.Now().In(berlinLoc).Format("2006-01-02 15:04:05 MST")))
	slog.Info(fmt.Sprint("═══════════════════════════════════════════════════════════"))

	// Initialize HTTP server with Gin
	slog.Info(fmt.Sprint("Initializing HTTP server on :8080..."))
	dutySecret := getEnv("DUTY_SECRET", "")
	if dutySecret == "" {
		slog.Warn(fmt.Sprint("WARNING: DUTY_SECRET is not set. The /who endpoint will return 503 until it is configured."))
	}
	router := httpserver.NewServer(store, telegramToken, dutySecret)

	// Create HTTP server for graceful shutdown
	srv := &http.Server{
		Addr:		":8080",
		Handler:	router,
	}

	// Start HTTP server in background
	go func() {
		slog.Info(fmt.Sprint("HTTP server listening on :8080"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error(fmt.Sprintf("HTTP server error: %v", err))
			os.Exit(1)
		}
	}()

	slog.Info(fmt.Sprint("Roster Bot v0.1.0 initialized successfully"))
	slog.Info(fmt.Sprint("Press Ctrl+C to shut down"))

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info(fmt.Sprint("Shutting down gracefully..."))

	// Stop cron scheduler
	slog.Info(fmt.Sprint("Stopping cron scheduler..."))
	cronCtx := c.Stop()
	<-cronCtx.Done()

	// Graceful shutdown of HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error(fmt.Sprintf("HTTP server shutdown error: %v", err))
	}

	// Stop Telegram bot
	botCancel()

	slog.Info(fmt.Sprint("Roster Bot stopped"))
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
