# Fix /help command user detection

## Overview
The `/help` command currently displays all commands, including administrative ones, to every user. This task will refactor the `HandleHelp` handler to detect the user's admin status and filter the displayed commands accordingly, ensuring a cleaner interface for regular users and better security through obscurity.

## Context
- Files involved:
    - `internal/telegram/handlers/commands.go`: Contains the help message constants and the `HandleHelp` function.
    - `internal/telegram/handlers/commands_test.go`: Contains tests for the help command.
- Related patterns:
    - Use `h.checkAdmin(telegramUserID int64)` from `internal/telegram/handlers/admin.go` to determine permissions.
    - The bot uses `tgbotapi.ModeMarkdown` for the help message.

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Refactor Help Message Constants

**Files:**
- Modify: `internal/telegram/handlers/commands.go`

- [ ] Split the existing `helpMessage` constant into `userHelpMessage` and `adminHelpMessageSection`.
- [ ] `userHelpMessage` should contain: `/start`, `/help`, `/status`, `/schedule`, `/volunteer`, `/explain`, and `/chore` (user view).
- [ ] `adminHelpMessageSection` should contain the `*Admin Commands:*` header and all admin-specific commands.
- [ ] run project test suite - must pass before task 2

### Task 2: Implement Admin Detection in HandleHelp

**Files:**
- Modify: `internal/telegram/handlers/commands.go`

- [ ] Update `HandleHelp` to call `h.checkAdmin(m.From.ID)`.
- [ ] Handle cases where `m.From` might be `nil` (default to non-admin).
- [ ] Concatenate `userHelpMessage` with `adminHelpMessageSection` only if the user is an admin.
- [ ] Maintain `tgbotapi.ModeMarkdown` for formatting.
- [ ] run project test suite - must pass before task 3

### Task 3: Add and Update Tests

**Files:**
- Modify: `internal/telegram/handlers/commands_test.go`

- [ ] Update `TestHandleHelp` to verify that regular users (non-admins) do not see admin commands.
- [ ] Add `TestHandleHelp_Admin` to verify that admins see the full command list.
- [ ] Use mock store and `NewWithAdminID` to simulate different user roles.
- [ ] run project test suite - must pass before task 4

### Task 4: Verify Acceptance Criteria

- [ ] manual test: Send `/help` as a regular user and verify the "Admin Commands" section is hidden.
- [ ] manual test: Send `/help` as the configured admin and verify the full help message appears.
- [ ] run full test suite (go test ./...)
- [ ] run linter (go vet ./...)

### Task 5: Update Documentation

- [ ] update README.md if user-facing changes (if help command behavior change needs documentation)
- [ ] move this plan to `docs/plans/completed/2026-03-14-fix-help-command-user-detection.md`
