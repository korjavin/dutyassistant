# Register Bot Commands with Per-Scope Autocomplete

## Overview

Today the bot does not call Telegram's `setMyCommands` API, so users have no autocomplete suggestions when typing `/` in chat with the bot. This plan adds a single registration step at bot startup that publishes three command lists, one per Telegram scope:

- **All private chats** (regular users in DM)
- **Admin's private chat** (the `ADMIN_ID` user — sees user commands + admin commands)
- **The main DISH_GROUP chat** (read-only / informational subset to keep group autocomplete clean)

Benefits:

- Users discover commands via Telegram's built-in autocomplete UI.
- Admins get admin commands surfaced only in their own DM with the bot.
- Group chat autocomplete is narrow, so the group is not noisy with action commands like `/volunteer` or `/spend` that belong in DM.
- Aligns with `docs/plans/2026-03-14-redesign-command-system-ux.md` (which earmarks `setMyCommands` as the discovery surface for the redesigned UX).

This is a pure additive Telegram-side change. It does not modify command dispatch logic or any handler — only what Telegram suggests in autocomplete.

## Context (from discovery)

**Files involved:**

- `internal/telegram/bot.go:23-40` — `NewBot()` is where `tgbotapi.NewBotAPI` is called; new logic plugs in immediately after authorization log on line 29, before `return`.
- `internal/telegram/commands.go` *(new)* — module holding the curated command lists and the registration helper, keeping `bot.go` lean.
- `internal/telegram/commands_test.go` *(new)* — unit tests using the existing `NewTestClient` HTTP mock pattern (see CLAUDE.md "Mocking Telegram API").
- `internal/telegram/bot.go:193-250` — `handleCommand()` switch (the canonical list of commands; we mirror this when defining what to register).
- `internal/telegram/handlers/commands.go:23-45` — current English `userHelpMessage` / `adminHelpMessageSection` strings (source of descriptions and language convention).
- `cmd/roster-bot/main.go:82-89` — where `NewBot(token, handlers, groupID, adminID)` is called; no change needed since both IDs are already passed.

**Related patterns found:**

- All user-facing strings in the codebase are English (one tiny Russian fallback in `/explain`). English descriptions match.
- `tgbotapi` v5 supports `tgbotapi.NewSetMyCommandsWithScope` and `tgbotapi.NewSetMyCommandsWithScopeAndLanguage`. The library exposes `BotCommandScopeAllPrivateChats`, `BotCommandScopeChat`, etc.
- Existing tests use `NewTestClient` to inspect Telegram HTTP payloads — same approach captures `setMyCommands` requests by URL substring match.
- Non-fatal Telegram errors are logged with `slog` and execution continues (e.g., `checkAccess` returns false on error rather than panicking). We follow the same posture.

**Dependencies identified:**

- `github.com/go-telegram-bot-api/telegram-bot-api/v5` (already in go.mod).
- No DB, no scheduler, no LLM, no config changes.

**Scope decisions (locked in via planning questions):**

| Scope | Telegram scope object | Commands |
|---|---|---|
| User DMs | `BotCommandScopeAllPrivateChats` | start, help, status, schedule, volunteer, explain, chore, takechore, sviniya, spend |
| Admin DM | `BotCommandScopeChat(adminID)` | *all user commands* + assign, unassign, modify, cancel, edit, offduty, vacation, users, toggle_active, ratings, chore_stats, list, complete, overdue, set_sviniya_balance |
| Group chat | `BotCommandScopeChat(groupID)` | help, status, schedule, ratings, sviniya, chore_stats, overdue |

**Why admin DM includes user commands**: Telegram's scope precedence means `BotCommandScopeChat` *replaces* (does not merge with) `BotCommandScopeAllPrivateChats` for that user. If admin's chat scope contains only admin commands, the admin loses autocomplete for `/schedule`, `/volunteer`, etc. We therefore register **the union** in the admin chat scope.

**Failure handling**: `setMyCommands` errors are logged via `slog.Warn` and startup continues. Commands in autocomplete are nice-to-have UX, not a startup precondition.

**Guards**: Skip the admin scope call if `adminID == 0`. Skip the group scope call if `groupID == 0`. Both are valid configurations (e.g., local dev without an admin/group set).

## Development Approach

- **Testing approach**: Regular (code first, then tests using the `NewTestClient` HTTP mock from CLAUDE.md)
- Complete each task fully before moving to the next
- Make small, focused changes
- **Every task includes new/updated tests** for code changes in that task — success and error scenarios
- All tests must pass before starting the next task
- Run tests after each change
- Maintain backward compatibility (no behavior change for users on Telegram clients that ignore `setMyCommands`)
- Update this plan file if scope shifts during implementation

## Testing Strategy

