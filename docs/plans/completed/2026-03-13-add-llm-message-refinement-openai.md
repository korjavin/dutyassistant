---
# Add LLM Message Refinement with OpenAI

## Overview
Add an LLM adapter that optionally refines hardcoded notification messages with humor and personality. The adapter calls OpenAI to reformat messages while preserving their intent. Falls back to vanilla messages if LLM is not configured or fails.

## Context
- Files involved:
  - `deployments/docker-compose.yml` - add OPENAI_* env vars
  - `cmd/roster-bot/main.go` - read new env vars, initialize LLM adapter
  - `internal/llm/openai.go` - NEW: LLM adapter (modeled after german-conjuctions-trainer reference)
  - `internal/notification/notifier.go` - inject LLM, refine duty assignment messages
  - `internal/notification/formatter.go` - has FormatDutyAssignedMessage, FormatDMToAssignee
  - `internal/telegram/handlers/chore_reminder.go` - has SendCompletionToGroup
- Related patterns: getEnv() utility in main.go, Notifier struct pattern in notifier.go
- Reference: /Users/iv/Projects/german-conjuctions-trainer/pkg/llm/openai.go

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Create LLM adapter package

**Files:**
- Create: `internal/llm/openai.go`
- Create: `internal/llm/openai_test.go`

- [ ] Create `internal/llm/` package
- [ ] Define `Client` struct with fields: apiKey, baseURL string, timeout time.Duration
- [ ] Implement `NewClient(apiKey, baseURL string, timeoutSeconds int) *Client`
- [ ] If apiKey is empty, return nil (disabled state)
- [ ] Implement `RefineMessage(ctx context.Context, intent, vanilla string) string` on Client
  - Builds an OpenAI chat completion request: system prompt asks for fun/humorous reformatting, user message is the intent + vanilla text
  - On any error (timeout, API error, parse error) returns vanilla unchanged
  - Uses HTTP client with configured timeout
  - Uses Bearer token auth
  - Defaults baseURL to "https://api.openai.com/v1" if empty
- [ ] Write unit tests using httptest.NewServer to mock OpenAI API
  - Test: nil client returns vanilla
  - Test: successful response returns LLM text
  - Test: timeout returns vanilla
  - Test: non-200 response returns vanilla
- [ ] Run tests: `go test ./internal/llm/...`

### Task 2: Add env vars to docker-compose and wire up in main.go

**Files:**
- Modify: `deployments/docker-compose.yml`
- Modify: `cmd/roster-bot/main.go`

- [ ] Add to docker-compose.yml environment section:
  - `- OPENAI_API_KEY=${OPENAI_API_KEY:-}`
  - `- OPENAI_URL=${OPENAI_URL:-}`
  - `- OPENAI_TIMEOUT_SECONDS=${OPENAI_TIMEOUT_SECONDS:-10}`
- [ ] In main.go read envs using getEnv():
  - `openaiAPIKey := getEnv("OPENAI_API_KEY", "")`
  - `openaiURL := getEnv("OPENAI_URL", "")`
  - `openaiTimeout := parseInt64(getEnv("OPENAI_TIMEOUT_SECONDS", "10"), 10)`
- [ ] Initialize `llmClient := llm.NewClient(openaiAPIKey, openaiURL, int(openaiTimeout))`
- [ ] Pass llmClient to Notifier constructor (updated in Task 3)
- [ ] Run: `go build ./...`

### Task 3: Inject LLM into Notifier and apply to key messages

**Files:**
- Modify: `internal/notification/notifier.go`
- Modify: `internal/notification/formatter.go`
- Modify: `internal/telegram/handlers/chore_reminder.go`

- [ ] Add `llmClient *llm.Client` field to Notifier struct
- [ ] Update Notifier constructor to accept `*llm.Client`
- [ ] In `SendDutyAssignment()` (or wherever FormatDutyAssignedMessage result is sent), call `llmClient.RefineMessage(ctx, "congratulate duty assignee proudly", msg)` before sending
- [ ] In `SendDMToAssignee()`, refine the DM with intent "friendly congratulatory DM to person assigned chore"
- [ ] In `SendCompletionToGroup()` in chore_reminder.go, inject llmClient and refine "celebrate chore completion, be proud of them" before sending to group
- [ ] All calls use a `if llmClient != nil` guard so nil client is safe
- [ ] Write tests for Notifier with mock LLM client (nil and active)
- [ ] Run tests: `go test ./internal/notification/... ./internal/telegram/...`

### Task 4: Verify acceptance criteria

- [ ] Manual test: set OPENAI_API_KEY to empty, start bot - messages use hardcoded text
- [ ] Manual test: set valid OPENAI_API_KEY, assign a duty - message is LLM-refined
- [ ] Run full test suite: `go test ./...`
- [ ] Run linter: `go vet ./...`

### Task 5: Update documentation

- [ ] Update README.md with new OPENAI_* env vars and their defaults
- [ ] Move this plan to `docs/plans/completed/`
