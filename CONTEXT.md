# Development Context for Next Coding Agent

## Project Overview
**Duty Assistant Bot** - A Telegram bot for managing on-call duty rosters using a queue-based assignment system with interactive inline keyboard UI.

## Recent Work Completed

### Session: October 5, 2025

#### 1. Inline Keyboard UI Implementation (Commits: 27881d7, 9e298ec)
Transformed all bot commands from text-based prompts to interactive inline keyboards for improved UX.

**Commands Updated:**
- `/volunteer` - Day selection buttons (1-7 + Custom)
- `/assign` - User selection → days selection → confirmation flow
- `/modify` / `/change` - Date selection → user selection → confirmation flow
- `/toggleactive` - User selection with status indicators (✅ active / ❌ inactive)
- `/offduty` - User selection buttons → text input for dates

**Implementation Details:**
- All callback handlers created in `internal/telegram/handlers/admin.go` and `internal/telegram/handlers/volunteer.go`
- Callback routing added to `internal/telegram/bot.go` switch statement
- Callback data format: `action:param1:param2` (e.g., `assign_user:123`, `modify_date:2025-10-10`)
- Progressive disclosure pattern: Show options step by step, update message with EditMessageText
- Consistent emoji feedback: ✅ for success, ❌ for errors, 👤 for users, 📅 for dates

**Callback Actions Registered:**
```go
case "assign_user":        // User selected for /assign
case "assign_days":        // Days selected for /assign
case "assign_custom":      // Custom day input requested
case "volunteer_days":     // Days selected for /volunteer
case "volunteer_custom":   // Custom day input requested
case "modify_date":        // Date selected for /modify
case "modify_user":        // User selected for /modify
case "toggle_user":        // User selected for /toggleactive
case "offduty_user":       // User selected for /offduty
```

#### 2. Previous Session Work (Queue-Based System Implementation)
Context from previous session continuation:
- Implemented complete queue-based duty assignment system (replacing calendar-based)
- Database schema updated with volunteer_queue_days, admin_queue_days, off_duty_start/end, completed_at
- Cron scheduler added for automated tasks (11AM, 21PM, Sunday 21:10PM Berlin time)
- Timezone support fixed in Dockerfile (added tzdata to Alpine)
- Database cleaned on production server (pet.kfamcloud.com)
- See `IMPLEMENTATION_PLAN.md` for full 9-phase implementation details

## Current System Architecture

### Technology Stack
- **Language:** Go 1.23
- **Bot Framework:** go-telegram-bot-api/telegram-bot-api/v5
- **Database:** SQLite (modernc.org/sqlite - CGo-free)
- **Scheduler:** robfig/cron/v3
- **Web Framework:** Gin
- **Deployment:** Docker (Alpine + tzdata), GitHub Actions → Portainer webhook
- **Timezone:** Europe/Berlin for all cron jobs

### Key Files & Their Responsibilities

#### Bot Layer
- `internal/telegram/bot.go` - Main bot routing and callback dispatcher
- `internal/telegram/handlers/admin.go` - Admin commands and their callback handlers
- `internal/telegram/handlers/volunteer.go` - Volunteer command and callbacks
- `internal/telegram/handlers/commands.go` - Common commands (help, status, start)

#### Business Logic
- `internal/scheduler/scheduler.go` - Queue-based duty assignment logic
  - `AssignTodaysDuty()` - Runs at 11AM, implements priority: volunteer → admin → round-robin
  - `CompleteTodaysDuty()` - Runs at 21PM, marks duties as completed
  - `selectRoundRobinUser()` - Fairness based on last 14 days (excludes admin assignments)

#### Data Layer
- `internal/store/store.go` - Store interface definitions
- `internal/store/sqlite/sqlite.go` - SQLite implementation
- Database schema includes:
  - `users` table: volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
  - `duties` table: assignment_type (voluntary/admin/round_robin), completed_at

#### Entry Point
- `cmd/roster-bot/main.go` - Initializes cron scheduler with Berlin timezone

### Inline Keyboard Button Patterns

**Day Selection Grid (1-7 + Custom):**
```go
var buttons [][]tgbotapi.InlineKeyboardButton
row := []tgbotapi.InlineKeyboardButton{}
for days := 1; days <= 7; days++ {
    row = append(row, tgbotapi.NewInlineKeyboardButtonData(
        fmt.Sprintf("%d", days),
        fmt.Sprintf("action_name:%d", days),
    ))
    if days%4 == 0 || days == 7 {
        buttons = append(buttons, row)
        row = []tgbotapi.InlineKeyboardButton{}
    }
}
buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
    tgbotapi.NewInlineKeyboardButtonData("✏️ Custom", "action_custom"),
})
```

