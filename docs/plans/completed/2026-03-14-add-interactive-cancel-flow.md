---
# Add interactive flow for /cancel command with ID argument

## Overview
When a user types `/cancel 16` instead of the correct format (`/cancel chore 16` or `/cancel task 16`), the bot currently returns a text error message. This plan adds an interactive flow with buttons that guides the user to select the correct type of item to cancel.

## Context
- Files involved:
  - `internal/telegram/handlers/list_cancel.go` - Modify `HandleCancel` function
  - `internal/telegram/handlers/cancel_interactive_test.go` - Add new tests
- Related patterns: The `/list` command already shows an interactive menu when called without arguments (`ListMenu()`)
- Dependencies: None (uses existing tgbotapi patterns)

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- Follow existing patterns from `ListMenu()` for inline keyboard creation
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add HandleCancelIDSelection function

**Files:**
- Modify: `internal/telegram/handlers/list_cancel.go`

- [ ] Add `HandleCancelIDSelection(m *tgbotapi.Message, id string)` function
- [ ] This function creates an inline keyboard with buttons for:
  - "Cancel chore {id}" (callback: `cancel_assignment:R{id}`)
  - "Cancel task {id}" (callback: `cancel_assignment:A{id}`)
  - "Show all items" (callback to full interactive menu)
  - "Cancel operation" (callback: `cancel_flow`)
- [ ] Admin check is already done by caller
- [ ] Write tests for HandleCancelIDSelection

### Task 2: Modify HandleCancel to call interactive flow on invalid format

**Files:**
- Modify: `internal/telegram/handlers/list_cancel.go`

- [ ] Modify `HandleCancel` to detect when args is a single number (ID-like)
- [ ] When invalid format detected, check if args contains only digits (potential ID)
- [ ] If single number, call `HandleCancelIDSelection` with the ID
- [ ] If not a number, keep existing error message behavior
- [ ] Write tests for the new behavior:
  - Test with `/cancel 16` → shows interactive menu
  - Test with `/cancel abc` → shows error message
  - Test with `/cancel` → shows full interactive menu (existing)

### Task 3: Update tests for existing behavior

**Files:**
- Modify: `internal/telegram/handlers/cancel_interactive_test.go`

- [ ] Add `TestHandleCancelWithIDArgument` test
- [ ] Add `TestHandleCancelIDSelection` test
- [ ] Verify all existing tests still pass

### Task 4: Verify acceptance criteria

- [ ] Manual test: Type `/cancel 16` → see interactive buttons
- [ ] Manual test: Click "Cancel chore 16" → confirms cancellation
- [ ] Manual test: Click "Cancel task 16" → confirms cancellation
- [ ] Manual test: Type `/cancel` → shows full menu (existing behavior preserved)
- [ ] Run full test suite: `go test ./...`
- [ ] Verify test coverage meets 80%+

### Task 5: Update documentation

- [ ] No README changes needed (internal UX improvement)
- [ ] No CLAUDE.md changes needed (pattern follows existing conventions)
- [ ] Move this plan to `docs/plans/completed/`
