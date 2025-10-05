# Implementation Plan: Queue-Based Duty System

## Status: 🟡 In Progress
**Last Updated:** 2025-10-05
**Completed:** 1/13 tasks (8%)

---

## Phase 1: Database Layer ✅ COMPLETED

### 1.1 Schema Updates ✅
- [x] Add `volunteer_queue_days` INTEGER to users table
- [x] Add `admin_queue_days` INTEGER to users table
- [x] Add `off_duty_start` TEXT to users table
- [x] Add `off_duty_end` TEXT to users table
- [x] Add `completed_at` TEXT to duties table
- [x] Update `User` struct in store.go
- [x] Update `Duty` struct in store.go
- [x] Add migration logic to handle existing databases

**Files Modified:**
- ✅ `internal/store/store.go`
- ✅ `internal/store/sqlite/sqlite.go`

---

## Phase 2: Store Layer Methods 🔴 TODO

### 2.1 Update Existing User Queries
**File:** `internal/store/sqlite/sqlite.go`

Need to update ALL user queries to include new fields:

- [ ] `GetUserByTelegramID` - add queue fields to SELECT and Scan
- [ ] `GetUserByName` - add queue fields to SELECT and Scan
- [ ] `ListActiveUsers` - add queue fields to SELECT and Scan
- [ ] `ListAllUsers` - add queue fields to SELECT and Scan
- [ ] `CreateUser` - add queue fields to INSERT
- [ ] `UpdateUser` - add queue fields to UPDATE

**Example for GetUserByTelegramID:**
```go
query := `SELECT id, telegram_user_id, first_name, is_admin, is_active,
          volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
          FROM users WHERE telegram_user_id = ?`
row := s.db.QueryRowContext(ctx, query, id)
user := &store.User{}
var offDutyStart, offDutyEnd sql.NullString
err := row.Scan(&user.ID, &user.TelegramUserID, &user.FirstName,
                &user.IsAdmin, &user.IsActive, &user.VolunteerQueueDays,
                &user.AdminQueueDays, &offDutyStart, &offDutyEnd)
// Convert NULL strings to *time.Time
if offDutyStart.Valid {
    t, _ := time.Parse("2006-01-02", offDutyStart.String)
    user.OffDutyStart = &t
}
// ... same for offDutyEnd
```

### 2.2 Add New Queue Management Methods
**File:** `internal/store/sqlite/sqlite.go`

Add to Store interface in `internal/store/store.go`:
```go
// Queue management
AddToVolunteerQueue(ctx context.Context, userID int64, days int) error
AddToAdminQueue(ctx context.Context, userID int64, days int) error
DecrementVolunteerQueue(ctx context.Context, userID int64) error
DecrementAdminQueue(ctx context.Context, userID int64) error
GetUsersWithVolunteerQueue(ctx context.Context) ([]*User, error)
GetUsersWithAdminQueue(ctx context.Context) ([]*User, error)

// Off-duty management
SetOffDuty(ctx context.Context, userID int64, start, end time.Time) error
ClearOffDuty(ctx context.Context, userID int64) error
GetOffDutyUsers(ctx context.Context, date time.Time) ([]*User, error)
IsUserOffDuty(ctx context.Context, userID int64, date time.Time) (bool, error)
```

Implementation examples:
```go
func (s *SQLiteStore) AddToVolunteerQueue(ctx context.Context, userID int64, days int) error {
    query := `UPDATE users SET volunteer_queue_days = volunteer_queue_days + ? WHERE id = ?`
    _, err := s.db.ExecContext(ctx, query, days, userID)
    return err
}

func (s *SQLiteStore) GetUsersWithVolunteerQueue(ctx context.Context) ([]*User, error) {
    query := `SELECT id, telegram_user_id, first_name, is_admin, is_active,
              volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
              FROM users WHERE volunteer_queue_days > 0 AND is_active = 1
              ORDER BY volunteer_queue_days DESC, id ASC`
    // ... implementation
}
```

### 2.3 Update Duty Queries
**File:** `internal/store/sqlite/sqlite.go`

