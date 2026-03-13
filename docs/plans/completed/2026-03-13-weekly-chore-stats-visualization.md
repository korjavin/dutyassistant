---
# Weekly Chore Statistics with Visualization and Winner

## Overview
Extend the existing Sunday weekly stats notification to include per-user metrics (chore count, average execution time, average late time), emoji bar chart visualization, and a "winner of the week" announcement. All rendered as text/emoji using existing Telegram HTML mode - no new dependencies.

## Context
- Files involved:
  - `internal/store/store.go` - add UserWeeklyStats struct + Store interface method
  - `internal/store/sqlite/chores.go` - implement GetUserWeeklyStats SQL query
  - `internal/store/sqlite/chores_test.go` - store tests
  - `internal/notification/notifier.go` - update SendWeeklyChoreStats, add bar chart and winner helpers
  - `internal/notification/notifier_test.go` - notification tests
- Related patterns: strings.Builder + HTML mode messages, Berlin timezone cron at Sunday 21:10, existing GetTopCompletedChoresUsers/GetTopOverdueChores patterns
- Dependencies: none new

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add UserWeeklyStats type and GetUserWeeklyStats store method

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite/chores.go`
- Modify: `internal/store/sqlite/chores_test.go`

- [ ] Add UserWeeklyStats struct to store.go: `{ Name string, CompletedCount int, AvgExecSeconds float64, AvgLateSeconds float64 }`
- [ ] Add `GetUserWeeklyStats(ctx context.Context, since time.Time) ([]*UserWeeklyStats, error)` to Store interface
- [ ] Implement GetUserWeeklyStats in sqlite/chores.go: JOIN chores+users, filter completed_at >= since, GROUP BY user_id, compute AVG exec seconds (completed_at - assigned_at) and AVG late seconds (MAX(0, completed_at - deadline_at)), ORDER BY completed_count DESC
- [ ] Write tests for GetUserWeeklyStats covering: empty result, single user, multiple users with late chores
- [ ] Run project test suite - must pass before task 2

### Task 2: Update SendWeeklyChoreStats with bar charts and winner detection

**Files:**
- Modify: `internal/notification/notifier.go`
- Modify: `internal/notification/notifier_test.go`

- [ ] Add `renderBarChart(value, maxValue float64, width int) string` helper - fills Unicode block chars (█ filled, ░ empty) proportionally
- [ ] Add `determineWinner(stats []*store.UserWeeklyStats) *store.UserWeeklyStats` helper - ranks by completed count desc, then avg late seconds asc
- [ ] Update SendWeeklyChoreStats to call `GetUserWeeklyStats(ctx, time.Now().AddDate(0, 0, -7))`
- [ ] Build new message section "🏆 Top Performers this week:" with per-user rows showing: rank, name, bar chart for chore count, count, avg execution time in human-readable format (e.g. "2h30m"), on-time or late indicator with avg late time
- [ ] Add "🥇 Winner of the week: {Name}" line below the table, with short reason (e.g. "most chores completed, on time!")
- [ ] Keep existing overdue section intact
- [ ] Write tests for renderBarChart and determineWinner helpers, and the full message format
- [ ] Run project test suite - must pass before task 3

### Task 3: Verify acceptance criteria

- [ ] manual test: trigger SendWeeklyChoreStats with test data and verify Telegram message renders correctly
- [ ] run full test suite: `go test ./...`
- [ ] run linter: `golangci-lint run` (or `go vet ./...`)
- [ ] verify all three metrics shown per user (count, execution time, late time)
- [ ] verify winner line appears and picks correct user

### Task 4: Update documentation

- [ ] update README.md to document the weekly stats message format
- [ ] move this plan to `docs/plans/completed/`
