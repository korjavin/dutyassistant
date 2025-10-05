# Queue-Based Duty System - Implementation Summary

## Overview
Successfully transformed the duty assignment system from calendar-based to queue-based, implementing the logic specified in [logic.md](logic.md).

## Completed Phases

### ✅ Phase 1: Database Layer
- Added queue fields to User: `volunteer_queue_days`, `admin_queue_days`, `off_duty_start`, `off_duty_end`
- Added `completed_at` field to Duty
- Updated migration to handle existing databases safely
- Removed `round_robin_state` table (no longer needed)

### ✅ Phase 2: Store Layer
- Updated all user queries to include new fields
- Created helper functions `scanUser()` and `scanUserRows()` for consistent scanning
- Added queue management methods:
  - `AddToVolunteerQueue/AddToAdminQueue`
  - `DecrementVolunteerQueue/DecrementAdminQueue`
  - `GetUsersWithVolunteerQueue/GetUsersWithAdminQueue`
- Added off-duty management methods:
  - `SetOffDuty/ClearOffDuty`
  - `IsUserOffDuty/GetOffDutyUsers`
- Added duty completion methods:
  - `CompleteDuty`
  - `GetTodaysDuty`
  - `GetCompletedDutiesInRange`

### ✅ Phase 3: Scheduler Logic
Complete rewrite of [internal/scheduler/scheduler.go](internal/scheduler/scheduler.go):
- Implemented queue-based assignment with priority: volunteer > admin > round-robin
- Added `AssignTodaysDuty()` - runs daily at 11AM Berlin time
- Added balancing logic when multiple users have same queue count
- Round-robin selection based on last 14 days of completed duties (excluding admin assignments)
- Off-duty filtering integrated into assignment logic
- Added `CompleteTodaysDuty()` - runs daily at 21PM
- Added `ChangeDutyUser()` for admin to modify assignments

### ✅ Phase 4: Command Handlers
Updated Telegram bot commands:
- `/volunteer <days>` - Adds to volunteer queue (was calendar-based)
- `/assign <username> <days>` - Adds to admin queue (was calendar-based)
- `/change <date> <username>` - Changes duty assignment (alias for /modify)
- `/offduty <username> <start> <end>` - Sets off-duty period (NEW)
- Updated `/status` to show queue counts and off-duty status
- Updated `/users` to display queues and off-duty with emojis
- Updated `/help` with new command formats

### ✅ Phase 5: Scheduled Jobs
Added cron scheduler in [cmd/roster-bot/main.go](cmd/roster-bot/main.go):
- **11:00 AM Berlin** - Assign today's duty using queue priority
- **21:00 PM Berlin** - Mark duty as completed
- **Sunday 21:10 PM Berlin** - Weekly stats (placeholder)
- Graceful shutdown for cron scheduler

### ✅ Phase 6: Frontend Display
- Enhanced `/status` output with queue info and off-duty status (HTML formatted)
- Enhanced `/users` output with queue counts and off-duty periods (emojis)
- Removed calendar-based volunteer interaction (no longer needed)

### ✅ Phase 7: Cleanup
- Removed `internal/http/handlers/prognosis.go`
- Removed prognosis endpoint from server routes
- Removed old round-robin methods: `GetNextRoundRobinUser`, `IncrementAssignmentCount`
- Removed `round_robin_state` table from schema

## Key Files Modified

### Core Logic
- `internal/scheduler/scheduler.go` - Complete rewrite
- `internal/scheduler/adapter.go` - Updated interface

### Database
- `internal/store/store.go` - Updated interfaces
- `internal/store/sqlite/sqlite.go` - Updated queries and added new methods

### Commands
- `internal/telegram/handlers/admin.go` - Updated admin commands
- `internal/telegram/handlers/volunteer.go` - Simplified to queue-based
- `internal/telegram/handlers/commands.go` - Updated help and status
- `internal/telegram/bot.go` - Added new command routes

### Main
- `cmd/roster-bot/main.go` - Added cron jobs

### Config
- `internal/http/server.go` - Removed prognosis endpoint

## Breaking Changes

1. **API Changes:**
   - `/volunteer` now requires `<days>` instead of calendar selection
   - `/assign` now requires `<days>` instead of date

2. **Database Schema:**
   - New fields added to users table
   - `round_robin_state` table removed (data migration not needed)

3. **Removed Features:**
   - Prognosis endpoint and display
   - Calendar-based volunteering UI

## Testing Status

⚠️ **Tests are broken** - Mock stores need regeneration with new interface methods:
- Add queue management methods to mocks
- Update scheduler mocks with new method signatures
- Remove old round-robin method references

Main application binary builds successfully: ✅

## Next Steps

1. **Regenerate Mocks:**
   ```bash
   mockgen -source=internal/store/store.go -destination=internal/store/mocks/store.go -package=mocks
   ```

2. **Update Tests:**
   - Fix scheduler tests to use new queue-based methods
   - Update handler tests with new mock signatures

3. **Optional Enhancements:**
   - Implement weekly stats notification to DISH_GROUP
   - Add notification when duty is assigned at 11AM
   - Add web UI for queue management

## Deployment Notes

- Database migration is backward compatible (uses ALTER TABLE with error suppression)
- Existing users will have queue counts of 0
- Cron jobs use Europe/Berlin timezone
- Environment variables unchanged
