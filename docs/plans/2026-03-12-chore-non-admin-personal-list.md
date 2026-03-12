# /chore for Non-Admin: Show Personal Chore List

## Overview

When a non-admin user sends `/chore`, instead of getting "admin only" message, they see a list of their currently active (unfinished) chores with each chore's description and assignment time.

## Context

- Files involved:
  - `internal/store/store.go` — add `GetActiveChoresByUserID` to interface
  - `internal/store/sqlite/chores.go` — implement the new method
  - `internal/mocks/store.go` — add mock for new method
  - `internal/telegram/handlers/chore.go` — change non-admin path in `HandleChore`
  - `internal/store/sqlite/sqlite_chore_test.go` — store-level tests
  - `internal/telegram/handlers/chore_test.go` — handler-level tests
- Related patterns: existing `GetActiveChores` method, same scan helpers, HTML formatting with `tgbotapi.ModeHTML`
- Dependencies: none new

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add store method and SQLite implementation

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite/chores.go`
- Modify: `internal/mocks/store.go`
- Modify: `internal/store/sqlite/sqlite_chore_test.go`

- [x] Add `GetActiveChoresByUserID(ctx context.Context, userID int64) ([]*Chore, error)` to `Store` interface in `store.go`
- [x] Implement in `sqlite/chores.go`: query chores where `user_id = ?` and `completed_at IS NULL`, join users table, reuse `scanChoreRowsWithUser`
- [x] Add mock method to `internal/mocks/store.go`
- [x] Write tests in `sqlite_chore_test.go` covering: returns empty list for user with no chores, returns only incomplete chores for the user, does not return another user's chores
- [x] Run `go test ./internal/store/...` — must pass

### Task 2: Modify HandleChore for non-admin users

**Files:**
- Modify: `internal/telegram/handlers/chore.go`
- Modify: `internal/telegram/handlers/chore_test.go`

- [x] In `HandleChore`, replace the early-return `adminOnlyMessage` block with: look up calling user by telegram ID, fetch their active chores via `GetActiveChoresByUserID`, format a list showing description and `assigned_at` per chore, return it
- [x] If user not found in DB, return a message directing them to use `/start`
- [x] If user has no active chores, return a friendly "no active chores" message
- [x] HTML-escape descriptions in the output
- [x] Write tests in `chore_test.go`:
  - `TestHandleChore_NonAdmin_NoChores` — non-admin with no chores sees "no active chores" message
  - `TestHandleChore_NonAdmin_WithChores` — non-admin with active chores sees formatted list with description and assigned time
  - `TestHandleChore_NonAdmin_UserNotRegistered` — non-admin not in DB sees "/start" prompt
- [x] Run `go test ./internal/telegram/...` — must pass

### Task 3: Update help text

**Files:**
- Modify: `internal/telegram/handlers/commands.go`

- [x] Update `helpMessage` to document that non-admins can use `/chore` to see their assigned chores
- [x] Run `go test ./...` — must pass

### Task 4: Verify acceptance criteria

- [x] Manual test: non-admin user sends `/chore`, receives list of their active chores with descriptions and assignment times
- [x] Manual test: admin sends `/chore <description>`, behavior unchanged
- [x] Run full test suite: `go test ./...`
- [x] Run linter: `go vet ./...`

### Task 5: Update documentation

- [ ] Update README.md if it documents bot commands
- [ ] Move this plan to `docs/plans/completed/`
