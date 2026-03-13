# Automatic Chore Translation for Non-Latin Descriptions

## Overview
Implement automatic translation of chore descriptions to English if they contain non-Latin characters (e.g., Cyrillic, Greek, Han scripts). This feature will trigger when an admin creates a recurring or one-off chore, ensuring consistent language in the group and database.

## Context
- Files involved:
  - `internal/llm/openai.go`: Add translation capability.
  - `internal/telegram/handlers/handlers.go`: Add detection and translation helper.
  - `internal/telegram/handlers/chore.go`: Integrate translation into chore creation flows.
- Related patterns: Existing `LLMClient` usage for message refinement.
- Dependencies: OpenAI API (via existing `LLMClient`).

## Development Approach
- **Testing approach**: Regular (code first, then tests).
- Complete each task fully before moving to the next.
- **CRITICAL: every task MUST include new/updated tests.**
- **CRITICAL: all tests must pass before starting next task.**

## Implementation Steps

### Task 1: Extend LLM Client with Translation

**Files:**
- Modify: `internal/llm/openai.go`
- Modify: `internal/llm/openai_test.go`

- [ ] Add `TranslateToEnglish(ctx context.Context, text string) (string, error)` to the `Client` interface and implementation.
- [ ] Use prompt: "You are a translator. Translate the given chore description to English. Be concise, do not be verbose. Return ONLY the translated text. Preserve emojis. If translation is not possible or text is already English, return original."
- [ ] Implement error handling: return original text if API call fails or client is nil.
- [ ] Add unit test in `openai_test.go` to verify the translation call.
- [ ] Run `go test ./internal/llm/...`

### Task 2: Implement Detection and Integrate in Handlers

**Files:**
- Modify: `internal/telegram/handlers/handlers.go`
- Modify: `internal/telegram/handlers/chore.go`

- [ ] Add `translateIfNonLatin(ctx context.Context, description string) string` helper to `Handlers` struct in `handlers.go`.
- [ ] Implement detection logic: Iterate through runes; if `unicode.IsLetter(r) && !unicode.Is(unicode.Latin, r)`, then it's non-Latin.
- [ ] In `HandleChore` (`chore.go`), apply translation to `description` before creating `RecurringChore`.
- [ ] In `HandleChore` (`chore.go`), apply translation to `description` before calling `assignChore` for one-off chores.
- [ ] In `HandleChoreInteractive` (`chore.go`), apply translation to the user-provided message text before assignment.
- [ ] Run `go test ./internal/telegram/handlers/...`

### Task 3: Verification and Testing

- [ ] Create `internal/telegram/handlers/chore_translation_test.go` to test:
  - [ ] Detection logic with various inputs (English, Russian, Mixed, Emojis).
  - [ ] Integration in `HandleChore` using a mock LLM client.
- [ ] Run project test suite: `go test ./...`
- [ ] Verify that no double-translation happens and that English/Latin descriptions remain unchanged.

### Task 4: Documentation and Cleanup

- [ ] Update README.md if translation behavior is user-visible documentation.
- [ ] Move this plan to `docs/plans/completed/`
- [ ] Verify there are no regressions in one-off or recurring chore creation.
- [ ] Run linter: `golangci-lint run` (if available) or `go vet ./...`.
