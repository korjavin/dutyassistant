# Duty Assistant Bot

Duty Assistant Bot is a Telegram bot designed to help manage on-call duty rosters using a queue-based assignment system. It features an interactive inline keyboard UI for all commands and provides both Telegram and web interfaces.

## Features

*   **Queue-Based Assignment System**: Three-tier priority system (Volunteer → Admin → Round-Robin)
*   **Interactive UI**: All commands use inline keyboard buttons for easy interaction
*   **Automated Daily Assignments**: Automatic duty assignment at 11:00 AM Berlin time
*   **Duty Completion Tracking**: Automatic completion marking at 21:00 PM Berlin time
*   **Volunteer System**: Users can volunteer for duty days using interactive buttons
*   **Admin Commands**: Full duty management with button-based UX
*   **Off-Duty Periods**: Temporary exclusion from duty rotation with queue freezing
*   **User Management**: Toggle active/inactive status via buttons
*   **Weekly Statistics**: Automated weekly reports every Sunday at 21:10 PM
*   **Monthly Participant Ratings**: Daily admin scoring prompt, month-to-date rating calendar, and month-end winners announcement
*   **Web Interface**: View duty schedule and queue status in browser

## Bot Commands

### User Commands
- `/start` - Register with the bot
- `/help` - Show available commands
- `/status` - View your duty statistics and queue status
- `/schedule` - View the current month's duty schedule
- `/volunteer` - Volunteer for duty (shows interactive day selection buttons)
- `/chore` - View your currently assigned active chores
- `/explain` - Explain how the most recent dish hero duty was assigned

### Admin Commands
- `/assign` - Assign days to a user's admin queue (interactive user + days selection)
- `/chore <description> [/<N>d]` - Assign a one-off chore to a random user or make it periodic every `N` days.
- `/list` - View all active periodic chores or regular tasks (interactive list type selection, or `/list chore` / `/list task`).
- `/cancel` - Cancel a duty, active chore, or recurring chore (interactive item selection).
- `/complete` - Mark any active chore as completed (interactive chore selection).
- `/modify` or `/change` - Change duty assignment for a date (interactive date + user selection)
- `/offduty` - Set off-duty period for a user (interactive user selection, text date input)
- `/toggleactive` - Toggle user active/inactive status (interactive user selection with status indicators)
- `/unassign` - Remove days from a user's admin queue (interactive user + days selection)
- `/vacation [on|off]` - Toggle vacation mode to pause all duty assignments (interactive button UI when no argument provided)
- `/users` - List all users with their queues and status
- `/ratings` - Show the current month's participant rating calendar

### Participant Rating Flow

- Every day at 20:50 Europe/Berlin, the bot sends the configured admin a participant rating prompt when there are active non-admin participants to score.
- The prompt lists participants in a stable order. Reply with one space-separated score per participant, using integers from 1 to 5.
- Sending another reply later on the same day overwrites that day's participant ratings instead of creating duplicates.
- `/ratings` shows the current month from day 1 through today, with missing scores displayed as `-`.
- At 21:00 Europe/Berlin on the last calendar day of the month, the bot posts the monthly participant totals and 1st, 2nd, and 3rd place winners to the group.

### Interactive UX

All commands use **inline keyboard buttons** for a friendly user experience:
- **Day selection**: 1-7 buttons in grid layout + "Custom" option
- **User selection**: One button per user with status indicators (✅/❌) or emoji (👤)
- **Date selection**: Today + next 7 days with formatted labels
- **Progressive disclosure**: Commands show relevant options step-by-step
- **Real-time feedback**: Buttons update to show confirmation messages with ✅/❌ indicators

## Duty Assignment Logic

The system manages duty assignments through a queue-based system with three priority levels: **Voluntary**, **Admin**, and **Round-Robin**. Assignments are finalized daily at 11:00 AM Berlin time.

### 1. Voluntary Queue
Users volunteer for a specific number of days using the `/volunteer` command.

**Command Format:**
- `/volunteer` - Shows **inline keyboard** with day selection buttons (1-7 + Custom)
  - Buttons: `[1] [2] [3] [4]` on first row, `[5] [6] [7]` on second row, `[✏️ Custom]` on third row
  - Clicking a number button immediately adds that many days to volunteer queue
  - Clicking "Custom" prompts: "Please type the number of days: /volunteer [days]"
- `/volunteer 3` - Direct text input also supported (adds 3 days directly)

