# Sviniya Award Balance System

## Overview
Add a "sviniya" award system where the monthly rating winner automatically receives 1 sviniya. Users can view all balances via /sviniya and spend them via /spend with an LLM-generated announcement. An admin command /set_sviniya_balance handles bootstrapping existing balances (granting Crusader's March win).

## Context
- Files involved:
  - `internal/store/store.go` - add SviniyaBalance type and store interface methods
  - `internal/store/sqlite/sqlite.go` - add sviniya_balances table in migration
  - `internal/store/sqlite/sviniya.go` - new file for sviniya store implementation
  - `internal/store/mocks/mock_store.go` - add mock methods for new store interface
  - `internal/telegram/handlers/sviniya.go` - new file for /sviniya, /spend, /set_sviniya_balance handlers
  - `internal/telegram/handlers/sviniya_test.go` - tests
  - `internal/telegram/handlers/session.go` - add SessionTypeSpendSviniya
  - `internal/telegram/handlers/ratings.go` - hook sviniya grant into BuildMonthlyRatingsWinnersAnnouncement
  - `internal/telegram/bot.go` - register new commands and session routing
- Related patterns:
  - Interactive dialog via SessionManager (same as chore creation)
  - LLM RefineMessage for announcement text
  - Admin check via h.checkAdmin()
  - GroupID broadcast for announcements

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Database - sviniya_balances table and store interface

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite/sqlite.go`
- Create: `internal/store/sqlite/sviniya.go`

- [x] Add `SviniyaBalance` struct to `store.go` with fields: UserID int64, UserName string, Balance int
- [x] Add sviniya methods to `Store` interface: `GetAllSviniyaBalances`, `GetSviniyaBalance`, `AddSviniyaBalance`, `SetSviniyaBalance`, `DecrementSviniyaBalance`
- [x] Add `CREATE TABLE IF NOT EXISTS sviniya_balances` to `migrate()` in sqlite.go (user_id PK, balance INTEGER DEFAULT 0)
- [x] Implement all store methods in `sqlite/sviniya.go`
- [x] Write unit tests for store methods in `sqlite/sviniya_test.go`
- [x] run `go test ./internal/store/...` - must pass

### Task 2: Update mock store

**Files:**
- Modify: `internal/store/mocks/mock_store.go`

- [x] Add mock implementations for all 5 new sviniya store methods following existing mock patterns
- [x] run `go test ./...` - must pass (compile check)

### Task 3: /sviniya command and /set_sviniya_balance admin command

**Files:**
- Create: `internal/telegram/handlers/sviniya.go`
- Modify: `internal/telegram/bot.go`

- [x] Implement `HandleSviniya(m)` - fetches all balances, formats as list (name: N sviniya(s)), returns message to caller's chat
- [x] Implement `HandleSetSviniyaBalance(m)` - admin-only, parses `/set_sviniya_balance <name> <num>`, looks up user by name, calls SetSviniyaBalance, confirms
- [x] Register `"sviniya"` and `"set_sviniya_balance"` cases in `bot.go` handleCommand switch
- [x] Create `internal/telegram/handlers/sviniya_test.go` with tests for both handlers including zero-balance display and admin check
- [x] run `go test ./internal/telegram/...` - must pass

### Task 4: /spend interactive command

**Files:**
- Modify: `internal/telegram/handlers/session.go`
- Modify: `internal/telegram/handlers/sviniya.go`
- Modify: `internal/telegram/handlers/sviniya_test.go`
- Modify: `internal/telegram/bot.go`

- [x] Add `SessionTypeSpendSviniya SessionType = "spend_sviniya"` to session.go
- [x] Implement `HandleSpend(m)` in sviniya.go:
  - If arg provided: treat as description and process immediately
  - If no arg: check balance first (reply "sorry, no sviniyas on your balance" if zero), else start `SessionTypeSpendSviniya` session and prompt for description
- [x] Implement `HandleSpendInteractive(m)` for mid-session messages:
  - Handle /cancel → EndSession, reply cancelled
  - Otherwise treat text as description: decrement balance, build announcement via LLM RefineMessage (fallback to vanilla if nil), send to GroupID, confirm to user
- [x] Register `"spend"` in bot.go handleCommand
- [x] Add `SessionTypeSpendSviniya` routing in bot.go handleMessage switch
- [x] Add tests: zero balance path, cancel path, happy path with mock LLM, LLM unavailable fallback
- [x] run `go test ./internal/telegram/...` - must pass

### Task 5: Auto-grant sviniya on monthly rating winner announcement

**Files:**
- Modify: `internal/telegram/handlers/ratings.go`
- Modify: `internal/telegram/handlers/ratings_test.go`

- [x] In `BuildMonthlyRatingsWinnersAnnouncement`, after computing `totals`, grant 1 sviniya to `totals[0]` (first place) via `h.Store.AddSviniyaBalance`
- [x] Log error but don't fail the announcement if sviniya grant fails
- [x] Add test case: winner gets sviniya granted, store error is tolerated
- [x] run `go test ./internal/telegram/...` - must pass

### Task 6: Verify acceptance criteria

- [x] run `go test ./...`
- [x] run `go vet ./...`
- [x] Verify /sviniya shows balances, /spend works with and without inline desc, /set_sviniya_balance sets balance, winner auto-grants (manual test - skipped - not automatable)

### Task 7: Update documentation

- [x] update CLAUDE.md if new patterns introduced
- [x] move this plan to `docs/plans/completed/`
