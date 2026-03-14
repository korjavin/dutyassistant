# Refactor Go Logs to Use slog

## Overview
Replace all usages of the standard `log` package with `log/slog` (Go 1.21+ structured logging). Configure a default slog handler in main.go. Convert all log calls to structured slog calls with appropriate levels and key-value attributes. Custom string prefixes like [CRON], [ACCESS] become slog attributes (e.g., "component", "cron").

## Context
- Files involved: 16 Go files across cmd/, internal/llm/, internal/telegram/, internal/notification/, internal/http/middleware/
- Go version: 1.23 - slog is available in stdlib
- Current pattern: global `log.Printf/Println/Fatalf` with string prefixes for categorization
- Target pattern: package-level `slog.Info/Debug/Error/Warn` with structured key-value pairs
- One file (ratings.go) already imports slog but does not use it

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- Use package-level slog functions (slog.Info, slog.Error, etc.) - consistent with existing global log usage
- Convert string prefixes [CRON], [ACCESS], [WEB_AUTH] to slog attribute: "component", "cron" etc.
- log.Fatalf → slog.Error + os.Exit(1)
- log.Printf → slog.Info or slog.Debug or slog.Error based on content
- log.Println → slog.Info or slog.Debug based on content
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Configure slog default handler in main.go

**Files:**
- Modify: `cmd/roster-bot/main.go`

- [ ] Add slog.SetDefault call at start of main() with NewTextHandler writing to os.Stderr
- [ ] Set HandlerOptions with LevelDebug to preserve existing verbosity
- [ ] Replace all log.Printf/Println/Fatalf calls in main.go with appropriate slog calls
- [ ] Convert [CRON] prefix usages to slog calls with "component", "cron" attribute
- [ ] Replace log.Fatal/Fatalf with slog.Error + os.Exit(1)
- [ ] Remove "log" import, add "log/slog" and "os" if not present
- [ ] Run: go build ./... (must succeed before task 2)

### Task 2: Refactor internal/llm package

**Files:**
- Modify: `internal/llm/openai.go`

- [ ] Replace log.Printf/Println calls with slog.Info/slog.Error/slog.Debug
- [ ] Remove "log" import, use "log/slog"
- [ ] Run: go build ./...

### Task 3: Refactor internal/telegram package

**Files:**
- Modify: `internal/telegram/bot.go`
- Modify: `internal/telegram/handlers/commands.go`
- Modify: `internal/telegram/handlers/handlers.go`
- Modify: `internal/telegram/handlers/chore.go`
- Modify: `internal/telegram/handlers/schedule.go`
- Modify: `internal/telegram/handlers/edit_interactive.go`
- Modify: `internal/telegram/handlers/chore_interactive.go`
- Modify: `internal/telegram/handlers/cancel_interactive.go`
- Modify: `internal/telegram/handlers/admin.go`
- Modify: `internal/telegram/handlers/chore_callback.go`
- Modify: `internal/telegram/handlers/chore_cron.go`
- Modify: `internal/telegram/handlers/ratings.go`

- [ ] Replace log.Printf/Println calls with slog.Info/slog.Error/slog.Debug in each file
- [ ] Convert [ACCESS] prefix to "component", "access" attribute in bot.go
- [ ] In ratings.go: remove the unused slog import (if still unused) or add actual slog calls
- [ ] Remove "log" imports, use "log/slog" where needed
- [ ] Run: go build ./...

### Task 4: Refactor internal/notification and internal/http/middleware

**Files:**
- Modify: `internal/notification/notifier.go`
- Modify: `internal/http/middleware/auth.go`

- [ ] Replace log.Printf/Println calls in notifier.go with slog.Info/slog.Error/slog.Debug
- [ ] Replace log.Printf/Println calls in auth.go with slog.Info/slog.Debug/slog.Warn
- [ ] Convert [WEB_AUTH] prefix to "component", "web_auth" attribute
- [ ] Remove "log" imports, add "log/slog"
- [ ] Run: go build ./...

### Task 5: Verify acceptance criteria

- [ ] Run: go build ./...
- [ ] Run: go vet ./...
- [ ] Run: go test ./... (all tests must pass)
- [ ] Manually verify no "log" package imports remain (grep -r '"log"' --include="*.go" .)
- [ ] Check that the application starts and logs appear with structured format

### Task 6: Update documentation

- [ ] Update CLAUDE.md if it mentions logging patterns
- [ ] Move this plan to `docs/plans/completed/`
