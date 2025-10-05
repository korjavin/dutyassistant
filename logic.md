# Duty Assignment Logic

## Overview
The system manages duty assignments through a queue-based system with three priority levels: **Voluntary**, **Admin**, and **Round-Robin**. Assignments are finalized daily at 11:00 AM Berlin time.

---

## Assignment Types

### 1. Voluntary Queue
Users volunteer for a specific number of days using the `/volunteer` command.

**Command Format:**
- `/volunteer` - Bot prompts for number of days (default: 1 day if no response within ~10 minutes)
- `/volunteer 3` - User volunteers for 3 days directly

**Behavior:**
- Adds the specified number of days to the user's volunteer queue
- Does NOT pre-assign specific calendar dates
- Has **highest priority** when assigning duties
- Cannot change today's assignment (only affects future days)
- Queue count is displayed on web calendar and `/schedule` command per user

**Example:**
- User volunteers for 3 days → Queue: [User: 3 days]
- When assigning tomorrow's duty → Take 1 day from this user's queue → Queue: [User: 2 days]

---

### 2. Admin Assignment Queue
Admin assigns a user to duty for a specific number of days using `/assign`.

**Command Format:**
- `/assign` - Bot prompts for username (with button list) and number of days
- `/assign username` - Bot prompts for number of days
- `/assign username 5` - Directly assigns user to 5 days

**Behavior:**
- Adds the specified number of days to the user's admin assignment queue
- Has **second-highest priority** (after voluntary queue)
- Cannot change today's assignment (only affects future days)
- Queue count is displayed on web calendar and `/schedule` command per user

---

### 3. Round-Robin Assignment
Automatic assignment when no volunteer or admin queue entries exist.

**Selection Criteria:**
- Only considers **active** users (`is_active = 1`)
- Excludes **admin** users (`is_admin = 0`)
- Excludes users who are **off-duty** (see Off-Duty section)
- Calculates fairness based on the **last 14 days** of completed duties
- **Excludes admin-assigned days** from fairness calculation (only counts voluntary and round-robin)

**Calculation:**
- Count completed duties per user in the last 14 days (voluntary + round-robin only)
- Assign to the user with the fewest completed duties
- If tied, use the user who served least recently

---

## Daily Assignment Process

### 11:00 AM Daily Finalization (Berlin Time)
Every day at 11:00 AM, the bot:

1. **Determines today's assignee** using priority order:
   - **Priority 1:** Check volunteer queues - select from user(s) with volunteer queue entries
   - **Priority 2:** Check admin assignment queues - select from user(s) with admin queue entries
   - **Priority 3:** Use round-robin algorithm to select an active user

2. **Queue Balancing:** If multiple users have the same priority queue type:
   - Round-robin between them to distribute fairly
   - Example: UserA has 2 volunteer days, UserB has 2 volunteer days
     - Day 1: UserA (remaining: A=1, B=2)
     - Day 2: UserB (remaining: A=1, B=1)
     - Day 3: UserA (remaining: A=0, B=1)
     - Day 4: UserB (remaining: A=0, B=0)
     - Day 5: Round-robin starts

3. **Send notifications:**
   - Direct message to the assigned user
   - Announcement to the group chat (DISH_GROUP env variable)

**Message Format:**
```
🍽️ Duty Assignment for [Date]
@Username is on duty today!
```

---

### 21:00 PM Daily Completion
Every day at 21:00 PM (Berlin time):

1. **Mark duty as completed** by the assigned user
2. **Record in calendar** with assignment type (voluntary, admin, or round-robin)
3. **Update round-robin statistics** (used for next assignments)

---

## Admin Commands

### `/change <username>` - Reassign Today's Duty
Allows admin to change who is on duty **today** (after 11:00 AM announcement).

**Behavior:**
1. Change today's assignment to the specified user
2. Send notification to **DISH_GROUP**: "Duty reassigned from @OldUser to @NewUser"
3. Send DM to **old assignee**: "You are no longer on duty today"
4. Send DM to **new assignee**: "You are now on duty today"

**Important:** This does NOT affect queues - it's a one-time change for today only.

---

### `/offduty <username>` - Temporary Exclusion
Put a user off-duty for a specified period.

**Command Flow (Interactive Dialog):**
1. Admin: `/offduty` or `/offduty username`
2. Bot prompts for username (if not provided) with button selection
3. Bot prompts: "When should this start?"
   - "Now" button
   - "Future date" button → prompts for date
4. Bot prompts: "For how many days?"
   - Number input (e.g., 5, 12, etc.)

**Behavior During Off-Duty Period:**
- User is **excluded from round-robin** selection
- User's volunteer queue is **frozen** (days remain but aren't used)
- User's admin queue is **frozen** (days remain but aren't used)
- User is marked visibly as "Off-Duty" on calendar