- [ ] `GetDutyByDate` - add completed_at to SELECT and Scan
- [ ] `GetDutiesByMonth` - add completed_at to SELECT and Scan
- [ ] Add `CompleteDuty(ctx context.Context, date time.Time, completedAt time.Time) error`
- [ ] Add `GetTodaysDuty(ctx context.Context, date time.Time) (*Duty, error)`
- [ ] Add `GetCompletedDutiesInRange(ctx context.Context, userID int64, start, end time.Time, excludeAdmin bool) (int, error)`

---

## Phase 3: Scheduler Logic Rewrite 🔴 TODO

### 3.1 New Queue-Based Assignment Logic
**File:** `internal/scheduler/scheduler.go`

**Complete rewrite needed.** Current logic is date-based assignment. New logic is queue-based.

New method signatures:
```go
// AssignDailyDuty runs at 11:00 AM to assign today's duty
func (s *Scheduler) AssignDailyDuty(ctx context.Context, date time.Time) (*store.Duty, error)

// CompleteDailyDuty runs at 21:00 PM to mark duty as completed
func (s *Scheduler) CompleteDailyDuty(ctx context.Context, date time.Time) error

// CalculateRoundRobinUser determines next round-robin user based on 14-day fairness
func (s *Scheduler) CalculateRoundRobinUser(ctx context.Context, date time.Time) (*store.User, error)

// SelectFromQueues handles queue balancing logic
func (s *Scheduler) SelectFromQueues(ctx context.Context, users []*store.User, queueType string) (*store.User, error)
```

**AssignDailyDuty Logic:**
```go
func (s *Scheduler) AssignDailyDuty(ctx context.Context, date time.Time) (*store.Duty, error) {
    // 1. Check if today already has a duty (from previous run or /change)
    existing, _ := s.store.GetDutyByDate(ctx, date)
    if existing != nil {
        return existing, nil // Already assigned
    }

    var selectedUser *store.User
    var assignmentType store.AssignmentType

    // 2. Priority 1: Check volunteer queues
    volunteerUsers, _ := s.store.GetUsersWithVolunteerQueue(ctx)
    if len(volunteerUsers) > 0 {
        // Filter out off-duty users
        activeVolunteers := filterOffDuty(volunteerUsers, date)
        if len(activeVolunteers) > 0 {
            selectedUser = s.SelectFromQueues(ctx, activeVolunteers, "volunteer")
            assignmentType = store.AssignmentTypeVoluntary
            s.store.DecrementVolunteerQueue(ctx, selectedUser.ID)
        }
    }

    // 3. Priority 2: Check admin queues (if no volunteer)
    if selectedUser == nil {
        adminUsers, _ := s.store.GetUsersWithAdminQueue(ctx)
        if len(adminUsers) > 0 {
            activeAdmins := filterOffDuty(adminUsers, date)
            if len(activeAdmins) > 0 {
                selectedUser = s.SelectFromQueues(ctx, activeAdmins, "admin")
                assignmentType = store.AssignmentTypeAdmin
                s.store.DecrementAdminQueue(ctx, selectedUser.ID)
            }
        }
    }

    // 4. Priority 3: Round-robin (if no queues)
    if selectedUser == nil {
        selectedUser, _ = s.CalculateRoundRobinUser(ctx, date)
        assignmentType = store.AssignmentTypeRoundRobin
    }

    // 5. Create duty assignment
    duty := &store.Duty{
        UserID:         selectedUser.ID,
        DutyDate:       date,
        AssignmentType: assignmentType,
        CreatedAt:      time.Now(),
    }
    s.store.CreateDuty(ctx, duty)

    return duty, nil
}
```

**CalculateRoundRobinUser Logic:**
```go
func (s *Scheduler) CalculateRoundRobinUser(ctx context.Context, date time.Time) (*store.User, error) {
    // Get all active users (excluding off-duty, inactive, and admin)
    users, _ := s.store.ListActiveUsers(ctx)

    // Filter out off-duty users for this date
    eligibleUsers := []*store.User{}
    for _, user := range users {
        if user.IsAdmin {
            continue
        }
        offDuty, _ := s.store.IsUserOffDuty(ctx, user.ID, date)
        if offDuty {
            continue
        }
        eligibleUsers = append(eligibleUsers, user)
    }

    // Calculate 14-day counts for each user (excluding admin assignments)
    start := date.AddDate(0, 0, -14)
    userCounts := make(map[int64]int)
    var lastDutyDates map[int64]time.Time

    for _, user := range eligibleUsers {
        count, _ := s.store.GetCompletedDutiesInRange(ctx, user.ID, start, date, true) // excludeAdmin=true
        userCounts[user.ID] = count
    }

    // Select user with lowest count (or oldest last duty date if tied)
    // ... selection logic
}
```

