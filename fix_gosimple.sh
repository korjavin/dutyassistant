#!/bin/bash
sed -i 's/slog.Error(fmt.Sprintf("\[CRON\] ERROR: Failed to assign recurring chore %d: %v", chore.ID, err))/slog.Error(fmt.Sprintf("\[CRON\] ERROR: Failed to assign recurring chore %d: %v", chore.ID, err))/g' internal/telegram/handlers/chore_cron.go

# Oh wait, the error is at `internal/telegram/handlers/chore_cron.go:163`, let's see what that is.
