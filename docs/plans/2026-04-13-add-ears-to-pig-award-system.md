# Add Ears to Pig Award Rating System

## Overview

Add an "ear" modifier to the rating system. When a participant's performance exceeds what a max score of 5 can express, the admin can award "5e" (5 with ear). The score remains 5 for calculation purposes, but ears are tracked and displayed separately - shown as "5e" in calendars and daily summaries, and counted per participant in the monthly congratulations.

## Context

- Files involved:
  - `internal/store/store.go` - ParticipantDailyRating and ParticipantMonthlyTotal structs
  - `internal/store/sqlite/ratings.go` - DB queries for saving/loading ratings
  - `internal/store/sqlite/sqlite.go` - Schema migration
  - `internal/telegram/handlers/ratings.go` - Score parsing, calendar display, daily/monthly summaries
  - Test files for the above
- Related patterns: Existing rating flow (admin sends space-separated scores, parsed by `parseParticipantScores`)
- Dependencies: None

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add ear field to store types and database schema

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite/sqlite.go`
- Modify: `internal/store/sqlite/ratings.go`

- [x] Add `HasEar bool` field to `ParticipantDailyRating` struct in `store.go`
- [x] Add `EarCount int` field to `ParticipantMonthlyTotal` struct in `store.go`
- [x] Add migration in `sqlite.go` to add `has_ear INTEGER NOT NULL DEFAULT 0` column to `participant_ratings` table
- [x] Update `SaveDailyParticipantRatings` to persist the `HasEar` field
- [x] Update `scanParticipantDailyRating` to scan the `has_ear` column
- [x] Update `GetMonthlyParticipantTotals` query to include `SUM(pr.has_ear) AS ear_count`
- [x] Update `getParticipantRatingsBetween` query to select `has_ear`
- [x] Write tests for saving and loading ratings with ears
- [x] Run project test suite - must pass before task 2

### Task 2: Update score parsing to accept "5e" input

**Files:**
- Modify: `internal/telegram/handlers/ratings.go`

- [x] Create a `parsedScore` struct with `Score int` and `HasEar bool` fields (local to ratings.go)
- [x] Update `parseParticipantScores` to return `[]parsedScore` instead of `[]int`; accept "5e" as valid input (only "5e", not other scores with "e")
- [x] Update `HandleDailyRatingsInteractive` to map parsed scores (including HasEar) into `ParticipantDailyRating` structs
- [x] Update `buildDailyRatingsPrompt` example text to mention "5e" option (e.g., "Use 5e for an ear award")
- [x] Write tests for parsing "5e", rejecting "3e", mixed input like "5 5e 4 5"
- [x] Run project test suite - must pass before task 3

### Task 3: Update display formatting to show ears

**Files:**
- Modify: `internal/telegram/handlers/ratings.go`

- [x] Update `buildRatingsCalendarTable` to display "5e" for scores with ears (adjust column width if needed)
- [x] Update `formatDailyAndMonthlySummary` to show "5e" for daily ratings with ears, and ear count in monthly standings (e.g., "Alice - 47 point(s), 3 ear(s)")
- [x] Update `formatMonthlyRatingsWinnersDigest` to include ear counts in both winners and totals sections when ear count > 0
- [x] Write tests verifying ear display in calendar, daily summary, and monthly digest
- [x] Run project test suite - must pass before task 4

### Task 4: Verify acceptance criteria

- [x] Run full test suite (`go test ./...`)
- [x] Run linter (`go vet ./...`)
- [x] Verify: admin can submit "5 5e 4 5" and it saves correctly
- [x] Verify: calendar shows "5e" for ear-awarded scores
- [x] Verify: monthly digest shows ear counts per participant

### Task 5: Update documentation

- [ ] Update CLAUDE.md if internal patterns changed
- [ ] Move this plan to `docs/plans/completed/`