### 3.2 Delete Old Methods
**Files to update:**
- `internal/scheduler/scheduler.go` - remove `AssignDutyVoluntary`, `AssignDutyAdmin`, `AssignDutyRoundRobin`
- `internal/scheduler/adapter.go` - update interface

---

## Phase 4: Command Handlers Rewrite 🔴 TODO

### 4.1 Rewrite /volunteer Command
**File:** `internal/telegram/handlers/volunteer.go`

**Current:** Shows calendar, user selects date, creates duty
**New:** Prompts for number of days, adds to queue

```go
func (h *Handlers) HandleVolunteer(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
    args := m.CommandArguments()

    if args == "" {
        // Start interactive flow - ask for number of days
        return h.askForVolunteerDays(m)
    }

    // Parse number of days from argument
    days, err := strconv.Atoi(args)
    if err != nil || days < 1 {
        return tgbotapi.NewMessage(m.Chat.ID, "Please provide a valid number of days (e.g., /volunteer 3)")
    }

    // Get user
    user, _ := h.Store.GetUserByTelegramID(ctx, m.From.ID)
    if user == nil {
        return tgbotapi.NewMessage(m.Chat.ID, "Please use /start first")
    }

    // Add to volunteer queue
    h.Store.AddToVolunteerQueue(ctx, user.ID, days)

    msg := fmt.Sprintf("✅ Added %d days to your volunteer queue!\nYour queue: %d days",
                       days, user.VolunteerQueueDays + days)
    return tgbotapi.NewMessage(m.Chat.ID, msg)
}
```

**Interactive flow needed:**
- Store conversation state (use callback data or conversation manager)
- Timeout after 10 minutes → default to 1 day

### 4.2 Rewrite /assign Command
**File:** `internal/telegram/handlers/admin.go`

**Current:** `/assign username date`
**New:** `/assign [username] [days]` with interactive prompts

```go
func (h *Handlers) HandleAssign(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
    isAdmin, _ := h.checkAdmin(m.From.ID)
    if !isAdmin {
        return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage)
    }

    args := strings.Fields(m.CommandArguments())

    // Case 1: No arguments - prompt for username with buttons
    if len(args) == 0 {
        return h.promptForAssignUsername(m)
    }

    // Case 2: One argument (username) - prompt for days
    if len(args) == 1 {
        return h.promptForAssignDays(m, args[0])
    }

    // Case 3: Both provided - execute
    username := args[0]
    days, err := strconv.Atoi(args[1])
    if err != nil || days < 1 {
        return tgbotapi.NewMessage(m.Chat.ID, "Invalid number of days")
    }

    user, _ := h.Store.GetUserByName(ctx, username)
    if user == nil {
        return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("User %s not found", username))
    }

    h.Store.AddToAdminQueue(ctx, user.ID, days)

    msg := fmt.Sprintf("✅ Added %d days to %s's admin queue", days, username)
    return tgbotapi.NewMessage(m.Chat.ID, msg)
}

func (h *Handlers) promptForAssignUsername(m *tgbotapi.Message) {
    users, _ := h.Store.ListActiveUsers(ctx)
    // Create inline keyboard with user buttons
    // ... implementation
}
```

### 4.3 Implement /change Command (NEW)
**File:** `internal/telegram/handlers/admin.go`

