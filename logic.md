# Duty Assignment Logic

## Overview
The system manages duty assignments with three types: **Voluntary**, **Admin**, and **Round-Robin**.

## Assignment Types

### 1. Voluntary (`voluntary`)
- User volunteers for a specific date via `/volunteer` command
- **Restrictions:**
  - Cannot volunteer for past dates
  - Can override existing round-robin assignments
  - **Special case:** When volunteering for an admin-assigned day:
    - The admin assignment is moved to the next available day
    - Admin assignment looks for: first unassigned day OR first round-robin day (overrides it)

### 2. Admin Assignment (`admin`)
- Admin manually assigns a user via `/assign <username> <date>`
- **Priority:** Highest - can override any existing assignment
- No date restrictions (can assign to past or future)

### 3. Round-Robin (`round_robin`)
- Automatic assignment to balance workload
- **Current implementation:** Created by prognosis endpoint
- **Selection criteria:**
  - Only active users (`is_active = 1`)
  - Excludes admins (`is_admin = 0`)
  - Orders by: assignment count ASC, last assigned timestamp ASC
  - Takes user with lowest count/oldest assignment

## Current Flow

### Volunteering Flow (`/volunteer`)
1. User clicks `/volunteer` → sees calendar
2. User selects a date
3. System checks:
   - Is date in the past? → **Reject** with error
   - Does duty already exist?
     - If admin assignment → **Reassign admin** to next available day, then create voluntary
     - If voluntary/round-robin → **Override** with new voluntary assignment
     - If none → **Create** new voluntary assignment

### Admin Assignment Flow (`/assign`)
1. Admin runs `/assign <username> <date>`
2. System checks admin authorization (ADMIN_ID from env)
3. Creates/updates duty with `admin` type
4. **Always succeeds** - overrides any existing assignment

### Prognosis/Round-Robin Flow
**CURRENT PROBLEM AREA:**

#### Web Prognosis (`/api/v1/prognosis/:year/:month`)
- **Current behavior:** Shows only next unassigned future day
- Does NOT create duties in database
- Calls `GetNextRoundRobinUser()` to predict

#### Historical Issue (Fixed)
- **Old behavior:** Called `AssignDutyRoundRobin()` for all unassigned days
- This CREATED round-robin duties in the database
- Result: Many unwanted round-robin assignments stored

## Database Schema

### Users Table
```
- id (primary key)
- telegram_user_id (unique)
- first_name
- is_admin (boolean) - set automatically if matches ADMIN_ID env var
- is_active (boolean) - controls participation in round-robin
  - Regular users: true by default
  - Admin users: false by default
```

### Duties Table
```
- id (primary key)
- user_id (foreign key to users)
- duty_date (date)
- assignment_type (enum: 'voluntary', 'admin', 'round_robin')
- created_at (timestamp)
```

### Round-Robin State Table
```
- user_id (primary key, foreign key to users)
- assignment_count (integer) - tracks total assignments
- last_assigned_timestamp (datetime)
```

## Current Problems

### Problem 1: Inactive Admin in Round-Robin
**Status:** Should be fixed by `is_active = 0` and `is_admin = 0` filters
**Symptom:** Admin still appears in rotation despite being inactive
**Possible causes:**
1. Old round-robin duties created before the fix
2. Admin user not properly marked as `is_active = 0`
3. Assignment count increment happening incorrectly

### Problem 2: Round-Robin Assignment Count
**When is count incremented?**
- Only in `AssignDutyRoundRobin()` after successful creation
- **Issue:** If old prognosis created duties, counts were incremented
- **Issue:** If admin reassignment happens, counts might be wrong

### Problem 3: Unclear Assignment Flow
**Questions:**
1. Should round-robin duties be created automatically? If so, when?
2. Should prognosis just predict, or also assign?
3. What happens to assignment counts when duties are overridden?
4. Should there be a background job to auto-assign round-robin?

## Suggested Clarifications Needed

1. **Round-Robin Assignment Strategy:**
   - Option A: Never auto-create, only show prognosis (current after fix)
   - Option B: Auto-create for next N days via background job
   - Option C: Auto-create on-demand when viewing calendar

2. **Assignment Count Management:**
   - Should counts only increment for actual served duties?
   - Should counts decrement if duty is overridden?
   - Should counts track voluntary vs round-robin separately?

3. **Admin Behavior:**
   - Should admin be in users table at all?
   - Should admin have a duty count?
   - Can admin volunteer for duties?

4. **Database Cleanup:**
   - Should old round-robin assignments be deleted?
   - How to handle orphaned assignment counts?

## Display Logic

### Web Calendar
- Shows all duties (voluntary=green, admin=blue, round-robin=white)
- Shows prognosis for next unassigned day (gray/italic)

### Telegram Calendar
- Shows day number + user circle number (e.g., "6①")
- Legend maps numbers to users with emoji indicators
- Read-only in `/schedule`, interactive in `/volunteer`

## Authorization

### Admin Check (`checkAdmin()`)
1. If `AdminID` configured in handlers → compare Telegram user ID
2. Else fallback to `is_admin` flag in database
3. Admin commands: `/assign`, `/modify`, `/users`, `/toggleactive`