- **Unit tests** (required per task): mock `bot.api.Request(...)` via `NewTestClient` to intercept POSTs to `setMyCommands`. Decode the form body, assert:
  - The `commands` JSON array contains the expected command names in expected order.
  - The `scope` JSON object has the correct `type` (`all_private_chats`, `chat`) and `chat_id` where applicable.
  - Description strings are non-empty English.
- **No E2E tests**: this project has no Playwright/Cypress harness. Manual verification against a real bot token is captured in Post-Completion.

## Progress Tracking

- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with ➕ prefix
- Document issues/blockers with ⚠️ prefix
- Update plan if implementation deviates from original scope
- Keep plan in sync with actual work done

## What Goes Where

- **Implementation Steps** (`[ ]` checkboxes): code edits and unit tests in this repo.
- **Post-Completion** (no checkboxes): manual verification on a real Telegram bot token.

## Implementation Steps

### Task 1: Define command catalog and per-scope lists

- [x] Create `internal/telegram/commands.go` with three exported slices: `userCommands`, `adminCommands`, `groupCommands` — each `[]tgbotapi.BotCommand{...}` with `Command` (no leading `/`) and `Description` (English, ≤ 80 chars per Telegram limit).
- [x] Populate `userCommands` with: start, help, status, schedule, volunteer, explain, chore, takechore, sviniya, spend. Descriptions adapted from `userHelpMessage` in `handlers/commands.go:23-31`.
- [x] Populate `adminCommands` as the **concatenation** of `userCommands` plus admin-only entries: assign, unassign, modify, cancel, edit, offduty, vacation, users, toggle_active, ratings, chore_stats, list, complete, overdue, set_sviniya_balance. Descriptions adapted from `adminHelpMessageSection` in `handlers/commands.go:33-45`.
- [x] Populate `groupCommands` with the read-only subset: help, status, schedule, ratings, sviniya, chore_stats, overdue.
- [x] Add a small unit test asserting each slice is non-empty, has no duplicate command names within itself, and every description is ≤ 256 chars (Telegram limit) and non-empty.
- [x] Run `go test ./internal/telegram/...` — must pass before next task.

### Task 2: Implement `registerCommands` helper

- [x] In `internal/telegram/commands.go`, add `func registerCommands(api *tgbotapi.BotAPI, adminID, groupID int64) error` that issues up to three `setMyCommands` requests via `api.Request(...)`:
  - Always: `tgbotapi.NewSetMyCommandsWithScope(tgbotapi.NewBotCommandScopeAllPrivateChats(), userCommands...)`
  - If `adminID != 0`: `tgbotapi.NewSetMyCommandsWithScope(tgbotapi.NewBotCommandScopeChat(adminID), adminCommands...)`
  - If `groupID != 0`: `tgbotapi.NewSetMyCommandsWithScope(tgbotapi.NewBotCommandScopeChat(groupID), groupCommands...)`
- [x] On each individual call's error, log `slog.Warn` with the scope label and error, and continue to the next call. Return `nil` (collected errors are best-effort; failures are non-fatal per design decision).
- [x] Write a unit test that uses `NewTestClient` to capture all `setMyCommands` HTTP requests when both `adminID` and `groupID` are non-zero. Decode each request's `scope` and `commands` form fields and assert the expected three payloads (one per scope) with correct command lists.
- [x] Write a unit test for `adminID == 0, groupID == 0`: assert only the `all_private_chats` request is made.
- [x] Write a unit test for `adminID != 0, groupID == 0`: assert two requests (private chats + admin chat); no group chat request.
- [x] Write a unit test for the failure path: configure `NewTestClient` to return HTTP 500 for the admin-scope call; assert the function still returns `nil` and the other scope calls still happen.
- [x] Run `go test ./internal/telegram/...` — must pass before next task.

### Task 3: Wire `registerCommands` into `NewBot`

- [x] In `internal/telegram/bot.go`, after the `slog.Info("Authorized on account ...")` line (currently line 29), call `registerCommands(api, ownerID, groupID)`. The result is intentionally discarded; per-scope failures are already logged inside the helper.
- [x] Add an `slog.Info("registered bot commands", ...)` log after the call for observability.
- [x] Add a unit test for `NewBot` flow: use `NewTestClient` that handles `getMe` + all three `setMyCommands` calls; instantiate via `tgbotapi.NewBotAPIWithClient` and call the registration helper directly (since `NewBot` itself takes a token, not a client — see the pattern of factoring the side-effect into a helper for testability). Assert the three Telegram requests happened in expected order.
- [x] Verify existing `internal/telegram` and `internal/telegram/handlers` tests still pass: `go test ./internal/telegram/...`
- [x] Run tests — must pass before next task.

### Task 4: Update help text + CLAUDE.md project knowledge

