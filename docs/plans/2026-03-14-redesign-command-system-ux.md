---
# Redesign Bot Command System for Better UX

## Overview
Replace the error-prone two-word and inline-arg commands with single-word commands that are fully interactive and clickable from help messages. Remove all old commands with no aliases. Register all commands with Telegram's SetMyCommands for autocomplete.

## Problem Analysis
Current pain points:
- `/chore list` creates a chore named "list" (the core bug)
- `/edit chore`, `/chore translate` are two-word, not clickable in Telegram
- `/chore_stats`, `/toggle_active` have underscores, inconsistent
- `/modify` duplicates `/change`
- `/list` accepts "chore" as subcommand arg
- `/assign john 3`, `/unassign john 3` work inline but are error-prone
- No SetMyCommands - no Telegram autocomplete

## New Command Map
User commands (unchanged): /start, /help, /status, /schedule, /volunteer, /explain

Admin commands (new names, old ones deleted):
- /chores - chore management menu for admin (was no-arg /chore admin path), personal chore list for users (was non-admin /chore)
- /newchore - interactively create a chore (was /chore desc for admin, now always starts a session)
- /editchore - interactively edit a chore description (was /edit chore)
- /translate - interactively pick a chore to translate (was /chore translate id)
- /stats - show chore statistics (was /chore_stats)
- /activate - toggle user active status (was /toggle_active)
- /assign, /unassign, /cancel, /change, /offduty, /vacation, /users, /ratings, /complete, /overdue - kept, inline arg paths removed

Removed without replacement: /list (merged into /chores), /modify (duplicate of /change), /edit, /chore, /chore_stats, /toggle_active

## Context
- Files involved:
  - `internal/telegram/bot.go` - command dispatch switch + SetMyCommands
  - `internal/telegram/handlers/chore.go` - split HandleChore; remove inline arg creation path
  - `internal/telegram/handlers/chore_interactive.go` - HandleChoreActionCallback (chore creation session already exists here)
  - `internal/telegram/handlers/list_cancel.go` - HandleList removed; HandleEdit refactored to HandleEditChore
  - `internal/telegram/handlers/commands.go` - help text rewrite
  - `internal/telegram/handlers/admin.go` - rename HandleToggleActive; strip inline arg paths from HandleAssign/HandleUnassign/HandleChange
- Related patterns: session-based interactive flows use SessionManager.StartSession + text handler reading m.Text; inline keyboards use callback data
- Dependencies: none new

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Register SetMyCommands at bot startup

**Files:**
- Modify: `internal/telegram/bot.go`

- [ ] In `NewBot()`, after bot API is initialized, call `api.Request(tgbotapi.NewSetMyCommands(...))` with user command list: start, help, status, schedule, volunteer, explain, chores
- [ ] Call `api.Request(tgbotapi.NewSetMyCommandsWithScopeAndLanguage(...))` scoped to admin user ID with full admin list: chores, newchore, editchore, translate, stats, activate, assign, unassign, cancel, change, offduty, vacation, users, ratings, complete, overdue
- [ ] Log success/failure (non-fatal)
- [ ] Write test verifying NewBot initializes without crashing (mock API exists)
- [ ] Run test suite - must pass before task 2

### Task 2: Split /chore → /chores and /newchore; remove /list

**Files:**
- Modify: `internal/telegram/handlers/chore.go`
- Modify: `internal/telegram/bot.go`

- [ ] In `HandleChore`: extract non-admin path (personal chore list) into `HandleChores` for non-admins; extract admin no-arg path (shows ChoreMenu) into `HandleChores` for admins; delete inline arg creation path and inline "translate" subcommand path from this function
- [ ] Add `HandleNewChore`: always starts a chore creation session (calls SessionManager.StartSession with SessionTypeChoreCreation, prompts for description) — never accepts inline args
- [ ] In `bot.go` switch: replace `case "chore"` with `case "chores"` → HandleChores and `case "newchore"` → HandleNewChore; delete `case "list"` entirely
- [ ] Update tests in `chore_test.go` and `chore_interactive_test.go` to use new function names and verify no inline-arg path exists
- [ ] Run test suite - must pass before task 3

