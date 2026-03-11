---
# Add Cancel Support for Regular (One-Off) Chores

## Overview
Add the ability for admins to cancel regular (one-off) chores that were created by mistake. This requires: a way to list active regular chores with their IDs, a store method to cancel them, and extending the /cancel command with a new subcommand.

## Context
- Files involved:
  - `internal/store/store.go` - Store interface
  - `internal/store/sqlite/chores.go` - Regular chore SQL operations
  - `internal/store/sqlite/sqlite.go` - DB schema/migration
  - `internal/telegram/handlers/list_cancel.go` - /list and /cancel handlers
  - `internal/telegram/handlers/chore_test.go` (or similar) - Tests
- Related patterns:
  - Periodic chore cancel: sets `is_active = 0` (soft delete)
  - `/cancel chore <id>` pattern for periodic chores
  - `/list chore` pattern for listing periodic chores with IDs
- Dependencies: none new

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add cancelled_at column to chores table and store methods

**Files:**
- Modify: `internal/store/sqlite/sqlite.go` (schema migration)
- Modify: `internal/store/sqlite/chores.go` (new store methods)
- Modify: `internal/store/store.go` (extend interface)

- [ ] Add `cancelled_at TEXT` column to `chores` table schema and migration (consistent with soft-delete pattern used for recurring_chores)
- [ ] Add `cancelled_at` field to `Chore` struct in `internal/store/store.go`
- [ ] Add `CancelChore(ctx context.Context, id int64) error` to the Store interface
- [ ] Implement `CancelChore` in SQLiteStore: `UPDATE chores SET cancelled_at = ? WHERE id = ? AND completed_at IS NULL AND cancelled_at IS NULL`
- [ ] Add `ListActiveChores(ctx context.Context) ([]*store.Chore, error)` to the Store interface (returns chores where completed_at IS NULL AND cancelled_at IS NULL)
- [ ] Implement `ListActiveChores` in SQLiteStore with JOIN on users table
- [ ] Update existing queries that filter active chores to also exclude `cancelled_at IS NOT NULL`
- [ ] Write tests for `CancelChore` and `ListActiveChores` in sqlite test files
- [ ] Run project test suite - must pass before task 2

### Task 2: Extend /list and /cancel handlers

**Files:**
- Modify: `internal/telegram/handlers/list_cancel.go`

- [ ] Add `task` subcommand to `HandleList`: `/list task` returns a formatted list of active regular chores with their IDs, descriptions, assigned user, and assigned_at time (admin-only, consistent with `/list chore`)
- [ ] Add `task` subcommand to `HandleCancel`: `/cancel task <id>` cancels a regular chore by ID (admin-only)
  - Validate the chore exists and is not already completed or cancelled
  - Call `CancelChore` store method
  - Return confirmation message with chore description
- [ ] Update the unknown-command fallback messages in both handlers to mention the new subcommands
- [ ] Write handler tests for `/list task` and `/cancel task <id>` covering happy path, not-found, already-completed, and already-cancelled cases
- [ ] Run project test suite - must pass before task 3

### Task 3: Verify acceptance criteria

- [ ] Manual test: create a chore with `/chore`, run `/list task` to see it with its ID, run `/cancel task <id>` to cancel it, verify it no longer appears in `/list task`
- [ ] Manual test: verify `/cancel task <id>` on a non-existent ID returns a clear error
- [ ] Run full test suite (`go test ./...`)
- [ ] Run linter (`go vet ./...` or project-specific linter)
- [ ] Verify test coverage meets 80%+

### Task 4: Update documentation

- [ ] Update `README.md` to document `/list task` and `/cancel task <id>` commands
- [ ] Update `CLAUDE.md` if internal patterns changed
- [ ] Move this plan to `docs/plans/completed/`