**User Selection List:**
```go
var buttons [][]tgbotapi.InlineKeyboardButton
for _, u := range users {
    row := []tgbotapi.InlineKeyboardButton{
        tgbotapi.NewInlineKeyboardButtonData(
            fmt.Sprintf("👤 %s", u.FirstName),
            fmt.Sprintf("action_user:%d", u.ID),
        ),
    }
    buttons = append(buttons, row)
}
```

**Callback Handler Pattern:**
```go
func (h *Handlers) HandleSomeCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
    parts := strings.Split(q.Data, ":")
    // Parse parts[1], parts[2], etc.

    // Execute business logic

    // Return updated message
    edit := tgbotapi.NewEditMessageText(
        q.Message.Chat.ID,
        q.Message.MessageID,
        "✅ Success message",
    )
    edit.ParseMode = tgbotapi.ModeHTML
    return edit, nil
}
```

## Known Issues & TODOs

### High Priority
1. **Weekly Statistics Not Implemented** - Sunday 21:10PM cron job exists but TODO comment in main.go
2. **Tests Broken** - Mock stores need regeneration after interface changes (not blocking, main app works)

### Medium Priority
1. **DISH_GROUP Notifications** - Not fully implemented for duty reassignments in /modify
2. **Off-Duty Date Selection** - Currently text input only, could be enhanced with calendar picker

### Low Priority
1. **Prognosis Cleanup** - Some references may remain (prognosis.go deleted but verify no imports remain)

## Environment Variables
Required:
- `TELEGRAM_APITOKEN` - Bot API token
- `DATABASE_PATH` - SQLite database file path (default: /app/data/roster.db)
- `DNS_NAME` - DNS name for web interface
- `ADMIN_ID` - Telegram user ID of admin (optional, can set via DB)
- `DISH_GROUP` - Telegram chat ID for group announcements (optional)
- `GIN_MODE` - Gin mode (default: debug)

## Deployment Details

### Production Server
- **Host:** pet.kfamcloud.com
- **Access:** Passwordless SSH with sudo
- **Container Runtime:** Podman under root
- **Orchestration:** Portainer
- **CI/CD:** GitHub Actions → builds image → triggers Portainer webhook

### Build Process
1. Multi-stage Docker build
2. Frontend build (npm) in first stage
3. Go build with vendored dependencies
4. Alpine final stage with tzdata package
5. Timezone set to Europe/Berlin

## Documentation Files
- `README.md` - Updated with inline keyboard UX, queue system, commands
- `logic.md` - Complete specification of queue system and interactive UX flows
- `IMPLEMENTATION_PLAN.md` - 9-phase implementation plan (phases 1-7 completed)
- `CHANGES.md` - Comprehensive changelog (if exists)

## Code Style & Patterns

### Error Handling
- Return user-friendly messages with ❌ emoji
- Log errors with `log.Printf()`
- Use `tgbotapi.ModeHTML` for formatted messages

### Message Formatting
- Use HTML tags: `<b>bold</b>`, `<code>code</code>`
- Emojis: ✅ success, ❌ error, 👤 user, 📅 date, 🏖 off-duty, 🔄 modify, 🙋 volunteer

### Commit Messages
- Follow conventional commits: `feat:`, `fix:`, `docs:`
- Include footer with Claude Code attribution

## Next Steps Suggestions
1. Implement weekly statistics report (Sunday 21:10PM job)
2. Add DISH_GROUP notifications for duty changes
3. Regenerate mock stores for tests
4. Consider calendar picker for date inputs (inline keyboard with month view)
5. Add user confirmation before toggling active status
6. Implement /clearqueue command for admin to reset queues

## Useful Commands
- **Build:** `go build -o /tmp/roster-bot ./cmd/roster-bot`
- **Test build:** Tests broken, skip for now
- **SSH to prod:** `ssh pet.kfamcloud.com`
- **View logs:** Check Portainer or `podman logs [container]`

## Git Workflow
1. Work on master branch (small project, no feature branches)
2. Commit with descriptive messages
3. Push to trigger GitHub Actions
4. Wait for deployment via Portainer webhook
5. Verify on production Telegram bot

---

**Last Updated:** October 5, 2025
**Session Ended:** After inline keyboard implementation completion
**Build Status:** ✅ Compiling successfully
**Deployment Status:** ✅ Pushed to production (commit 9e298ec)