```go
func (h *Handlers) HandleChange(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
    isAdmin, _ := h.checkAdmin(m.From.ID)
    if !isAdmin {
        return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage)
    }

    args := m.CommandArguments()
    if args == "" {
        return tgbotapi.NewMessage(m.Chat.ID, "Usage: /change <username>")
    }

    // Get today's duty
    now := time.Now()
    berlin, _ := time.LoadLocation("Europe/Berlin")
    today := time.Date(now.In(berlin).Year(), now.In(berlin).Month(), now.In(berlin).Day(), 0, 0, 0, 0, berlin)

    currentDuty, _ := h.Store.GetDutyByDate(ctx, today)
    if currentDuty == nil {
        return tgbotapi.NewMessage(m.Chat.ID, "No duty assigned for today yet")
    }

    // Get new user
    newUser, _ := h.Store.GetUserByName(ctx, args)
    if newUser == nil {
        return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("User %s not found", args))
    }

    oldUser := currentDuty.User

    // Update duty
    currentDuty.UserID = newUser.ID
    h.Store.UpdateDuty(ctx, currentDuty)

    // Send notifications
    // 1. To group
    groupMsg := fmt.Sprintf("🔄 Duty Reassignment\n@%s → @%s", oldUser.FirstName, newUser.FirstName)
    h.sendToGroup(groupMsg)

    // 2. To old user
    oldUserMsg := fmt.Sprintf("You are no longer on duty today. @%s will handle it.", newUser.FirstName)
    h.sendDM(oldUser.TelegramUserID, oldUserMsg)

    // 3. To new user
    newUserMsg := fmt.Sprintf("You are now on duty today (reassigned from @%s)", oldUser.FirstName)
    h.sendDM(newUser.TelegramUserID, newUserMsg)

    return tgbotapi.NewMessage(m.Chat.ID, "✅ Duty reassigned")
}
```

### 4.4 Implement /offduty Command (NEW)
**File:** `internal/telegram/handlers/admin.go`

Multi-step interactive flow:
1. Prompt for username (if not provided)
2. Prompt for start time (Now / Future date)
3. Prompt for number of days
4. Confirm and save

**Requires:** Conversation state management (callback data or session storage)

### 4.5 Update Bot Command Registration
**File:** `internal/telegram/bot.go`

```go
case "volunteer", "v":
    return b.handlers.HandleVolunteer(m)
case "assign", "a":
    return b.handlers.HandleAssign(m)
case "change":
    return b.handlers.HandleChange(m)
case "offduty":
    return b.handlers.HandleOffDuty(m)
```

### 4.6 Remove Old Logic
**Files:**
- Delete `internal/telegram/handlers/volunteer.go` HandleVolunteerCallback (no longer needed)
- Delete calendar interaction from /schedule (read-only)
- Delete /modify command (replaced by /change for today, /assign for future)

---

## Phase 5: Scheduled Jobs 🔴 TODO

### 5.1 Job Scheduler Setup
**File:** `cmd/roster-bot/main.go` or new `internal/jobs/scheduler.go`

Use cron library (e.g., `github.com/robfig/cron/v3`):

```go
import "github.com/robfig/cron/v3"

func setupJobs(store store.Store, bot *telegram.Bot) *cron.Cron {
    berlin, _ := time.LoadLocation("Europe/Berlin")
    c := cron.New(cron.WithLocation(berlin))

    sched := scheduler.NewScheduler(store)

    // 11:00 AM daily - assign duty
    c.AddFunc("0 11 * * *", func() {
        today := time.Now().In(berlin)
        duty, err := sched.AssignDailyDuty(context.Background(), today)
        if err == nil {
            bot.SendDutyNotification(duty)
        }
    })

    // 21:00 PM daily - complete duty
    c.AddFunc("0 21 * * *", func() {
        today := time.Now().In(berlin)
        sched.CompleteDailyDuty(context.Background(), today)
    })

    // Sunday 21:10 PM - weekly stats
    c.AddFunc("10 21 * * 0", func() {
        bot.SendWeeklyStats(context.Background())
    })

    c.Start()
    return c
}
```

**Dependencies to add:**
```bash
go get github.com/robfig/cron/v3
```

### 5.2 Notification Methods
**File:** `internal/telegram/bot.go` or new `internal/telegram/notifications.go`

```go
func (b *Bot) SendDutyNotification(duty *store.Duty) error {
    // Send to user
    userMsg := fmt.Sprintf("🍽️ You are on duty today (%s)!", duty.DutyDate.Format("Jan 2"))
    b.sendDM(duty.User.TelegramUserID, userMsg)

    // Send to group
    groupMsg := fmt.Sprintf("🍽️ Duty Assignment for %s\n@%s is on duty today!",
                            duty.DutyDate.Format("Jan 2"), duty.User.FirstName)
    b.sendToGroup(groupMsg)
}

func (b *Bot) SendWeeklyStats(ctx context.Context) error {
    // Calculate stats for last 7 days
    // Send to DISH_GROUP
}

func (b *Bot) sendDM(telegramUserID int64, text string) error {
    msg := tgbotapi.NewMessage(telegramUserID, text)
    _, err := b.api.Send(msg)
    return err
}

func (b *Bot) sendToGroup(text string) error {
    groupID, _ := strconv.ParseInt(os.Getenv("DISH_GROUP"), 10, 64)
    msg := tgbotapi.NewMessage(groupID, text)
    _, err := b.api.Send(msg)
    return err
}
```

