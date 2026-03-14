# Refactor Bot Architecture into Layered Design with FSM

## Overview
This plan refactors the `dutyassistant` codebase to achieve a clean separation of concerns. It introduces a Domain layer for core models and interfaces, a Service layer for platform-agnostic business logic, and a refactored Telegram layer using a dedicated Finite State Machine (FSM) for complex command flows.

## Context
- Files involved:
    - `internal/domain/`: (New) Core entities and interfaces.
    - `internal/service/`: (New) Business logic services.
    - `internal/bot/`: (Refactored from `internal/telegram`) Bot-specific logic and FSM.
    - `internal/api/`: (Refactored from `internal/http`) Web API layer.
    - `internal/store/`: (Repository) Data access implementation.
    - `cmd/roster-bot/main.go`: Application entry point and wiring.
- Related patterns: Clean Architecture (Ports and Adapters), Finite State Machine.
- Dependencies: Standard library, existing Telegram bot API, SQLite.

## Development Approach
- **Testing approach**: Regular (code first, then tests) but with high focus on unit testing the Service and Domain layers.
- Complete each task fully before moving to the next.
- **CRITICAL: every task MUST include new/updated tests.**
- **CRITICAL: all tests must pass before starting next task.**

## Implementation Steps

### Task 1: Establish Domain Layer

**Files:**
- Create: `internal/domain/models.go`
- Create: `internal/domain/interfaces.go`

- [ ] Define core entities: `User`, `Duty`, `Chore`, `RecurringChore`, `Rating`.
- [ ] Define `Repository` interface covering all data access needs.
- [ ] Define service interfaces: `DutyService`, `ChoreService`, `RatingService`.
- [ ] Write unit tests for domain model logic (if any).
- [ ] Run project test suite.

### Task 2: Implement Service Layer

**Files:**
- Create: `internal/service/chore.go`
- Create: `internal/service/duty.go`
- Create: `internal/service/rating.go`
- Modify: `internal/scheduler/` (Move logic to services)

- [ ] Implement `ChoreService` with logic for creation, assignment, and recurring processing.
- [ ] Implement `DutyService` for assignments, completions, and scheduling.
- [ ] Implement `RatingService` for managing participant ratings.
- [ ] Move LLM refinement logic into a service-level decorator or helper.
- [ ] **CRITICAL: Add comprehensive unit tests for all services using mocked repositories.**
- [ ] Run project test suite.

### Task 3: Build FSM and Session Infrastructure

**Files:**
- Create: `internal/bot/fsm/fsm.go`
- Create: `internal/bot/session/manager.go`

- [ ] Implement a generic FSM with support for states, events, and context.
- [ ] Implement a persistent session manager that stores FSM state and data.
- [ ] Define `ChoreCreationFlow` and `DailyRatingsFlow` using the new FSM.
- [ ] Write tests for the FSM engine and session transitions.
- [ ] Run project test suite.

### Task 4: Refactor Telegram Layer (internal/bot)

**Files:**
- Create: `internal/bot/bot.go`
- Create: `internal/bot/handlers.go`
- Create: `internal/bot/middleware.go`
- Remove: `internal/telegram/` (after migration)

- [ ] Implement a central dispatcher that routes messages/callbacks to FSM or simple handlers.
- [ ] Implement middleware for access control (owner/group check).
- [ ] Convert interactive handlers to use FSM states and events.
- [ ] Use `internal/service` for all business operations.
- [ ] **CRITICAL: Ensure existing bot tests are migrated and passing with the new structure.**
- [ ] Run project test suite.

### Task 5: Refactor Web Layer (internal/api)

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/handlers.go`
- Remove: `internal/http/` (after migration)

- [ ] Refactor HTTP handlers to depend on services instead of the raw store.
- [ ] Align API response structures with domain models.
- [ ] Update `main.go` to use the new API server.
- [ ] Verify API functionality with existing integration tests.
- [ ] Run project test suite.

### Task 6: Final Integration and Cleanup

**Files:**
- Modify: `cmd/roster-bot/main.go`
- Modify: `internal/scheduler/` (Clean up deprecated code)

- [ ] Move cron job orchestration logic from `main.go` to a dedicated `App` or `Scheduler` component.
- [ ] Update `main.go` to perform dependency injection for all layers.
- [ ] Remove all deprecated `internal/telegram/handlers` and old logic.
- [ ] Final end-to-end verification of all bot commands and API endpoints.
- [ ] Run project test suite.

### Task 7: Verify acceptance criteria

- [ ] manual test: Verify interactive chore creation works via FSM
- [ ] manual test: Verify daily ratings flow works via FSM
- [ ] run full test suite (go test ./...)
- [ ] run linter (golangci-lint run)
- [ ] verify test coverage meets 80%+

### Task 8: Update documentation

- [ ] update README.md if user-facing changes
- [ ] update CLAUDE.md if internal patterns changed
- [ ] move this plan to `docs/plans/completed/`
