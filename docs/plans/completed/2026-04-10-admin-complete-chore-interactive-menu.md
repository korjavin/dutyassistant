# Add "Complete Chore" to Admin /chore Interactive Menu

## Overview

Add a "Complete" button to the /chore interactive menu so admins can mark chores as done on behalf of participants directly from the /chore menu, without needing the separate /complete command.

## Context

- Files involved:
  - `internal/telegram/keyboard/keyboard.go` - ChoreMenu() definition
  - `internal/telegram/handlers/chore_interactive.go` - Menu action callbacks
  - `internal/telegram/bot.go` - Callback routing
  - `internal/telegram/handlers/admin.go` - Existing /complete logic (HandleComplete, HandleCompleteChoreCallback)
- Related patterns: The delete action in chore_interactive.go follows a similar pattern (show list with buttons -> handle selection). The /complete command already has the completion logic we can reuse.

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add "Complete" button to ChoreMenu keyboard

**Files:**
- Modify: `internal/telegram/keyboard/keyboard.go`

- [x] Add a new row to ChoreMenu() with button text "✅ Complete Chore" and callback data "chore_action:complete", placed between "Delete" and "Cancel"
- [x] Write test verifying ChoreMenu() returns the expected buttons including the new one
- [x] Run project test suite - must pass before task 2

### Task 2: Add complete action handler in chore_interactive.go

**Files:**
- Modify: `internal/telegram/handlers/chore_interactive.go`
- Modify: `internal/telegram/bot.go`

- [x] Add "complete" case to the switch in HandleChoreActionCallback() that calls a new handleChoreCompleteInteractive() function
- [x] Implement handleChoreCompleteInteractive() - fetch active chores via Store.GetActiveChores(), display them as inline buttons with callback data "chore_complete_interactive:{reminderID}" (similar to how handleChoreDeleteInteractive works)
- [x] Add callback routing in bot.go for "chore_complete_interactive" prefix, routing to a new HandleChoreCompleteInteractiveCallback() handler
- [x] Implement HandleChoreCompleteInteractiveCallback() - extract reminderID, verify admin, complete the chore (reuse logic from HandleCompleteChoreCallback: mark complete in DB, remove from reminder manager, send group notification), edit the message to confirm completion
- [x] Write tests for the new handlers: test that the complete action shows active chores, test that selecting a chore completes it, test that non-admins are rejected
- [x] Run project test suite - must pass before task 3

### Task 3: Verify acceptance criteria

- [x] Run full test suite: `go test ./...`
- [x] Run linter: `go vet ./...`

### Task 4: Update documentation

- [x] Update CLAUDE.md if internal patterns changed (no new patterns - existing patterns reused)
- [x] Move this plan to `docs/plans/completed/`
