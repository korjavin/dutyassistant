---
# Monthly Participant Rating Contest

## Overview
Implement a monthly participant rating flow in the Telegram bot: every day at 20:50 Europe/Berlin the bot asks the admin to rate all active participants from 1 to 5 in a fixed order, the admin replies with space-separated numbers, the bot stores one daily score per participant, and an admin command shows the current month as a score calendar. On the last calendar day of each month at 21:00 Europe/Berlin, the bot publishes the monthly winners and total points, then the next month starts from a clean slate because all queries are month-bounded.

## Context
- Files involved: `cmd/roster-bot/main.go`, `internal/store/store.go`, `internal/store/sqlite/sqlite.go`, `internal/store/sqlite/sqlite_test.go`, `internal/store/sqlite/ratings.go`, `internal/telegram/bot.go`, `internal/telegram/handlers/handlers.go`, `internal/telegram/handlers/session.go`, `internal/telegram/handlers/commands.go`, `internal/telegram/handlers/ratings.go`, `internal/telegram/handlers/ratings_test.go`, `internal/mocks/store.go`, `internal/store/mocks/store.go`, `README.md`
- Related patterns: cron jobs are registered in `cmd/roster-bot/main.go` with Europe/Berlin timezone; non-command message handling already exists for interactive flows via `SessionManager`; admin-only checks are centralized in the Telegram handlers; SQLite migrations are inline in `internal/store/sqlite/sqlite.go`, with feature methods split into dedicated files.
- Dependencies: no new external dependencies are required; reuse existing `ADMIN_ID` and `DISH_GROUP` configuration.

## Development Approach
- Testing approach: Regular (code first, then tests)
- Complete each task fully before moving to the next
- Keep the implementation date-bounded by month instead of adding a destructive monthly reset job
- Preserve existing Telegram command and handler conventions instead of introducing a new interaction framework
- CRITICAL: every task MUST include new/updated tests
- CRITICAL: all tests must pass before starting next task

## Implementation Steps

### Task 1: Add rating domain model and persistence

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite/sqlite.go`
- Create: `internal/store/sqlite/ratings.go`
- Modify: `internal/store/sqlite/sqlite_test.go`
- Modify: `internal/mocks/store.go`
- Modify: `internal/store/mocks/store.go`

- [ ] Add store types for daily participant rating rows and monthly aggregate rows
- [ ] Extend the `Store` interface with methods to save one day of ratings, fetch the ordered participant list for rating, fetch current-month ratings, and compute monthly totals/ranking
- [ ] Add a new SQLite table for daily participant ratings with uniqueness on participant plus date
- [ ] Implement SQLite queries for fixed participant ordering, insert/update of one full daily rating submission, month-to-date calendar data, and monthly totals with deterministic tie ordering
- [ ] Write store tests covering create/update semantics, stable ordering, month filtering, and aggregate ranking
- [ ] Run store tests and ensure they pass before task 2

### Task 2: Add admin rating session and score submission parsing

**Files:**
- Modify: `internal/telegram/handlers/session.go`
- Modify: `internal/telegram/bot.go`
- Modify: `internal/telegram/handlers/handlers.go`
- Create: `internal/telegram/handlers/ratings.go`
- Modify: `internal/telegram/handlers/ratings_test.go`
- Modify: `internal/mocks/store.go`
- Modify: `internal/store/mocks/store.go`

- [ ] Add a dedicated session type for waiting for daily ratings from the admin
- [ ] Implement a helper that builds the rating prompt from the active participant list in a stable order
- [ ] Implement an admin-only flow that accepts a plain-text reply with space-separated integers and validates score count, score range 1..5, and same-day correction by resubmission
- [ ] Wire non-command message routing so only the admin reply in an active rating session is consumed
- [ ] Return clear validation errors and echo the participant order back on failure
- [ ] Write handler tests for valid submission, invalid count, invalid range, unauthorized sender, and overwrite/correction
- [ ] Run Telegram handler tests and ensure they pass before task 3

### Task 3: Add admin command to show the current-month score calendar

**Files:**
- Modify: `internal/telegram/bot.go`
- Modify: `internal/telegram/handlers/commands.go`
- Modify: `internal/telegram/handlers/ratings.go`
- Modify: `internal/telegram/handlers/ratings_test.go`

- [ ] Add a new admin command to display month-to-date ratings in a readable text table/calendar
- [ ] Render dates from the 1st of the current month through today, with one row per day and participant scores aligned to the same stable participant order
- [ ] Show missing days or missing scores explicitly so the admin can see gaps
- [ ] Update `/help` text to document the new rating commands
- [ ] Write handler tests for populated calendar output, empty-month output, and admin access control
- [ ] Run Telegram handler tests and ensure they pass before task 4

### Task 4: Add scheduled daily reminder and month-end winner announcement

**Files:**
- Modify: `cmd/roster-bot/main.go`
- Modify: `internal/telegram/handlers/ratings.go`
- Modify: `internal/telegram/handlers/ratings_test.go`

- [ ] Add a cron job at 20:50 Europe/Berlin to send the daily rating prompt to the configured admin
- [ ] Ensure the prompt skips when there are no active non-admin participants
- [ ] Add a cron job for 21:00 on the last calendar day of the month to publish monthly totals and 1st, 2nd, and 3rd places to the group chat
- [ ] Use last-day-of-month logic rather than literal day 31 so the feature works in every month
- [ ] Keep the monthly reset implicit by querying and storing by date; do not add a destructive reset job
- [ ] Write tests for reminder text generation, winner digest formatting, and last-day-of-month announcement logic
- [ ] Run affected tests and ensure they pass before task 5

### Task 5: Verify acceptance criteria

**Files:**
- Modify: `README.md`

- [ ] Manual test: the cron-style prompt lists participants in stable order and admin reply `5 2 1` stores ratings for that day
- [ ] Manual test: resubmitting the same day replaces that day’s scores rather than duplicating them
- [ ] Manual test: the month-to-date calendar shows all days from the 1st through today
- [ ] Manual test: the month-end message shows totals for each participant and the top three winners
- [ ] Run the full Go test suite and ensure it passes
- [ ] Run the project linter if a standard lint command exists
- [ ] Verify new code paths have meaningful automated coverage and meet the project target

### Task 6: Update documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/2026-03-13-monthly-participant-rating-contest.md`

- [ ] Update `README.md` with the new admin commands, daily reminder behavior, and month-end announcement behavior if they are user-facing
- [ ] Note any internal conventions only if the implementation introduces a durable new pattern that future contributors must follow
- [ ] Move this plan to `docs/plans/completed/` after implementation is finished
---
