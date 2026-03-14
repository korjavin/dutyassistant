---
# Add Periodic Chore DM Reminders (Kid-Safe Hours)

## Overview
Add a goroutine-based random scheduler that sends DM reminders about active (unfinished) chores to each assigned user. Reminders only fire between 11:00 and 18:00 in the chore timezone, spaced randomly 3-6 hours apart. The goal is a gentle nudge, not nagging.

## Context
- Files involved:
  - `cmd/roster-bot/main.go` - wiring the scheduler goroutine at startup
  - `internal/store/store.go` - check/add GetActiveChoresForAllUsers method
  - `internal/store/sqlite/chores.go` - SQL implementation
  - `internal/notification/formatter.go` - add periodic reminder message formatter
  - `internal/notification/periodic_chore_reminder.go` - new: scheduler + send logic
- Related patterns: existing cron jobs use `berlinLoc` / `CHORE_TIMEZONE` env var; DMs sent via `bot.SendMessageHTML(user.TelegramUserID, msg)`; chore struct has `CompletedAt`/`CancelledAt` nil-check for active state
- Dependencies: none new (uses existing robfig/cron, tgbotapi already vendored)

## Development Approach
- Testing approach: Regular (code first, then tests)
- Complete each task fully before moving to the next
- CRITICAL: every task MUST include new/updated tests
- CRITICAL: all tests must pass before starting next task

## Implementation Steps

### Task 1: Add store method for listing active chores with user info

**Files:**
- Modify: `internal/store/store.go` (add interface method if missing)
- Modify: `internal/store/sqlite/chores.go` (add SQL query)
- Modify: `internal/mocks/mock_store.go` (add mock method)

- [ ] Check if a method exists that returns all active (not completed, not cancelled) chores with their User populated; if not, add `ListActiveChoresWithUsers(ctx context.Context) ([]Chore, error)` to the Store interface
- [ ] Implement in sqlite: `SELECT chores.*, users.* FROM chores JOIN users ON chores.user_id = users.id WHERE chores.completed_at IS NULL AND chores.cancelled_at IS NULL`
- [ ] Update mock to add the method
- [ ] Write unit test for the new store method using an in-memory SQLite db
- [ ] Run project test suite - must pass before task 2

### Task 2: Add formatter for periodic chore reminder message

**Files:**
- Modify: `internal/notification/formatter.go`

- [ ] Add `FormatPeriodicChoreReminder(chores []store.Chore) string` that produces a friendly HTML message listing the user's pending chores with their descriptions and deadlines
- [ ] Message tone: helpful, not nagging (e.g. "Just a reminder, you've got some chores on your list 🧹")
- [ ] Write unit test for the formatter covering 1 chore and multiple chores cases
- [ ] Run project test suite - must pass before task 3

### Task 3: Create periodic reminder scheduler

**Files:**
- Create: `internal/notification/periodic_chore_reminder.go`

- [ ] Define `StartPeriodicChoreReminders(ctx context.Context, bot BotSender, store Store, timezone string)` as the entry point (runs forever until ctx done)
- [ ] Implement `nextReminderTime(now time.Time, loc *time.Location) time.Time`:
  - Add random 3-6 hours to now
  - If result is before 11:00, advance to 11:00 same day
  - If result is after 18:00, advance to 11:00 next day + small random offset (0-30min) to avoid always firing exactly at open
  - Return the computed time
- [ ] Implement `sendChoreReminders(ctx, bot, store, loc)`:
  - Double-check current time is within 11:00-18:00 (safety guard)
  - Fetch all active chores via `ListActiveChoresWithUsers`
  - Group by user
  - For each user with active chores, send DM via `bot.SendMessageHTML`
  - Log each send attempt
- [ ] Loop: wait until `nextReminderTime`, call `sendChoreReminders`, repeat
- [ ] Write unit tests for `nextReminderTime` covering: within window, before 11am, after 6pm, wrapping to next day
- [ ] Run project test suite - must pass before task 4

### Task 4: Wire up in main.go and verify

**Files:**
- Modify: `cmd/roster-bot/main.go`

- [ ] After bot and store are initialized, launch `go notification.StartPeriodicChoreReminders(botCtx, bot, store, tz)` in its own goroutine (uses existing `botCtx` so it shuts down gracefully)
- [ ] Verify bot satisfies the `BotSender` interface expected by the new function (or use concrete type)
- [ ] Run full test suite - must pass
- [ ] Run linter

### Task 5: Verify acceptance criteria

- [ ] Manual test: assign a chore to a user, wait for the 11am window, verify DM is received
- [ ] Manual test: verify no DM is sent outside 11am-6pm window
- [ ] Run full test suite: `go test ./...`
- [ ] Run linter: `go vet ./...`
- [ ] Verify test coverage meets 80%+ for new code

### Task 6: Update documentation

- [ ] Update README.md if there is a section on scheduled notifications
- [ ] Move this plan to `docs/plans/completed/`