---

## Phase 6: Frontend Updates 🔴 TODO

### 6.1 Web Calendar - Queue Display
**File:** `internal/http/handlers/schedule.go`

Update response to include queue information:

```go
type userQueueInfo struct {
    UserID             int64  `json:"user_id"`
    UserName           string `json:"user_name"`
    VolunteerQueueDays int    `json:"volunteer_queue_days"`
    AdminQueueDays     int    `json:"admin_queue_days"`
}

// In GetSchedule handler:
users, _ := s.ListActiveUsers(ctx)
queueInfo := []userQueueInfo{}
for _, user := range users {
    if user.VolunteerQueueDays > 0 || user.AdminQueueDays > 0 {
        queueInfo = append(queueInfo, userQueueInfo{
            UserID:             user.ID,
            UserName:           user.FirstName,
            VolunteerQueueDays: user.VolunteerQueueDays,
            AdminQueueDays:     user.AdminQueueDays,
        })
    }
}

c.JSON(http.StatusOK, gin.H{
    "duties": response,
    "queues": queueInfo,
})
```

### 6.2 Web Calendar - Frontend Display
**File:** `web/js/ui/calendar.js`

Add queue display below calendar:

```javascript
function renderQueueInfo(queues) {
    const container = document.getElementById('queue-container');
    if (!queues || queues.length === 0) {
        container.innerHTML = '';
        return;
    }

    const html = queues.map(q => `
        <div class="queue-badge">
            <span class="user-name">${q.user_name}</span>
            ${q.volunteer_queue_days > 0 ? `<span class="badge-v">V:${q.volunteer_queue_days}</span>` : ''}
            ${q.admin_queue_days > 0 ? `<span class="badge-a">A:${q.admin_queue_days}</span>` : ''}
        </div>
    `).join('');

    container.innerHTML = `<div class="queue-section"><h3>Queues:</h3>${html}</div>`;
}
```

### 6.3 Telegram Calendar - Queue Display
**File:** `internal/telegram/keyboard/keyboard.go`

Update legend to include queue counts:

```go
// In Calendar function, after building user legend:
for idx, user := range userList {
    // ... existing code ...

    queueInfo := ""
    if user.VolunteerQueueDays > 0 {
        queueInfo += fmt.Sprintf(" V:%d", user.VolunteerQueueDays)
    }
    if user.AdminQueueDays > 0 {
        queueInfo += fmt.Sprintf(" A:%d", user.AdminQueueDays)
    }

    legendEntry := fmt.Sprintf("%s %s%s%s", numberCircle, strings.Join(emojis, ""), user.FirstName, queueInfo)
    // ...
}
```

---

## Phase 7: Cleanup 🔴 TODO

### 7.1 Remove Prognosis Logic
**Files to delete/modify:**
- [ ] Delete `internal/http/handlers/prognosis.go`
- [ ] Remove prognosis route from `internal/http/server.go`
- [ ] Remove prognosis API call from `web/js/api.js`
- [ ] Remove prognosis display from `web/js/ui/calendar.js`

### 7.2 Remove Old Round-Robin Data
**SQL cleanup script:**
```sql
-- Delete all round-robin duties (they're now assigned daily)
DELETE FROM duties WHERE assignment_type = 'round_robin' AND duty_date >= date('now');

-- Reset round-robin state (will be recalculated)
DELETE FROM round_robin_state;
```

---

## Phase 8: Testing Checklist 🔴 TODO

### 8.1 Database Tests
- [ ] Test schema migration on existing database
- [ ] Test queue increment/decrement
- [ ] Test off-duty date filtering
- [ ] Test round-robin 14-day calculation

