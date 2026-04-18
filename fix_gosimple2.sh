#!/bin/bash
sed -i 's/slog.Info(fmt.Sprintf("Announced recurring chore in group."))/slog.Info("Announced recurring chore in group.")/g' internal/telegram/handlers/chore_cron.go
sed -i 's/slog.Info(fmt.Sprintf("No group configured to announce recurring chore."))/slog.Info("No group configured to announce recurring chore.")/g' internal/telegram/handlers/chore_cron.go
sed -i 's/slog.Info(fmt.Sprintf("Bot API not available for group announcement."))/slog.Info("Bot API not available for group announcement.")/g' internal/telegram/handlers/chore_cron.go
