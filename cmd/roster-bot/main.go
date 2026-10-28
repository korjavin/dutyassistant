package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
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
	slog.Info("Roster Bot starting...")

	// Get configuration from environment
	dbPath := getEnv("DATABASE_PATH", "/app/data/roster.db")
	telegramToken := getEnv("TELEGRAM_APITOKEN", "")
	if telegramToken == "" {
		slog.Error("TELEGRAM_APITOKEN environment variable is required")
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

	anthropicAPIKey := getEnv("ANTHROPIC_API_KEY", "")
	anthropicURL := getEnv("ANTHROPIC_BASE_URL", "")
	anthropicTimeout := parseInt64(getEnv("ANTHROPIC_TIMEOUT_SECONDS", "30"), 30)
	anthropicModel := getEnv("ANTHROPIC_MODEL", "claude-opus-5-5")
	anthropicEffort := getEnv("ANTHROPIC_EFFORT", "low")
	assistantEffort := getEnv("ANTHROPIC_ASSISTANT_EFFORT", "medium")
	participantDailyLimit := int(parseInt64(getEnv("PARTICIPANT_ASSISTANT_DAILY_LIMIT", "20"), 20))

	llmProvider := resolveLLMProvider(os.Getenv("LLM_PROVIDER"), anthropicAPIKey)

	// Speech-to-text for voice reports (the Claude API does not take audio).
	// Without its own key it reuses the OpenAI key and URL.
	transcriptionAPIKey := getEnv("TRANSCRIPTION_API_KEY", "")
	transcriptionURL := getEnv("TRANSCRIPTION_URL", "")
	if transcriptionAPIKey == "" {
		transcriptionAPIKey = openaiAPIKey
		if transcriptionURL == "" {
			transcriptionURL = openaiURL
		}
	}
	transcriptionTimeout := parseInt64(getEnv("TRANSCRIPTION_TIMEOUT_SECONDS", "60"), 60)
	transcriptionModel := getEnv("TRANSCRIPTION_MODEL", "whisper-1")
	transcriptionLanguage := getEnv("TRANSCRIPTION_LANGUAGE", "")

	// Initialize database
	slog.Info(fmt.Sprint("Initializing database at", dbPath))
	ctx := context.Background()
	store, err := sqlite.New(ctx, dbPath)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to initialize database: %v", err))
		os.Exit(1)
	}

	// Initialize scheduler
	slog.Info("Initializing scheduler...")
	sched := scheduler.NewScheduler(store)

	// Initialize LLM client
	slog.Info("Initializing LLM client...")
	var llmClient *llm.Client
	switch llmProvider {
	case llm.ProviderAnthropic:
		llmClient = llm.NewAnthropicClient(anthropicAPIKey, anthropicURL, int(anthropicTimeout), anthropicModel, anthropicEffort)
		if llmClient != nil {
			model, _, url := llmClient.Config()
			slog.Info(fmt.Sprintf("LLM Client: Enabled (Provider: Anthropic, Model: %s, Effort: %s, BaseURL: %s)", model, llmClient.Effort(), url))
		} else {
			slog.Info("LLM Client: Disabled (ANTHROPIC_API_KEY not set)")
		}
	default:
		llmClient = llm.NewClient(openaiAPIKey, openaiURL, int(openaiTimeout), openaiModel, openaiTemperature)
		if llmClient != nil {
			model, temp, url := llmClient.Config()
			slog.Info(fmt.Sprintf("LLM Client: Enabled (Provider: OpenAI, Model: %s, Temperature: %.2f, BaseURL: %s)", model, temp, url))
		} else {
			slog.Info("LLM Client: Disabled (OPENAI_API_KEY not set)")
		}
	}

	// Initialize Telegram handlers
	slog.Info("Initializing Telegram handlers...")
	var telegramHandlers *handlers.Handlers
	if adminID != 0 {
		slog.Info(fmt.Sprintf("Admin ID configured: %d", adminID))
		telegramHandlers = handlers.NewWithAdminID(store, sched, dishGroupID, adminID, llmClient)
	} else {
		telegramHandlers = handlers.New(store, sched, dishGroupID, llmClient)
	}

	// Daily reports: the LLM proposes scores, the transcriber handles voice
	// and Claude reads images.
	// Avoid passing typed-nil pointers into the interfaces.
	if llmClient != nil {
		telegramHandlers.ReportAnalyzer = llmClient
		if llmClient.Provider() == llm.ProviderAnthropic {
			// Photos are described by Claude and then handled like text.
			telegramHandlers.ImageReader = llmClient
		}
	}
	if transcriber := llm.NewTranscriber(transcriptionAPIKey, transcriptionURL, int(transcriptionTimeout), transcriptionModel, transcriptionLanguage); transcriber != nil {
		telegramHandlers.Transcriber = transcriber
		model, url := transcriber.Config()
		slog.Info(fmt.Sprintf("Voice transcription: Enabled (Model: %s, BaseURL: %s)", model, url))
	} else {
		slog.Info("Voice transcription: Disabled (set TRANSCRIPTION_API_KEY or OPENAI_API_KEY); daily reports accept text only")
	}

	// Initialize and start Telegram bot
	slog.Info("Initializing Telegram bot...")
	bot, err := telegram.NewBot(telegramToken, telegramHandlers, dishGroupID, adminID)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to initialize Telegram bot: %v", err))
		os.Exit(1)
	}
	slog.Info(fmt.Sprintf("Access control configured: GroupID=%d, OwnerID=%d", dishGroupID, adminID))

	// Free-text assistants for the admin and participants (need Anthropic tool use).
	if sdk, model := llmClient.AnthropicSDK(); sdk != nil {
		choreLoc, err := time.LoadLocation(getEnv("CHORE_TIMEZONE", "Europe/Berlin"))
		if err != nil {
			choreLoc = time.Local
		}
		effort := llm.ParseEffort(assistantEffort, anthropic.OutputConfigEffortMedium)
		bot.EnableAssistants(telegram.AssistantConfig{
			Client:                sdk,
			Model:                 model,
			Effort:                effort,
			Store:                 store,
			Location:              choreLoc,
			ParticipantDailyLimit: participantDailyLimit,
		})
		slog.Info(fmt.Sprintf("Assistants: Enabled (Model: %s, Effort: %s, participant daily limit: %d, 0 = participants off)", model, effort, participantDailyLimit))
	} else {
		slog.Info("Assistants: Disabled (need the Anthropic LLM provider)")
	}

	// Start bot in background
	botCtx, botCancel := context.WithCancel(ctx)
	defer botCancel()
	go bot.Start(botCtx)

	// Launch periodic chore reminders goroutine.
	// Avoid passing a typed-nil *llm.Client into the interface.
	var choreNudger notification.ChoreNudger
	if llmClient != nil {
		choreNudger = llmClient
	}
	go notification.StartPeriodicChoreReminders(botCtx, bot, store, choreNudger, getEnv("CHORE_TIMEZONE", "Europe/Berlin"))

	// Initialize cron scheduler for scheduled jobs (all times in Europe/Berlin)
	slog.Info("Initializing cron scheduler...")
	berlinLoc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to load Europe/Berlin timezone: %v", err))
		os.Exit(1)
	}
	c := cron.New(cron.WithLocation(berlinLoc))

	// Every minute - assign recurring and scheduled chores whose run time has come
	_, err = c.AddFunc("* * * * *", func() {
		if err := telegramHandlers.ProcessRecurringChores(context.Background()); err != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to process recurring chores: %v", err), slog.String("component", "cron"))
		}
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule recurring chores job: %v", err))
		os.Exit(1)
	}

	// Daily at 07:30 AM Berlin - Assign today's duty
	_, err = c.AddFunc("30 7 * * *", func() {
		slog.Info("═══════════════════════════════════════════════════════════")
		slog.Info("Running daily duty assignment (07:30 AM Berlin)", slog.String("component", "cron"))
		slog.Info(fmt.Sprintf("Current time: %s", time.Now().In(berlinLoc).Format("2006-01-02 15:04:05 MST")), slog.String("component", "cron"))

		duty, err := sched.AssignTodaysDuty(context.Background())
		if err != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to assign today's duty: %v", err), slog.String("component", "cron"))
			return
		}

		if duty == nil {
			slog.Warn("WARNING: No duty was assigned (duty is nil)", slog.String("component", "cron"))
			return
		}

		slog.Info(fmt.Sprintf("✓ Successfully assigned duty to user %d (Assignment Type: %s)", duty.UserID, duty.AssignmentType), slog.String("component", "cron"))

		if duty.User == nil {
			slog.Error("ERROR: Duty.User is nil - cannot send notifications!", slog.String("component", "cron"))
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
			slog.Warn("WARNING: DISH_GROUP not configured (dishGroupID=0), skipping group notification", slog.String("component", "cron"))
			// Log skip to database
			if dbErr := store.LogNotification(context.Background(), duty.DutyDate, duty.UserID, "GROUP", "SKIPPED", "dishGroupID not configured"); dbErr != nil {
				slog.Error(fmt.Sprintf("ERROR: Failed to log group notification skip: %v", dbErr), slog.String("component", "cron"))
			}
		}

		slog.Info("Daily duty assignment completed", slog.String("component", "cron"))
		slog.Info("═══════════════════════════════════════════════════════════")
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule daily assignment job: %v", err))
		os.Exit(1)
	}

	// Daily at 21:00 PM Berlin - Mark duty as completed
	_, err = c.AddFunc("0 21 * * *", func() {
		slog.Info("Running daily duty completion (21:00 PM Berlin)", slog.String("component", "cron"))
		err := sched.CompleteTodaysDuty(context.Background())
		if err != nil {
			slog.Error(fmt.Sprintf("Error completing today's duty: %v", err), slog.String("component", "cron"))
		} else {
			slog.Info("Successfully marked today's duty as completed", slog.String("component", "cron"))
		}
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule daily completion job: %v", err))
		os.Exit(1)
	}

	// Daily at 20:00 Berlin - Ask participants for their report of merits and failures
	_, err = c.AddFunc("0 20 * * *", func() {
		slog.Info("Running daily report requests (20:00 Berlin)", slog.String("component", "cron"))
		if adminID == 0 {
			slog.Info("Daily report requests skipped: ADMIN_ID is not configured", slog.String("component", "cron"))
			return
		}
		n, err := telegramHandlers.SendDailyReportRequests(context.Background(), time.Now().In(berlinLoc))
		if err != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to send daily report requests: %v", err), slog.String("component", "cron"))
			return
		}
		slog.Info(fmt.Sprintf("Sent %d daily report request(s)", n), slog.String("component", "cron"))
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule daily report requests job: %v", err))
		os.Exit(1)
	}

	// Daily at 20:45 Berlin (handlers.ReportDeadline) - Send the admin every
	// report still open, and a "no report" assessment for those who sent
	// nothing, so all ratings can be confirmed before 21:00
	_, err = c.AddFunc("45 20 * * *", func() {
		n, err := telegramHandlers.FinalizeDailyReports(context.Background(), time.Now().In(berlinLoc))
		if err != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to finalize daily reports: %v", err), slog.String("component", "cron"))
			return
		}
		slog.Info(fmt.Sprintf("Finalized %d unsent daily report(s) (20:45 Berlin)", n), slog.String("component", "cron"))
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule daily report finalization job: %v", err))
		os.Exit(1)
	}

	// Daily at 21:00 Berlin - On the last calendar day, publish the monthly participant rating winners
	_, err = c.AddFunc("0 21 * * *", func() {
		slog.Info("Checking month-end participant ratings announcement (21:00 Berlin)", slog.String("component", "cron"))

		if dishGroupID == 0 {
			slog.Info("Month-end participant ratings announcement skipped: DISH_GROUP is not configured", slog.String("component", "cron"))
			return
		}

		msg, ok, err := telegramHandlers.BuildMonthlyRatingsWinnersAnnouncement(time.Now().In(berlinLoc))
		if err != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to build month-end participant ratings announcement: %v", err), slog.String("component", "cron"))
			return
		}
		if !ok {
			slog.Info("Month-end participant ratings announcement skipped: today is not the last calendar day of the month", slog.String("component", "cron"))
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

	// Sunday at 21:10 PM Berlin - Send weekly stats
	_, err = c.AddFunc("10 21 * * 0", func() {
		slog.Info("Running weekly stats (Sunday 21:10 PM Berlin)", slog.String("component", "cron"))
		err := notification.SendWeeklyChoreStats(context.Background(), bot.API(), store, dishGroupID)
		if err != nil {
			slog.Error(fmt.Sprintf("Error sending weekly chore stats: %v", err), slog.String("component", "cron"))
		} else {
			slog.Info("Successfully sent weekly chore stats", slog.String("component", "cron"))
		}
		slog.Info("Weekly stats job executed", slog.String("component", "cron"))
	})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to schedule weekly stats job: %v", err))
		os.Exit(1)
	}

	// Start cron scheduler
	c.Start()
	slog.Info("═══════════════════════════════════════════════════════════")
	slog.Info("Cron scheduler started with 6 jobs:")
	slog.Info("  1. Daily at 07:30 AM Berlin - Assign today's duty and send notifications")
	slog.Info("  2. Daily at 21:00 PM Berlin - Mark today's duty as completed")
	slog.Info("  3. Daily at 20:00 PM Berlin - Ask participants for their daily report")
	slog.Info("  4. Daily at 20:45 PM Berlin - Send the admin open and missing daily reports to rate")
	slog.Info("  5. Daily at 21:00 PM Berlin - Publish month-end participant rating winners on the last calendar day")
	slog.Info("  6. Sunday at 21:10 PM Berlin - Send weekly stats")
	slog.Info(fmt.Sprintf("Current Berlin time: %s", time.Now().In(berlinLoc).Format("2006-01-02 15:04:05 MST")))
	slog.Info("═══════════════════════════════════════════════════════════")

	// Initialize HTTP server with Gin
	slog.Info("Initializing HTTP server on :8080...")
	dutySecret := getEnv("DUTY_SECRET", "")
	if dutySecret == "" {
		slog.Warn("WARNING: DUTY_SECRET is not set. The /who endpoint will return 503 until it is configured.")
	}
	router := httpserver.NewServer(store, telegramToken, dutySecret)

	// Create HTTP server for graceful shutdown
	srv := createHTTPServer(router)

	// Start HTTP server in background
	go func() {
		slog.Info("HTTP server listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error(fmt.Sprintf("HTTP server error: %v", err))
			os.Exit(1)
		}
	}()

	slog.Info("Roster Bot v0.1.0 initialized successfully")
	slog.Info("Press Ctrl+C to shut down")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down gracefully...")

	// Stop cron scheduler
	slog.Info("Stopping cron scheduler...")
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

	slog.Info("Roster Bot stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// resolveLLMProvider picks the LLM provider. An explicit LLM_PROVIDER value
// ("openai" or "anthropic") wins; otherwise Anthropic is used when
// ANTHROPIC_API_KEY is set, falling back to the OpenAI-compatible API.
func resolveLLMProvider(explicit, anthropicAPIKey string) string {
	switch p := strings.ToLower(strings.TrimSpace(explicit)); p {
	case llm.ProviderOpenAI, llm.ProviderAnthropic:
		return p
	case "":
	default:
		slog.Warn("Unknown LLM_PROVIDER value (expected openai or anthropic), auto-detecting from API keys")
	}
	if anthropicAPIKey != "" {
		return llm.ProviderAnthropic
	}
	return llm.ProviderOpenAI
}

func parseInt64(s string, defaultValue int64) int64 {
	var result int64
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
		return defaultValue
	}
	return result
}

func createHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}
}