### 8.2 Command Tests
- [ ] `/volunteer` - adds to queue correctly
- [ ] `/volunteer 5` - adds 5 days to queue
- [ ] `/assign username 3` - adds to admin queue
- [ ] `/change username` - reassigns today's duty
- [ ] `/offduty` - sets off-duty period correctly
- [ ] `/toggleactive` - excludes user from everything

### 8.3 Scheduled Job Tests
- [ ] Manual trigger of 11:00 AM job - assigns correctly
- [ ] Priority order: volunteer > admin > round-robin
- [ ] Queue balancing works with multiple users
- [ ] Off-duty users are excluded
- [ ] Inactive users are excluded
- [ ] 21:00 PM job marks duty completed
- [ ] Weekly stats calculates correctly

### 8.4 Frontend Tests
- [ ] Web calendar shows queue counts
- [ ] Telegram calendar shows queue counts
- [ ] Queue badges display correctly
- [ ] Calendar updates after volunteering

---

## Phase 9: Documentation 🔴 TODO

### 9.1 User Documentation
- [ ] Update `/help` command with new logic
- [ ] Create usage guide for admins
- [ ] Document queue system for users

### 9.2 Developer Documentation
- [ ] Update README with queue system architecture
- [ ] Document scheduled job times
- [ ] Document environment variables (add DISH_GROUP)

---

## Dependencies to Add

```bash
# Cron scheduler
go get github.com/robfig/cron/v3

# Timezone data (if not embedded)
# Should already be available in Go, but verify Berlin timezone works
```

---

## Environment Variables to Add

**File:** `deployments/docker-compose.yml`

```yaml
environment:
  - DISH_GROUP=${DISH_GROUP}  # Telegram group chat ID for announcements
```

**File:** `.env.example` (create if doesn't exist)
```
TELEGRAM_APITOKEN=your_bot_token
ADMIN_ID=your_telegram_user_id
DISH_GROUP=-1001234567890  # Your group chat ID (starts with -100)
DATABASE_PATH=/app/data/roster.db
```

---

## Estimated Effort

| Phase | Complexity | Estimated Time |
|-------|-----------|----------------|
| 1. Database ✅ | Low | 30 min (DONE) |
| 2. Store Layer | Medium | 1-2 hours |
| 3. Scheduler | High | 2-3 hours |
| 4. Commands | High | 2-3 hours |
| 5. Jobs | Medium | 1 hour |
| 6. Frontend | Low-Medium | 1 hour |
| 7. Cleanup | Low | 30 min |
| 8. Testing | Medium | 2 hours |
| 9. Documentation | Low | 30 min |
| **Total** | | **10-13 hours** |

---

## Recommended Approach

### Sprint 1: Core Queue System (3-4 hours)
1. Phase 2: Store layer methods
2. Phase 3: Scheduler rewrite (basic version)
3. Test queue increment/decrement manually

### Sprint 2: Commands (2-3 hours)
1. Phase 4.1: Rewrite /volunteer
2. Phase 4.2: Rewrite /assign
3. Phase 4.3: Implement /change
4. Test commands

### Sprint 3: Automation (2-3 hours)
1. Phase 5: Scheduled jobs
2. Phase 4.4: /offduty command
3. Test daily assignment flow

### Sprint 4: Polish (2-3 hours)
1. Phase 6: Frontend updates
2. Phase 7: Cleanup
3. Phase 8: Full testing
4. Phase 9: Documentation

---

## Risk Areas

⚠️ **High Risk:**
- Scheduler rewrite - completely new logic, easy to introduce bugs
- Scheduled jobs - timezone handling, cron syntax
- Interactive command flows - state management complexity

⚠️ **Medium Risk:**
- Database migrations on production data
- Queue balancing algorithm
- Off-duty date filtering

⚠️ **Low Risk:**
- Frontend queue display
- Cleanup of old code
- Documentation

---

## Next Steps

When ready to continue implementation:

1. **Review this plan** - confirm approach and priorities
2. **Choose sprint** - which phase to tackle first?
3. **Backup database** - before making changes
4. **Start small** - implement one phase, test, commit, repeat

---

## Notes

- All times are Berlin timezone (Europe/Berlin)
- Queue counts are per-user, not global
- Off-duty is temporary, inactive is permanent
- Admin assignments don't count toward round-robin fairness
- Queues are frozen (not cleared) during off-duty periods