- [x] Verify the `userHelpMessage` / `adminHelpMessageSection` strings in `internal/telegram/handlers/commands.go:23-45` still match the registered command lists. If any discrepancy (e.g., `/sviniya` and `/spend` are missing from `userHelpMessage`), update the help strings so `/help` and Telegram autocomplete stay in sync.
- [x] Add a short section to `CLAUDE.md` under a new heading "Bot Commands & Scopes" documenting: where `setMyCommands` is registered, the three-scope split, and the rule "admin scope contains user commands too (Telegram scope precedence replaces, not merges)".
- [x] No new tests required for this task (docs/strings only); existing tests must still pass.
- [x] Run `go test ./...` and `go vet ./...` — must pass before next task.

### Task 5: Verify acceptance criteria

- [ ] Verify three scopes are registered when both `ADMIN_ID` and `DISH_GROUP` are set.
- [ ] Verify only `all_private_chats` is registered when both are zero.
- [ ] Verify `setMyCommands` failure does not block bot startup (the failure-path test from Task 2 covers this; also confirm no panic by inspecting the registration helper return type).
- [ ] Verify command descriptions are within Telegram limits (32 chars for command name, 256 chars for description — though we already aim for ≤ 80).
- [ ] Run `go test ./...` — full suite passes.
- [ ] Run `go vet ./...` — clean.

### Task 6: [Final] Build verification

- [ ] `go build -o /tmp/roster-bot ./cmd/roster-bot/` — clean build.
- [ ] Remove `/tmp/roster-bot` after verification.

*Note: ralphex automatically moves completed plans to `docs/plans/completed/`*

## Technical Details

**Telegram scope precedence (highest wins):**

```
BotCommandScopeChatMember
> BotCommandScopeChat
> BotCommandScopeAllChatAdministrators
> BotCommandScopeChatAdministrators
> BotCommandScopeAllGroupChats / BotCommandScopeAllPrivateChats
> BotCommandScopeDefault
```

This is why the admin DM scope (`BotCommandScopeChat(adminID)`) must contain user commands too — it *replaces*, not merges with, `BotCommandScopeAllPrivateChats`.

**API call shape** (per `tgbotapi` v5):

```go
cmd := tgbotapi.NewSetMyCommandsWithScope(
    tgbotapi.NewBotCommandScopeAllPrivateChats(),
    tgbotapi.BotCommand{Command: "schedule", Description: "View the duty schedule for the current month"},
    // ...
)
_, err := api.Request(cmd)
```

**File layout decision**: command lists live in `internal/telegram/commands.go` (new), not in `internal/telegram/handlers/commands.go` (existing). Rationale: the existing file contains help-text *constants* used by handlers; our new file contains *Telegram API command metadata* used by bot startup. Different concerns, different files, both small.

**Description style** (English, action-oriented, ≤ 80 chars, no trailing period — matches Telegram bot conventions):

- `start` — Register and show the welcome message
- `help` — Show available commands
- `status` — Show your current duty statistics
- `schedule` — View the duty schedule for the current month
- `volunteer` — Sign up for a duty
- `explain` — Explain how the last assignment was made
- `chore` — View your assigned chores
- `takechore` — Volunteer to take an available chore
- `sviniya` — View all sviniya balances
- `spend` — Spend a sviniya
- `assign` — Add days to a user's admin queue
- `unassign` — Remove days from a user's admin queue
- `modify` — Change the assigned user for a date
- `cancel` — Cancel a duty or chore
- `edit` — Edit an active chore
- `offduty` — Set off-duty period for a user
- `vacation` — Toggle vacation mode for a user
- `users` — List all users and their status
- `toggle_active` — Toggle a user's participation
- `ratings` — Show this month's participant ratings
- `chore_stats` — Show top overdue chores and top completions
- `list` — List active periodic chores or tasks
- `complete` — Mark any active chore as completed
- `overdue` — Send the overdue chores report
- `set_sviniya_balance` — Set sviniya balance for a user

(Final descriptions may be tweaked during Task 1; this list is the source of truth.)

## Post-Completion

*Items requiring manual intervention or external systems — no checkboxes, informational only*

**Manual verification on a real bot token:**

- Restart the bot against a development Telegram bot token with `ADMIN_ID` and `DISH_GROUP` set.
- In a non-admin user's DM with the bot, type `/` and confirm the user command list appears (no admin commands).
- In the admin's DM, type `/` and confirm both user and admin commands appear.
- In the DISH_GROUP, type `/` and confirm only the curated read-only subset appears.
- Restart Telegram client or wait a few minutes if the cached autocomplete is stale (Telegram caches command lists client-side).

**Operational notes:**

- `setMyCommands` is sticky on Telegram's side: once registered, the lists persist across bot restarts. A future change that removes a command should also re-register `setMyCommands` to drop it from autocomplete, otherwise stale entries remain visible to clients.
- If a future change introduces multilingual support, switch to `tgbotapi.NewSetMyCommandsWithScopeAndLanguage` and register per `language_code`.