### Task 3: Add /translate command (interactive chore selection)

**Files:**
- Modify: `internal/telegram/handlers/chore.go`
- Modify: `internal/telegram/bot.go`

- [ ] Add `HandleTranslate`: fetches all chores, renders inline keyboard for selection (chore ID → button), stores pending translate action in session or callback data; on button press calls existing translate logic
- [ ] Add callback handler for translate selection (e.g. `case "translate_chore"` in callback switch), calls existing translation logic
- [ ] In `bot.go` switch: add `case "translate"` → HandleTranslate
- [ ] Write tests for HandleTranslate and callback
- [ ] Run test suite - must pass before task 4

### Task 4: Add /editchore command; rename /stats and /activate

**Files:**
- Modify: `internal/telegram/handlers/list_cancel.go` (or edit_interactive.go)
- Modify: `internal/telegram/handlers/admin.go`
- Modify: `internal/telegram/handlers/commands.go`
- Modify: `internal/telegram/bot.go`

- [ ] Add `HandleEditChore`: fetches chores, shows inline keyboard to select which to edit; on selection starts edit session (reuse existing edit session logic from edit_interactive.go); remove all inline arg paths from old HandleEdit
- [ ] Rename `HandleToggleActive` → `HandleActivate` (or add HandleActivate that calls the same logic)
- [ ] Rename `HandleChoreStats` → `HandleStats`
- [ ] In `bot.go` switch: replace `case "edit"` with `case "editchore"` → HandleEditChore; replace `case "chore_stats"` with `case "stats"` → HandleStats; replace `case "toggle_active", "toggleactive"` with `case "activate"` → HandleActivate; delete `case "modify"` (remove HandleModify call)
- [ ] Write tests for HandleEditChore
- [ ] Run test suite - must pass before task 5

### Task 5: Strip inline arg paths from /assign, /unassign, /change

**Files:**
- Modify: `internal/telegram/handlers/admin.go`

- [ ] In `HandleAssign`: delete the `len(args) == 1` and `len(args) >= 2` branches; keep only the no-args interactive keyboard path
- [ ] In `HandleUnassign`: same — delete inline arg branches; keep only interactive keyboard path
- [ ] In `HandleChange` (or HandleModify if that's the name): delete inline arg branches if any; keep only interactive date-picker flow
- [ ] Update related tests to remove test cases for inline arg paths
- [ ] Run test suite - must pass before task 6

### Task 6: Update help text

**Files:**
- Modify: `internal/telegram/handlers/commands.go`

- [ ] Rewrite `adminHelpMessageSection` with new command names only: /chores, /newchore, /editchore, /translate, /stats, /activate, /assign, /unassign, /cancel, /change, /offduty, /vacation, /users, /ratings, /complete, /overdue
- [ ] Group commands: Chore management | Duty management | User management
- [ ] Update user help with /chores (was /chore)
- [ ] Ensure all command names in help are clickable (single-word, prefixed with /)
- [ ] Write test verifying help contains new names and does NOT contain old names (/list, /edit, /chore_stats, /toggle_active, /modify, /chore)
- [ ] Run test suite - must pass before task 7

### Task 7: Verify acceptance criteria

- [ ] Manual test: type /chores as user — shows personal active chores
- [ ] Manual test: type /chores as admin — shows chore management menu
- [ ] Manual test: type /newchore — bot asks for description interactively, no inline args accepted
- [ ] Manual test: type /chore — "unknown command" (old command removed)
- [ ] Manual test: /help shows all commands as clickable single-word links
- [ ] Manual test: Telegram autocomplete appears when typing /
- [ ] Manual test: /assign with no args shows user selection buttons (inline args ignored)
- [ ] Run full test suite: `go test ./...`
- [ ] Run linter: `go vet ./...`
- [ ] Verify test coverage meets 80%+

### Task 8: Update documentation

- [ ] Update README.md command table if it exists
- [ ] Move this plan to `docs/plans/completed/`
