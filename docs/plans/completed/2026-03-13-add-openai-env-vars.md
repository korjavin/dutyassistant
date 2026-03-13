# Add OPENAI_MODEL and OPENAI_TEMPERATURE configuration with initialization logging

## Overview
Currently, the OpenAI LLM client uses hardcoded values for the model (gpt-4o-mini) and temperature (0.7). This plan introduces two new environment variables, OPENAI_MODEL and OPENAI_TEMPERATURE, to allow these parameters to be configured. Additionally, it adds detailed logging during bot initialization to show the LLM configuration status, including whether it's enabled or disabled.

## Context
- Files involved:
  - `internal/llm/openai.go`: Update Client struct, NewClient, and RefineMessage.
  - `internal/llm/openai_test.go`: Update tests for the new NewClient signature.
  - `cmd/roster-bot/main.go`: Read new environment variables, pass them to llm.NewClient, and log the configuration.
  - `deployments/docker-compose.yml`: Add the new variables to the environment configuration.
- Related patterns:
  - Environment variable handling in `main.go` using `getEnv`.
  - Standard logging using `log` package.

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Update LLM Client to support configurable model and temperature

**Files:**
- Modify: `internal/llm/openai.go`

- [ ] Add `model string` and `temperature float64` fields to the `Client` struct.
- [ ] Update `NewClient` signature to accept `model` and `temperature`.
- [ ] Implement default values in `NewClient`:
  - If `model` is empty, use `gpt-4o-mini`.
  - If `temperature` is 0, use `0.7`.
- [ ] Add exported `Config()` (or similar) method to `Client` to retrieve current settings for logging.
- [ ] Update `RefineMessage` to use `c.model` and `c.temperature` when constructing the `chatRequest`.
- [ ] Run `go build ./internal/llm` to ensure no syntax errors.

### Task 2: Update tests for the new LLM Client signature

**Files:**
- Modify: `internal/llm/openai_test.go`

- [ ] Update all calls to `NewClient` in `internal/llm/openai_test.go` with additional arguments.
- [ ] Add a new test case in `internal/llm/openai_test.go` to verify that custom model and temperature are correctly passed in the request.
- [ ] Run tests: `go test ./internal/llm/...`

### Task 3: Wire up environment variables and add initialization logging in main.go

**Files:**
- Modify: `cmd/roster-bot/main.go`

- [ ] Read `OPENAI_MODEL` using `getEnv("OPENAI_MODEL", "gpt-4o-mini")`.
- [ ] Read `OPENAI_TEMPERATURE` using `getEnv("OPENAI_TEMPERATURE", "0.7")`.
- [ ] Convert `OPENAI_TEMPERATURE` string to `float64` (add `parseFloat64` helper in `main.go`).
- [ ] Pass the new values to `llm.NewClient`.
- [ ] Add logging to show:
  - Status: "LLM Client: Enabled" or "LLM Client: Disabled (OPENAI_API_KEY not set)".
  - If enabled, log "Provider: OpenAI", "Model: [model]", "Temperature: [temp]", and "BaseURL: [url]" (if custom).
- [ ] Run `go build ./cmd/roster-bot` to verify compilation.

### Task 4: Update deployment configuration

**Files:**
- Modify: `deployments/docker-compose.yml`

- [ ] Add `- OPENAI_MODEL=${OPENAI_MODEL:-gpt-4o-mini}` to the `roster-bot` service environment.
- [ ] Add `- OPENAI_TEMPERATURE=${OPENAI_TEMPERATURE:-0.7}` to the `roster-bot` service environment.

### Task 5: Verify implementation

- [ ] Run all project tests: `go test ./...`
- [ ] Run the bot and verify initialization logs: `go run ./cmd/roster-bot/main.go`
- [ ] Verify logs show expected LLM configuration details.
- [ ] Verify the bot starts and functions correctly.

### Task 6: Update documentation

- [ ] Update `README.md` if it lists environment variables.
- [ ] Move this plan to `docs/plans/completed/` after implementation (to be done by the developer).
