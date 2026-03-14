#!/bin/bash
sed -i 's/slog.Info(fmt.Sprintf("\[CRON\] Starting ProcessRecurringChores"))/slog.Info("\[CRON\] Starting ProcessRecurringChores")/g' internal/telegram/handlers/chore_cron.go
sed -i 's/slog.Info(fmt.Sprintf("\[CRON\] No recurring chores are due."))/slog.Info("\[CRON\] No recurring chores are due.")/g' internal/telegram/handlers/chore_cron.go
sed -i 's/slog.Warn(fmt.Sprintf("\[CHORE\] WARNING: Fallback selection triggered (this should not happen)"))/slog.Warn("\[CHORE\] WARNING: Fallback selection triggered (this should not happen)")/g' internal/telegram/handlers/chore.go
