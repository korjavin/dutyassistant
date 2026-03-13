---
# Add Admin Command to Edit Periodic Chore Description

## Overview
Add an `/edit chore <id> <new description>` admin command that updates the description of an active `RecurringChore` (periodic scheduled chore). Follows the existing `/cancel chore <id>` and `/list chore` conventions — direct arguments, no interactive session or inline buttons.

## Context
- Files involved:
  - `internal/store/store.go` — add `UpdateRecurringChoreDescription` to Store interface
  - `internal/store/sqlite/sqlite_chore.go` — implement `UpdateRecurringChoreDescription`
  - `internal/telegram/handlers/list_cancel.go` — add `HandleEdit` handler alongside list/cancel
  - `internal/telegram/bot.go` — register `"edit"` command
  - `internal/telegram/handlers/recurring_chore_test.go` — add handler tests
  - `internal/store/sqlite/sqlite_chore_test.go` — add store method test
- Related patterns: `/cancel chore <id>` in `list_cancel.go`; recurring chore store methods in `sqlite_chore.go`
- Dependencies: none

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add UpdateRecurringChoreDescription to store layer

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite/sqlite_chore.go`

- [ ] Add `UpdateRecurringChoreDescription(ctx context.Context, id int64, description string) error` to the Store interface in `store.go` (alongside other recurring chore methods)
- [ ] Implement in `sqlite_chore.go`: `UPDATE recurring_chores SET description = ? WHERE id = ? AND is_active = 1`; return descriptive error if 0 rows affected (not found or already cancelled)
- [ ] Add mock method to `internal/mocks/` (check if mock is generated or hand-written; update accordingly)
- [ ] Write unit test for the SQLite method in `sqlite_chore_test.go`
- [ ] Run `go test ./...` — must pass before task 2

### Task 2: Add HandleEdit handler

**Files:**
- Modify: `internal/telegram/handlers/list_cancel.go`

- [ ] Add `HandleEdit(m *tgbotapi.Message) (tgbotapi.MessageConfig, error)` following the structure of `HandleCancel`
- [ ] Admin check via `h.checkAdmin`
- [ ] Parse args: expect format `chore <id> <description>` (3+ parts where parts[0]=="chore", parts[1] is numeric ID, rest is the new description)
- [ ] Return usage hint if args don't match: "Use /edit chore <id> <new description>"
- [ ] Fetch chore via `h.Store.GetRecurringChore()` — return error if not found or not active
- [ ] Call `h.Store.UpdateRecurringChoreDescription(ctx, id, newDescription)`
- [ ] Return confirmation: "✅ Periodic chore description updated:\nOld: <old>\nNew: <new>"
- [ ] Write tests in `recurring_chore_test.go`: non-admin rejection, invalid args, chore not found, chore inactive, successful update
- [ ] Run `go test ./...` — must pass before task 3

### Task 3: Register command in bot.go

**Files:**
- Modify: `internal/telegram/bot.go`

- [ ] Add `case "edit": return b.handlers.HandleEdit(m)` in `handleCommand` switch (alongside list/cancel)
- [ ] Run `go test ./...` — must pass before task 4

### Task 4: Verify acceptance criteria

- [ ] manual test: admin sends `/edit chore 1 New description here`, bot replies with old and new description
- [ ] manual test: non-admin gets "admin only" reply
- [ ] manual test: `/edit chore 999 text` on nonexistent ID returns error
- [ ] run full test suite: `go test ./...`
- [ ] run linter: `golangci-lint run` (or equivalent)
- [ ] verify test coverage meets 80%+

### Task 5: Update documentation

- [ ] update CLAUDE.md if internal patterns changed
- [ ] move this plan to `docs/plans/completed/`