**Behavior:**
- Adds the specified number of days to the user's volunteer queue
- Does NOT pre-assign specific calendar dates
- Has **highest priority** when assigning duties
- Cannot change today's assignment (only affects future days)
- Queue count is displayed on web calendar and `/schedule` command per user
- Confirmation message shows with ✅ emoji

### 2. Admin Assignment Queue
Admin assigns a user to duty for a specific number of days using `/assign`.

**Command Format:**
- `/assign` - Shows **inline keyboard** with user selection buttons
  - Step 1: Shows list of users as buttons: `[👤 UserA]` `[👤 UserB]` etc.
  - Step 2: After user selection, shows day buttons: `[1] [2] [3] [4]` `[5] [6] [7]` `[✏️ Custom]`
  - Step 3: After day selection, shows confirmation and executes assignment
- `/assign username 5` - Direct text input also supported

**Behavior:**
- Adds the specified number of days to the user's admin assignment queue
- Has **second-highest priority** (after voluntary queue)
- Cannot change today's assignment (only affects future days)
- Queue count is displayed on web calendar and `/schedule` command per user
- Confirmation message shows with ✅ emoji

### 3. Round-Robin Assignment
Automatic assignment when no volunteer or admin queue entries exist.

**Selection Criteria:**
- Only considers **active** users
- Excludes **admin** users
- Excludes users who are **off-duty**
- Calculates fairness based on the **last 14 days** of completed duties
- **Excludes admin-assigned days** from fairness calculation (only counts voluntary and round-robin)

**Calculation:**
- Count completed duties per user in the last 14 days (voluntary + round-robin only)
- Assign to the user with the fewest completed duties
- If tied, use the user who served least recently

## Daily Assignment Process

### 11:00 AM Daily Finalization (Berlin Time)
Every day at 11:00 AM, the bot:

1. **Determines today's assignee** using priority order:
   - **Priority 1:** Check volunteer queues - select from user(s) with volunteer queue entries
   - **Priority 2:** Check admin assignment queues - select from user(s) with admin queue entries
   - **Priority 3:** Use round-robin algorithm to select an active user

2. **Queue Balancing:** If multiple users have the same priority queue type:
   - Round-robin between them to distribute fairly

3. **Send notifications:**
   - Direct message to the assigned user
   - Announcement to the group chat

### 21:00 PM Daily Completion
Every day at 21:00 PM (Berlin time):

1. **Mark duty as completed** by the assigned user
2. **Record in calendar** with assignment type (voluntary, admin, or round-robin)
3. **Update round-robin statistics** (used for next assignments)

## Off-Duty and Active Status

### Temporary Exclusion (`/offduty`)
Put a user off-duty for a specified period.

**Behavior During Off-Duty Period:**
- User is **excluded from round-robin** selection
- User's volunteer queue is **frozen** (days remain but aren't used)
- User's admin queue is **frozen** (days remain but aren't used)
- User is marked visibly as "Off-Duty" on calendar

**After Off-Duty Period Ends:**
- User automatically returns to active status
- Queues resume from where they were frozen

### Toggle User Active Status (`/toggleactive`)
Permanently toggle a user between active and inactive status.

**Inactive Users:**
- Completely hidden from calendar displays, round-robin selection, admin command suggestions, and statistics
- Treated as if they don't exist in the system
- Can be toggled back to active at any time

## User Status Overview

| Status | In Round-Robin? | Queues Active? | Visible in Calendar? | In Stats? |
|--------|-----------------|----------------|----------------------|-----------|
| Active | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |
| Off-Duty (temp) | ❌ No | ⏸️ Frozen | ✅ Yes (marked) | ✅ Yes |
| Inactive (perm) | ❌ No | ❌ No | ❌ No | ❌ No |
| Admin | ❌ No (default) | ✅ Yes | ✅ Yes (if assigned) | ✅ Yes (if assigned) |

## Weekly Statistics

### Sunday 21:10 PM - Weekly Report (Berlin Time)
The bot sends a summary showing duty statistics for the past week.
Only includes users who had **at least 1 completed duty** during the past week. Counts all assignment types (voluntary, admin, round-robin).

## Contribution

For developers, technical contributors, and AI agents, please refer to the detailed **[Agent.md](Agent.md)**. It contains:
- Architecture Design
- Environment Variables setup
- Docker and Source build instructions
- Deployment (CI/CD)
- Database Schema details
