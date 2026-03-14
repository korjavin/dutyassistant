#!/bin/bash

sed -i 's/slog.Info(fmt.Sprintf("LLM RefineMessage no choices returned"))/slog.Info("LLM RefineMessage no choices returned")/g' internal/llm/openai.go

sed -i 's/slog.Info(fmt.Sprint("Stopping notifier..."))/slog.Info("Stopping notifier...")/g' internal/notification/notifier.go
sed -i 's/slog.Info(fmt.Sprint("Notifier stopped."))/slog.Info("Notifier stopped.")/g' internal/notification/notifier.go
sed -i 's/slog.Info(fmt.Sprint("Cron job triggered: checking for tomorrow'\''s duty."))/slog.Info("Cron job triggered: checking for tomorrow'\''s duty.")/g' internal/notification/notifier.go

sed -i 's/msgText = fmt.Sprintf("✅ Periodic chore cancelled successfully.")/msgText = "✅ Periodic chore cancelled successfully."/g' internal/telegram/handlers/cancel_interactive.go

sed -i 's/slog.Info(fmt.Sprintf("\[CHORE\] Starting weighted selection for chore assignment"))/slog.Info("\[CHORE\] Starting weighted selection for chore assignment")/g' internal/telegram/handlers/chore.go
