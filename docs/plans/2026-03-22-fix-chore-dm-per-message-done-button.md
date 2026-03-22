# Fix /chore in DMs: Send Each Chore as Separate Message with Done Button

## Overview
When a non-admin user sends /chore in a private message, instead of listing all chores in a single text block, send one message per chore, each with a "Mark as Done" inline button.

## Context
- Files involved:
  - `internal/telegram/handlers/chore.go` — non-admin path to modify
  - `internal/telegram/handlers/chore_callback.go` — new callback handler to add
  - `internal/telegram/bot.go` — register new callback route
  - `internal/telegram/handlers/chore_test.go` — tests (if exists, otherwise create)
- Related patterns:
  - `Handlers.Bot *tgbotapi.BotAPI` is available for direct sends
  - `Handlers.GroupID int64` for group notifications
  - `Handlers.ChoreReminderManager` for in-memory cleanup + group notification
  - Store has `GetChoreByID(ctx, id)` and `CompleteChoreByReminderID(ctx, reminderID)`
  - Callback data format: `action:param`, 64-byte limit (safe: "chore_list_done:19digits" = 35 chars max)

## Development Approach
- Testing approach: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Modify non-admin HandleChore to send per-chore messages

**Files:**
- Modify: `internal/telegram/handlers/chore.go`

- [ ] Replace the single-message text builder in the non-admin /chore path (lines 48-67) with a loop that calls `h.Bot.Send()` for each chore
- [ ] Each message body: chore description + assigned date (same timezone logic as before)
- [ ] Each message gets an inline keyboard with one button: "✅ Mark as Done" with callback data `chore_list_done:<choreID>`
- [ ] Return `nil, nil` after sending all per-chore messages (no additional bot.Send in caller)
- [ ] Keep the "no chores" empty-state message as-is (single return)
- [ ] Write/update tests for the non-admin path in `internal/telegram/handlers/chore_test.go`
- [ ] Run project test suite — must pass before task 2

### Task 2: Add HandleChoreListDoneCallback

**Files:**
- Modify: `internal/telegram/handlers/chore_callback.go`

- [ ] Add `HandleChoreListDoneCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error)`
- [ ] Parse `chore_list_done:<choreID>` from callback data
- [ ] Fetch chore from DB: `h.Store.GetChoreByID(ctx, choreID)`
- [ ] If chore not found or already completed, edit message to show appropriate error
- [ ] Get caller's user record: `h.Store.GetUserByTelegramID(ctx, q.From.ID)` to verify ownership
- [ ] If caller's DB user ID != chore.UserID, edit message with "This chore is not assigned to you"
- [ ] Mark complete in DB: `h.Store.CompleteChoreByReminderID(ctx, chore.ReminderID)`
- [ ] Remove from in-memory manager if present: `h.ChoreReminderManager.CompleteChore(chore.ReminderID)` (no-op if not found)
- [ ] Send group notification via `h.ChoreReminderManager.SendCompletionToGroup(&ChoreAssignment{UserID: q.From.ID, UserName: q.From.FirstName, Description: chore.Description, GroupID: h.GroupID})`
- [ ] Edit message to show "✅ Chore marked as done: <description>"
- [ ] Write tests for the new callback handler
- [ ] Run project test suite — must pass before task 3

### Task 3: Register new callback route in bot.go

**Files:**
- Modify: `internal/telegram/bot.go`

- [ ] Add `case "chore_list_done":` in `handleCallbackQuery` routing to `b.handlers.HandleChoreListDoneCallback(q)`
- [ ] Run project test suite — must pass before task 4

### Task 4: Verify acceptance criteria

- [ ] Manual test: send /chore in DM — verify each chore appears as its own message with the button
- [ ] Press "Mark as Done" on a chore — verify message updates and group is notified
- [ ] Press "Mark as Done" on a chore belonging to another user — verify error message
- [ ] Run full test suite: `go test ./...`
- [ ] Run linter: `go vet ./...`

### Task 5: Update documentation

- [ ] No README changes needed (internal UX change)
- [ ] Move this plan to `docs/plans/completed/`