**After Off-Duty Period Ends:**
- User automatically returns to active status
- Queues resume from where they were frozen

---

### `/toggleactive <username>` or `/toggleactive`
Permanently toggle a user between active and inactive status.

**Inactive Users:**
- Completely hidden from:
  - Calendar displays
  - Round-robin selection
  - Admin command suggestions (username buttons)
  - Statistics
- Treated as if they don't exist in the system
- Can be toggled back to active at any time

**Active Users:**
- Visible in all system functions
- Participate in round-robin when not off-duty

---

## User Status Overview

| Status | In Round-Robin? | Queues Active? | Visible in Calendar? | In Stats? |
|--------|-----------------|----------------|----------------------|-----------|
| Active | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |
| Off-Duty (temp) | ❌ No | ⏸️ Frozen | ✅ Yes (marked) | ✅ Yes |
| Inactive (perm) | ❌ No | ❌ No | ❌ No | ❌ No |
| Admin | ❌ No (default) | ✅ Yes | ✅ Yes (if assigned) | ✅ Yes (if assigned) |

---

## Weekly Statistics

### Sunday 21:10 PM - Weekly Report (Berlin Time)

The bot sends a summary to **DISH_GROUP** showing duty statistics for the past week.

**Report Format:**
```
📊 Weekly Duty Report (Oct 29 - Nov 4)

🏆 Duty Days This Week:
• @UserA: 3 days
• @UserB: 2 days
• @UserC: 2 days

Total: 7 duty days completed
```

**Criteria:**
- Only includes users who had **at least 1 completed duty** during the past week
- Counts all assignment types (voluntary, admin, round-robin)
- Sent to the group specified in **DISH_GROUP** environment variable

---

## Environment Variables

- **ADMIN_ID**: Telegram user ID of the admin
- **DISH_GROUP**: Telegram chat ID of the group for announcements
- **DATABASE_PATH**: Path to SQLite database file
- **TELEGRAM_APITOKEN**: Bot API token

---

## Database Schema

### Users Table
```sql
- id (primary key)
- telegram_user_id (unique)
- first_name
- is_admin (boolean) - auto-set if matches ADMIN_ID
- is_active (boolean) - true for regular users, false for admins/inactive
- volunteer_queue_days (integer) - number of days in volunteer queue
- admin_queue_days (integer) - number of days in admin assignment queue
- off_duty_start (date, nullable) - start of off-duty period
- off_duty_end (date, nullable) - end of off-duty period
```

### Duties Table
```sql
- id (primary key)
- user_id (foreign key to users)
- duty_date (date, unique)
- assignment_type (enum: 'voluntary', 'admin', 'round_robin')
- created_at (timestamp)
- completed_at (timestamp, nullable) - set at 21:00 PM
```

### Round-Robin State Table
```sql
- user_id (primary key, foreign key to users)
- last_14_days_count (integer) - completed duties in last 14 days (excl. admin)
- last_duty_date (date, nullable) - most recent duty date
- updated_at (timestamp) - last calculation time
```

---

## Queue Display

### Web Calendar
- Each user with queue entries shows a badge: "👤 UserName (V:3 A:2)"
  - V: Volunteer queue days
  - A: Admin queue days

### Telegram `/schedule`
- Shows current month calendar
- User legend includes queue counts: "① 🟢UserA (V:2)"

---

## Implementation Notes

### Queue Management
- Volunteer and admin queues are **separate counters** per user
- Queues are **decremented by 1** each time a day is assigned from that queue
- Queues **never go negative**
- Multiple users can have queue entries simultaneously

### Timezone
- All time-based operations use **Berlin timezone (Europe/Berlin)**
- Critical times: 11:00 AM (assignment), 21:00 PM (completion)

### Today's Protection
- After 11:00 AM assignment, today's duty is **locked**
- Only `/change` command can modify it
- Volunteer/admin commands only affect future days

### Fairness Algorithm
- Round-robin considers **only the last 14 days**
- Admin assignments **don't count** toward fairness (to avoid penalizing admin-assigned users)
- Off-duty periods **don't count** as duties or penalties

---

## Migration from Current System

**Issues to Address:**
1. Remove prognosis/prediction logic (no longer needed)
2. Add volunteer_queue_days and admin_queue_days columns to users table
3. Add off_duty_start and off_duty_end columns to users table
4. Implement 11:00 AM and 21:00 PM scheduled tasks
5. Add DISH_GROUP environment variable
6. Rewrite assignment logic to use queue system instead of direct calendar assignments
7. Clean up old round-robin duties from database
