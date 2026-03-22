# Reduce Channel Message Verbosity

## Overview
Trim all channel-facing bot messages to be brief one-liners or compact summaries. DM messages are left unchanged.

## Context
- Files involved:
  - `internal/telegram/handlers/chore_reminder.go` (SendCompletionToGroup)
  - `internal/telegram/handlers/chore.go` (assignChore channel announcement)
  - `internal/telegram/handlers/chore_cron.go` (assignRecurringChore channel announcement)
  - `internal/notification/notifier.go` (SendDailyChoreSummary, SendWeeklyChoreStats)
- Related patterns: HTML parse mode with tgbotapi, html.EscapeString for user content
- Dependencies: none

## Development Approach
- Testing approach: Regular (code first, then tests)
- No new files needed, all edits are string literal changes
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Trim chore completion and assignment channel messages

**Files:**
- Modify: `internal/telegram/handlers/chore_reminder.go`
- Modify: `internal/telegram/handlers/chore.go`
- Modify: `internal/telegram/handlers/chore_cron.go`

- [x] In SendCompletionToGroup: replace multi-line completion message with `✅ <b>%s</b> completed: <i>%s</i>` (name, desc)
- [x] In assignChore: replace groupText with `🎯 <b>%s</b>: <i>%s</i>` (name, desc)
- [x] In assignRecurringChore: replace groupText with same one-liner format
- [x] Run tests: `go test ./internal/telegram/handlers/...`

### Task 2: Slim down daily chore summary

**Files:**
- Modify: `internal/notification/notifier.go`

- [x] Shorten section header to `⚠️ Overdue chores:`
- [x] Shorten category headers: `🔴 Critical (3+d):`, `🟠 Overdue (1-2d):`, `🟢 Due today:`
- [x] Shorten each chore line: remove verbose label prefixes, use `deadline: DATE (+N d)` format
- [x] Run tests: `go test ./internal/notification/...`

### Task 3: Slim down weekly stats

**Files:**
- Modify: `internal/notification/notifier.go`

- [ ] Remove bar chart rendering from per-user stats (remove renderBarChart call in SendWeeklyChoreStats)
- [ ] Remove avg execution time line per user
- [ ] Remove on-time/late indicator per user
- [ ] Each user becomes one line: `N. Name — X done`
- [ ] Keep winner line but shorten: `🥇 Winner: Name`
- [ ] Run tests: `go test ./internal/notification/...`

### Task 4: Verify acceptance criteria

- [ ] Manual review: confirm each modified message template looks correct
- [ ] Run full test suite: `go test ./...`
- [ ] Run linter: `go vet ./...`
- [ ] Move this plan to `docs/plans/completed/`
