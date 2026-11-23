package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/notification"
	"github.com/korjavin/dutyassistant/internal/store"
)

type morningDutyStore interface {
	notification.MorningDutyReader
	LogNotification(ctx context.Context, dutyDate time.Time, userID int64, notificationType, status, errorMessage string) error
}

func runMorningDuty(
	ctx context.Context,
	assign func(context.Context) (duty *store.Duty, queueErr, assignmentErr error),
	st morningDutyStore,
	llmClient *llm.Client,
	sendHTML func(chatID int64, text string) error,
	dishGroupID int64,
	berlin *time.Location,
) {
	logger := slog.With("component", "cron")
	logger.Info("Running daily duty assignment (07:30 AM Berlin)", "time", time.Now().In(berlin))

	duty, queueErr, err := assign(ctx)
	if err != nil {
		logger.Error("Failed to assign today's duty", "error", err)
		return
	}
	if duty == nil {
		logger.Warn("No duty was assigned (duty is nil)")
		return
	}
	if duty.User == nil {
		logger.Error("Duty has no user details", "user_id", duty.UserID, "assignment_type", duty.AssignmentType)
		return
	}
	logger = logger.With("user_id", duty.UserID, "date", duty.DutyDate.Format(time.DateOnly), "assignment_type", duty.AssignmentType)
	logger.Info("Successfully assigned duty", "name", duty.User.FirstName, "telegram_user_id", duty.User.TelegramUserID)

	var facts *notification.MorningDutyContext
	if queueErr != nil {
		logger.Error("Failed to decrement duty queue", "error", queueErr)
	} else if llmClient != nil && (duty.User.TelegramUserID != 0 || dishGroupID != 0) {
		facts, err = notification.ReadMorningDutyContext(ctx, st, duty, berlin)
		if err != nil {
			logger.Warn("Failed to read morning duty notification context", "error", err)
		}
	}

	logNotification := func(recipient, status, errorMessage string) {
		if err := st.LogNotification(ctx, duty.DutyDate, duty.UserID, recipient, status, errorMessage); err != nil {
			logger.Error("Failed to log duty notification", "recipient", recipient, "status", status, "error", err)
		}
	}
	deliver := func(recipient string, chatID int64, message string) {
		logger.Info("Sending duty notification", "recipient", recipient, "chat_id", chatID, "message", message)
		if err := sendHTML(chatID, message); err != nil {
			logger.Error("Failed to send duty notification", "recipient", recipient, "chat_id", chatID, "error", err)
			logNotification(recipient, "FAILED", err.Error())
			return
		}
		logger.Info("Successfully sent duty notification", "recipient", recipient, "chat_id", chatID)
		logNotification(recipient, "SUCCESS", "")
	}

	if duty.User.TelegramUserID != 0 {
		deliver("DM", duty.User.TelegramUserID, notification.ComposeMorningDutyDM(ctx, duty, facts, llmClient))
	} else {
		logger.Warn("Skipping duty DM because the user has no Telegram ID")
		logNotification("DM", "SKIPPED", "TelegramUserID is 0")
	}
	if dishGroupID != 0 {
		deliver("GROUP", dishGroupID, notification.ComposeMorningDutyGroup(ctx, duty, facts, llmClient))
	} else {
		logger.Warn("Skipping group duty notification because DISH_GROUP is unconfigured")
		logNotification("GROUP", "SKIPPED", "dishGroupID not configured")
	}
	logger.Info("Daily duty assignment completed")
}
