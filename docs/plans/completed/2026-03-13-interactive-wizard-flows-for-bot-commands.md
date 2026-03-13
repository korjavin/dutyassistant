# Interactive Wizard Flows for Bot Commands

## Overview
Add inline-keyboard-driven interactive flows for commands that currently require typed subcommands or IDs. When a command is invoked without arguments (e.g. from /help menu or Telegram autosuggestions), the bot presents contextual buttons to guide the user through the operation. All flows include a cancel button. No external FSM library is needed - the existing session manager and callback routing infrastructure already cover the requirements.

Note: /assign, /modify, /offduty, /toggle_active, and /vacation already have interactive flows. This work targets the remaining commands: /chore, /cancel, /unassign, /complete.

## Context
- Files involved:
  - `internal/telegram/bot.go` - callback router (handleCallbackQuery)
  - `internal/telegram/handlers/chore.go` - chore command and session
  - `internal/telegram/handlers/list_cancel.go` - /list and /cancel handlers
  - `internal/telegram/handlers/admin.go` - /unassign, /complete handlers
  - `internal/telegram/handlers/session.go` - session manager
  - `internal/telegram/keyboard/keyboard.go` - keyboard builders
  - `internal/telegram/handlers/*_test.go` - test files
- Related patterns: existing assign interactive flow (assign_user / assign_days callbacks), existing chore creation session, EditMessageText pattern for callback responses
- Dependencies: none new

## Architecture
For most interactive flows, state is encoded in callback data (e.g. `chore_delete_confirm:ID`), avoiding the need for sessions. Sessions are only used when text input is required (already the case for chore creation). A universal `cancel_flow` callback edits the message to show "Cancelled" and terminates the flow.

Existing text-based argument passing is preserved; interactive flow activates only when no arguments are provided.

## Development Approach
- **Testing approach**: Regular (implement then test)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Cancel Flow Infrastructure

**Files:**
- Modify: `internal/telegram/bot.go`
- Create: `internal/telegram/handlers/flow.go`

- [ ] Add `HandleCancelFlow(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error)` that edits the message to "❌ Operation cancelled" with no keyboard
- [ ] Register `cancel_flow` callback prefix in `handleCallbackQuery()` in bot.go
- [ ] Write tests for cancel_flow callback
- [ ] Run project test suite - must pass before task 2

### Task 2: /chore Interactive Flow

**Files:**
- Modify: `internal/telegram/handlers/chore.go`
- Modify: `internal/telegram/keyboard/keyboard.go`
- Modify: `internal/telegram/bot.go`
- Modify: `internal/telegram/handlers/chore_test.go`

- [ ] In `HandleChore`, when no args: return message with inline keyboard [📋 List Chores] [➕ Create Chore] [🗑 Delete Chore] [❌ Cancel]
- [ ] Add `chore_action:list` callback - runs list logic and edits message with chore list
- [ ] Add `chore_action:create` callback - starts existing chore creation session
- [ ] Add `chore_action:delete` callback - edits message to show chore list as buttons (each encoded as `chore_delete:ID`) plus cancel
- [ ] Add `chore_delete:ID` callback - edits message to confirm dialog: "Delete chore X?" [✅ Confirm] [❌ Cancel]
- [ ] Add `chore_delete_confirm:ID` callback - deletes the chore, edits message to "✅ Chore deleted"
- [ ] Register new callback prefixes in bot.go
- [ ] Write tests for all new callbacks and updated HandleChore
- [ ] Run project test suite - must pass before task 3

### Task 3: /cancel Interactive Flow (admin)

**Files:**
- Modify: `internal/telegram/handlers/list_cancel.go`
- Modify: `internal/telegram/bot.go`
- Create or modify: `internal/telegram/handlers/list_cancel_test.go`

- [ ] In `HandleCancel`, when no args: query upcoming/active assignments, return inline keyboard with each as a button (encoded as `cancel_assignment:ID`) plus [❌ Cancel]
- [ ] Add `cancel_assignment:ID` callback - edits message to confirm: "Cancel assignment for user X on date Y?" [✅ Confirm] [❌ Cancel]
- [ ] Add `cancel_assignment_confirm:ID` callback - executes cancellation, edits message to "✅ Assignment cancelled"
- [ ] Register new callback prefixes in bot.go
- [ ] Write tests
- [ ] Run project test suite - must pass before task 4

### Task 4: /unassign Interactive Flow (admin)

**Files:**
- Modify: `internal/telegram/handlers/admin.go`
- Modify: `internal/telegram/bot.go`
- Modify: `internal/telegram/handlers/admin_test.go`

- [ ] In `HandleUnassign`, when no args: return user selection keyboard (reuse existing user-list keyboard pattern from assign flow) with [❌ Cancel]
- [ ] Add `unassign_user:ID` callback - edits message to confirm: "Unassign user X?" [✅ Confirm] [❌ Cancel]
- [ ] Add `unassign_confirm:ID` callback - executes unassign, edits message to "✅ Unassigned"
- [ ] Register new callback prefixes in bot.go
- [ ] Write tests
- [ ] Run project test suite - must pass before task 5

### Task 5: /complete Interactive Flow (admin)

**Files:**
- Modify: `internal/telegram/handlers/admin.go`
- Modify: `internal/telegram/bot.go`
- Modify: `internal/telegram/handlers/admin_test.go`

- [ ] Inspect `HandleComplete` to understand what entity/ID it requires
- [ ] When no args: query pending items, return as inline keyboard buttons (encoded as `complete_item:ID`) plus [❌ Cancel]
- [ ] Add `complete_item:ID` callback - executes completion or shows confirmation step
- [ ] Register new callback prefixes in bot.go
- [ ] Write tests
- [ ] Run project test suite - must pass before task 6

### Task 6: Verify Acceptance Criteria

- [ ] Manual test: trigger /chore, /cancel, /unassign, /complete without args - all show button menus
- [ ] Manual test: cancel button in each flow shows "Cancelled" message
- [ ] Manual test: existing text-based argument passing still works (e.g. `/chore list`)
- [ ] Run full test suite: `go test ./...`
- [ ] Run linter: `go vet ./...`
- [ ] Verify test coverage meets 80%+

### Task 7: Update Documentation

- [ ] Update README.md if user-facing command descriptions changed
- [ ] Move this plan to `docs/plans/completed/`
