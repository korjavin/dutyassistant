# Extend /who endpoint with active chores and assignees

## Overview
Modify the GET /who handler to also return currently active chores (tasks) and their assignees alongside the existing duty person name field. Deadlines are expressed as human-readable relative strings ("today", "yesterday", "2 days ago", etc.).

Note: no Swagger/OpenAPI files exist in this project, so no API doc update is needed.

## Context
- Files involved:
  - `internal/http/handlers/who.go` — handler to modify
  - `internal/http/handlers/who_test.go` — tests to update
- Related patterns: `GetActiveChores` in `internal/http/handlers/chores.go` shows the chore response shape
- Dependencies: `store.GetActiveChores` already exists and populates `chore.User`

Current response: `{"name": "Ivan"}`

New response shape:
```json
{
  "name": "Ivan",
  "chores": [
    {"description": "Buy milk", "deadline_at": "today", "assignee": "Maria"},
    ...
  ]
}
```

When no chores exist, `"chores"` is an empty array. When no duty is assigned, `"name"` is `""` and `"chores"` is still populated.

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Extend GetWho handler and update tests

**Files:**
- Modify: `internal/http/handlers/who.go`
- Modify: `internal/http/handlers/who_test.go`

- [ ] Add a `choreItem` struct with fields `Description`, `DeadlineAt`, `Assignee` (all strings)
- [ ] Add a helper to convert a `time.Time` deadline to a relative string: "today", "yesterday", "2 days ago", "3 days ago", etc.
- [ ] Change response struct to include `Name string` and `Chores []choreItem`
- [ ] Call `s.GetActiveChores(ctx)` after fetching duty; on error, log and return 500
- [ ] Build `[]choreItem` from active chores: use relative deadline string and `chore.User.FirstName` when User is non-nil, else empty string
- [ ] Return unified JSON with both `name` and `chores` in all success branches (including no-duty case)
- [ ] Update `TestGetWho_DutyAssigned` to also mock `GetActiveChores` and assert `chores` field
- [ ] Update `TestGetWho_NoDutyAssigned` to mock `GetActiveChores` returning empty list and assert `chores` is `[]`
- [ ] Update `TestGetWho_DutyWithoutUser` similarly
- [ ] Add `TestGetWho_ChoresError` for the case where `GetActiveChores` returns an error (expect 500)
- [ ] Add tests for the relative date helper covering: today, yesterday, 2 days ago, 3+ days ago
- [ ] Run `go test ./internal/http/handlers/...` — must pass

### Task 2: Verify acceptance criteria

- [ ] Manual test: call `GET /who` (with HMAC auth) and verify response includes both `name` and `chores`
- [ ] Run full test suite: `go test ./...`
- [ ] Run linter: `go vet ./...`
- [ ] Move this plan to `docs/plans/completed/`
