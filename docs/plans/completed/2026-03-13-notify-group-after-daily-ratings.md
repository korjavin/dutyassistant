# Notify group after daily ratings are set

## Overview
Implement an automatic notification to the group chat whenever the admin submits daily participant ratings. The message will include today's scores and the current month's aggregate standings (total points), providing immediate feedback and transparency to all participants.

## Context
- Files involved:
  - `internal/telegram/handlers/ratings.go`: Implementation of message formatting and notification trigger.
  - `internal/telegram/handlers/ratings_test.go`: Tests for the new group notification flow.
- Related patterns:
  - `Handlers` already has access to `h.Bot` (tgbotapi.BotAPI) and `h.GroupID`.
  - Monthly totals are already computed via `h.Store.GetMonthlyParticipantTotals`.
  - Similar pattern to `BuildMonthlyRatingsWinnersAnnouncement` but triggered interactively.

## Development Approach
- Testing approach: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Implement group notification formatting

**Files:**
- Modify: `internal/telegram/handlers/ratings.go`

- [ ] Create `formatDailyAndMonthlySummary` helper function in `ratings.go`.
- [ ] Format "Today's Ratings" section with names and scores from the current submission.
- [ ] Format "Monthly Standings" section with the ranked total points for the current month.
- [ ] Use `h.Store.GetMonthlyParticipantTotals` to fetch current rankings.
- [ ] Ensure formatting is clean and readable (using simple text).
- [ ] Add unit tests for the formatting function in `ratings_test.go`.

### Task 2: Trigger group notification in HandleDailyRatingsInteractive

**Files:**
- Modify: `internal/telegram/handlers/ratings.go`

- [ ] In `HandleDailyRatingsInteractive`, after successful call to `SaveDailyParticipantRatings`:
- [ ] Call `h.Store.GetMonthlyParticipantTotals` for the current month.
- [ ] If `h.Bot` is not nil and `h.GroupID` is set, format the notification message using the new helper.
- [ ] Send the message to `h.GroupID` using `h.Bot.Send`.
- [ ] Ensure any errors in sending the group message are logged but do not block the admin's success response.

### Task 3: Update and add tests for the new notification flow

**Files:**
- Modify: `internal/telegram/handlers/ratings_test.go`

- [ ] Update `TestHandleDailyRatingsInteractive_ValidSubmission` to verify that a message is sent to the group chat (using a mock bot if possible, or verifying that no panic occurs if `h.Bot` is set).
- [ ] Add a dedicated test case `TestHandleDailyRatingsInteractive_SendsGroupNotification` where `h.Bot` is mocked to expect a `Send` call to the group ID.
- [ ] Ensure all existing tests in `internal/telegram/handlers/ratings_test.go` still pass.

### Task 4: Verify acceptance criteria

- [ ] manual test: as admin, submit daily ratings and verify the group receives a summary with daily scores and monthly totals.
- [ ] run full test suite: `go test ./...`
- [ ] run linter (if available, e.g., `golangci-lint run`)
- [ ] verify test coverage for new logic

### Task 5: Update documentation

- [ ] update `CHANGES.md` to reflect the new automatic group notification for ratings.
- [ ] move this plan to `docs/plans/completed/`
