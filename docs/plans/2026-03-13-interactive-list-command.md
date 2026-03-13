# Interactive /list command

## Overview
Modify the `/list` command to be interactive when no arguments are provided. Instead of showing an error message, the bot will present an inline keyboard with two options: "Periodic Chores" and "Regular Chores". Clicking either button will display the corresponding list by updating the original message.

## Context
- Files involved:
    - `internal/telegram/handlers/list_cancel.go`: Contains the `/list` command handler logic.
    - `internal/telegram/keyboard/keyboard.go`: Central location for inline keyboards.
    - `internal/telegram/bot.go`: Dispatches callback queries to handlers.
- Related patterns:
    - Inline keyboard callbacks for stateful/interactive flows.
    - Admin-only checks for management commands.

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add ListMenu to keyboard.go

**Files:**
- Modify: `internal/telegram/keyboard/keyboard.go`

- [ ] Add a new function `ListMenu()` that returns an inline keyboard with two buttons:
    - "📋 Periodic Chores" with callback data `list:chore`
    - "📝 Regular Chores" with callback data `list:task`
- [ ] ensure `keyboard.go` has necessary constants if applicable
- [ ] run project test suite - must pass before task 2

### Task 2: Update HandleList in list_cancel.go

**Files:**
- Modify: `internal/telegram/handlers/list_cancel.go`

- [ ] Update `HandleList` to check if `args` is empty
- [ ] If empty, return a `tgbotapi.MessageConfig` with the `ListMenu()` keyboard and the prompt: "Select which type of chores you want to list:"
- [ ] write tests for this task in `internal/telegram/handlers/recurring_chore_test.go` (or similar)
- [ ] run project test suite - must pass before task 3

### Task 3: Implement HandleListCallback in list_cancel.go

**Files:**
- Modify: `internal/telegram/handlers/list_cancel.go`

- [ ] Add `HandleListCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error)` to the `Handlers` struct
- [ ] Implement callback logic:
    - Perform admin check (consistent with other handlers)
    - Extract the type (`chore` or `task`) from `q.Data`
    - Reuse the logic from `HandleList` to generate the list content based on the type
    - Return a `tgbotapi.EditMessageTextConfig` containing the list content
- [ ] write tests for this task
- [ ] run project test suite - must pass before task 4

### Task 4: Register the callback in bot.go

**Files:**
- Modify: `internal/telegram/bot.go`

- [ ] Add a case for `list` in the `handleCallbackQuery` function to route to `b.handlers.HandleListCallback(q)`
- [ ] ensure callback routing is correct
- [ ] run project test suite - must pass before task 5

### Task 5: Verify acceptance criteria

- [ ] manual test: verify `/list` without arguments shows buttons
- [ ] manual test: verify clicking "Periodic Chores" shows the periodic chores list
- [ ] manual test: verify clicking "Regular Chores" shows the regular chores list
- [ ] manual test: verify admin-only restriction is maintained
- [ ] run full test suite (use project-specific command)
- [ ] run linter (use project-specific command)
- [ ] verify test coverage meets 80%+

### Task 6: Update documentation

- [ ] update README.md if user-facing changes (if `/list` interactivity is worth noting)
- [ ] move this plan to `docs/plans/completed/`
