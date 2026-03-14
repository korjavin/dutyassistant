---
# Improve Admin /users Command

## Overview
Enhance the `/users` admin command to show each user's current (upcoming) duty assignments, and filter off-duty display to only show active (non-expired) periods.

## Context
- Files involved:
  - `internal/store/store.go` — Store interface (add new method)
  - `internal/store/sqlite/sqlite.go` — SQLite implementation (add new method)
  - `internal/store/mocks/store.go` — Mock (add mock method)
  - `internal/mocks/store.go` — Second mock (add mock method)
  - `internal/telegram/handlers/admin.go` — HandleUsers handler
  - `internal/telegram/handlers/admin_test.go` — Tests
- Related patterns: existing store methods like `GetDutiesByMonth`, `GetCompletedDutiesInRange`
- Dependencies: none new

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add GetFutureDuties to Store Interface and SQLite

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite/sqlite.go`

- [ ] Add `GetFutureDuties(ctx context.Context, from time.Time) ([]*Duty, error)` to the Store interface in store.go
- [ ] Implement `GetFutureDuties` in sqlite.go — query duties with `duty_date >= from` and `completed_at IS NULL`, join users table, order by duty_date ASC
- [ ] Run `go build ./...` to verify compilation

### Task 2: Add Mock Method

**Files:**
- Modify: `internal/store/mocks/store.go`
- Modify: `internal/mocks/store.go`

- [ ] Add `GetFutureDuties` mock method to both mock files following the existing pattern (e.g. `GetCompletedDutiesInRange`)
- [ ] Run `go build ./...` to verify compilation

### Task 3: Update HandleUsers Handler

**Files:**
- Modify: `internal/telegram/handlers/admin.go`

- [ ] Call `h.Store.GetFutureDuties(ctx, time.Now())` to get upcoming duties
- [ ] Build a `map[int64][]*store.Duty` from user ID to their upcoming duties
- [ ] In the per-user output loop, if the user has upcoming duties, append each as `  📅 Duty: YYYY-MM-DD` (or similar concise format)
- [ ] For off-duty display: only show the off-duty block if `u.OffDutyEnd != nil && !u.OffDutyEnd.Before(today)` (i.e., end date is today or in the future)
- [ ] Run `go build ./...` to verify

### Task 4: Update Tests

**Files:**
- Modify: `internal/telegram/handlers/admin_test.go`

- [ ] Update `TestHandleUsers_Success` to mock `GetFutureDuties` returning an empty slice (no assignments for Alice/Bob)
- [ ] Add test case `TestHandleUsers_WithAssignments` — user has an upcoming duty, verify the duty date appears in output
- [ ] Add test case `TestHandleUsers_ExpiredOffDuty` — user has off-duty with end date in the past, verify it does NOT appear in output
- [ ] Add test case `TestHandleUsers_ActiveOffDuty` — user has off-duty with end date today or future, verify it appears
- [ ] Run `go test ./internal/telegram/handlers/...` — must pass

### Task 5: Verify acceptance criteria

- [ ] manual test: run bot locally, execute /users as admin, verify assignments and off-duty display correctly
- [ ] run full test suite: `go test ./...`
- [ ] run linter: `golangci-lint run` (or `go vet ./...` if linter not configured)
- [ ] verify test coverage meets 80%+

### Task 6: Update documentation

- [ ] update README.md if user-facing changes (likely no changes needed)
- [ ] update CLAUDE.md if internal patterns changed
- [ ] move this plan to `docs/plans/completed/`
