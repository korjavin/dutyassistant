---
# Fix LLM Translation and Add /chore translate Command

## Overview
The `TranslateToEnglish` method in `internal/llm/openai.go` uses a hardcoded model name "gpt-4o-mini" instead of the configured model (`c.model`). When users configure a different LLM provider like DeepSeek, the translation fails with a "Model Not Exist" error. This plan fixes that issue and adds a `/chore translate <id>` command to allow retrospective translation of existing recurring chore descriptions.

## Context
- Files involved:
  - `internal/llm/openai.go`: Fix hardcoded model name in TranslateToEnglish
  - `internal/telegram/handlers/chore.go`: Add /chore translate <id> subcommand
  - `internal/telegram/bot.go`: No changes needed (handled in existing HandleChore)
- Related patterns: Existing translateIfNonLatin helper and command handling in HandleChore
- Dependencies: None (uses existing LLMClient and Store methods)

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Fix Hardcoded Model Name in TranslateToEnglish

**Files:**
- Modify: `internal/llm/openai.go`
- Modify: `internal/llm/openai_test.go`

- [ ] Change line 138 in TranslateToEnglish: replace hardcoded `"gpt-4o-mini"` with `c.model`
- [ ] Verify that temperature on line 143 is 0.3 (correct for translation - low temperature needed)
- [ ] Add unit test in openai_test.go to verify TranslateToEnglish uses the configured model
- [ ] Run `go test ./internal/llm/...`

### Task 2: Add /chore translate <id> Command

**Files:**
- Modify: `internal/telegram/handlers/chore.go`
- Create: `internal/telegram/handlers/chore_translate_test.go`

- [ ] Add subcommand parsing in HandleChore: check if args starts with "translate" followed by an ID
- [ ] Implement HandleChoreTranslate(m *tgbotapi.Message) handler function:
  - [ ] Parse the chore ID from arguments (format: `/chore translate <id>`)
  - [ ] Admin check (only admins can translate existing chores)
  - [ ] Fetch recurring chore via `h.Store.GetRecurringChore(id)`
  - [ ] Use existing `h.translateIfNonLatin(ctx, description)` to translate if non-Latin
  - [ ] Update description via `h.Store.UpdateRecurringChoreDescription(id, translated)`
  - [ ] Return success/error message to user
- [ ] Add integration tests in chore_translate_test.go:
  - [ ] Test successful translation of non-Latin description
  - [ ] Test no-op when description is already Latin
  - [ ] Test error handling for invalid chore ID
  - [ ] Test admin access control
- [ ] Run `go test ./internal/telegram/handlers/...`

### Task 3: Verify and Test End-to-End

- [ ] Manual test: Run bot, create a recurring chore with non-Latin description, verify translation works
- [ ] Manual test: Use `/chore translate <id>` on an existing untranslated chore
- [ ] Run project test suite: `go test ./...`
- [ ] Verify no regressions in existing /chore command behavior
- [ ] Run linter: `go vet ./...`

### Task 4: Documentation

- [ ] Update README.md if user-facing changes need documentation (optional - command is self-explanatory)
- [ ] Move this plan to `docs/plans/completed/`
