---
# Admin /complete Command - Mark Any Active Chore as Completed

## Overview
Implement an admin-only `/complete` command that allows admins to mark any active chore as completed. This handles the use case where a user completes a task
offline and the admin verifies it in person. The command should show a list of all active chores, allow the admin to select one, and send the same group
notification as when a user self-completes.

## Context
- Files involved:
  - `internal/telegram/handlers/admin.go` (new handler function)
  - `internal/telegram/bot.go` (register new command and callback)
  - `internal/telegram/handlers/commands.go` (update help message)
  - `internal/telegram/handlers/chore_reminder.go` (reuse SendCompletionToGroup)
- Related patterns:
  - Admin check via `checkAdmin()` function
  - Inline keyboard for selection (similar to `/assign`, `/modify`)
  - Group notification via `SendCompletionToGroup()`
  - CompleteChoreByReminderID for database update
- Dependencies: None (uses existing store and notification patterns)

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- Follow existing admin command patterns (inline keyboards, callback handling)
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Create HandleComplete command handler

**Files:**
- Modify: `internal/telegram/handlers/admin.go`

- [x] Add `HandleComplete(m *tgbotapi.Message)` function that checks admin status
- [x] Fetch all active chores using `h.Store.GetActiveChores(ctx)`
- [x] If no active chores, return message saying so
- [x] Create inline keyboard with chore buttons showing: description, assigned user, deadline
- [x] Use callback data format: `complete_chore:reminderID`
- [x] Write unit tests for HandleComplete (with mock store)
- [x] Run project test suite - must pass before task 2

### Task 2: Create HandleCompleteChoreCallback handler

**Files:**
- Modify: `internal/telegram/handlers/admin.go`

- [x] Add `HandleCompleteChoreCallback(q *tgbotapi.CallbackQuery)` function
- [x] Parse reminderID from callback data
- [x] Get chore by reminderID using `h.Store.GetChoreByReminderID(ctx, reminderID)`
- [x] Send group notification using `h.ChoreReminderManager.SendCompletionToGroup(assignment)`
- [x] Mark chore as complete using `h.Store.CompleteChoreByReminderID(ctx, reminderID)`
- [x] Remove from active tracking using `h.ChoreReminderManager.CompleteChore(reminderID)`
- [x] Update callback message to show completion confirmation
- [x] Write unit tests for HandleCompleteChoreCallback
- [x] Run project test suite - must pass before task 3

### Task 3: Register command and callback in bot router

**Files:**
- Modify: `internal/telegram/bot.go`

- [ ] Add `case "complete"` route to handleCommand() switch
- [ ] Add `case "complete_chore"` route to handleCallbackQuery() switch
- [ ] Run project test suite - must pass before task 4

### Task 4: Update help message

**Files:**
- Modify: `internal/telegram/handlers/commands.go`

- [ ] Add `/complete - Admin: Mark any active chore as completed` to helpMessage admin commands section
- [ ] Run project test suite - must pass before task 5

### Task 5: Verify acceptance criteria

- [ ] manual test: Run bot, use /complete as admin, verify chore list appears
- [ ] manual test: Select a chore, verify group notification matches self-completion format
- [ ] manual test: Verify chore is removed from active tracking (no more reminders)
- [ ] manual test: Verify non-admins get admin-only error
- [ ] run full test suite
- [ ] run linter
- [ ] verify test coverage meets 80%+

### Task 6: Update documentation

- [ ] update README.md if user-facing changes (add /complete to command list)
- [ ] update CLAUDE.md if internal patterns changed
- [ ] move this plan to `docs/plans/completed/`
