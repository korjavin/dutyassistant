# Duty Assistant Bot

Duty Assistant Bot is a Telegram bot designed to help manage on-call duty rosters using a queue-based assignment system. It features an interactive inline keyboard UI for all commands and provides both Telegram and web interfaces.

## Features

*   **Queue-Based Assignment System**: Three-tier priority system (Volunteer → Admin → Round-Robin)
*   **Security Best Practices**: Includes HTTP timeout protections, rate-limiting, comprehensive security headers (CSP, X-Frame-Options), and safe DOM rendering.
*   **Interactive UI**: All commands use inline keyboard buttons for easy interaction
*   **Automated Daily Assignments**: Automatic duty assignment at 07:30 AM Berlin time
*   **Duty Completion Tracking**: Automatic completion marking at 21:00 PM Berlin time
*   **Volunteer System**: Users can volunteer for duty days using interactive buttons
*   **Admin Commands**: Full duty management with button-based UX
*   **Off-Duty Periods**: Temporary exclusion from duty rotation with queue freezing
*   **User Management**: Toggle active/inactive status via buttons
*   **Weekly Statistics**: Automated weekly reports every Sunday at 21:10 PM
*   **Monthly Participant Ratings**: Daily admin scoring prompt with optional "5e" ear award, month-to-date rating calendar, and month-end winners announcement with ear counts
*   **Sviniya Award System**: Monthly rating winner automatically receives 1 sviniya, users can spend sviniyas with LLM-generated announcements, admins can manage balances
*   **Web Interface**: View duty schedule and queue status in browser

## Environment Variables

To run the Duty Assistant Bot, you need to set the following environment variables:

| Variable             | Purpose                               | Required | Default Value        |
| -------------------- | ------------------------------------- | -------- | -------------------- |
| `GIN_MODE`           | The mode for the Gin web framework.   | No       | `debug`              |
| `TELEGRAM_APITOKEN`  | The Telegram Bot API token.           | Yes      |                      |
| `ADMIN_ID`          | Telegram user ID allowed to run admin-only commands and receive the daily reports to rate. | No       | `0` |
| `DATABASE_PATH`      | The path to the SQLite database file. | No       | `/app/data/roster.db` |
| `DISH_GROUP`        | Telegram chat ID for the main group, used for group duty and month-end participant rating announcements. | No       | `0` |
| `DNS_NAME`           | The DNS name for the web interface.   | No       |                      |
| `LLM_PROVIDER`       | LLM backend: `openai` (any OpenAI-compatible API) or `anthropic`. Empty = auto: `anthropic` if `ANTHROPIC_API_KEY` is set, otherwise `openai`. | No | auto |
| `ANTHROPIC_API_KEY`  | The Anthropic API key for LLM-refined messages. | No       |                      |
| `ANTHROPIC_BASE_URL` | The Anthropic API base URL (can be customized). | No       | `https://api.anthropic.com` |
| `ANTHROPIC_TIMEOUT_SECONDS` | Timeout in seconds for Anthropic API calls. | No  | `30`                 |
| `ANTHROPIC_MODEL`    | The Claude model to use for LLM requests.     | No       | `claude-opus-5-5`    |
| `ANTHROPIC_EFFORT`   | Claude effort level: `low`, `medium`, `high`, `xhigh`, `max`. Temperature is not used for Anthropic. | No | `low` |
| `ANTHROPIC_ASSISTANT_EFFORT` | Effort for the free-text assistants (tool use): `low`..`max`. | No | `medium` |
| `PARTICIPANT_ASSISTANT_DAILY_LIMIT` | Max free-text messages per participant per day for the participant assistant; `0` turns it off. | No | `20` |
| `OPENAI_API_KEY`     | The OpenAI API key for LLM-refined messages.  | No       |                      |
| `OPENAI_URL`         | The OpenAI API base URL (can be customized).  | No       | `https://api.openai.com/v1` |
| `OPENAI_TIMEOUT_SECONDS` | Timeout in seconds for LLM API calls.     | No       | `10`                 |
| `OPENAI_MODEL`       | The OpenAI model to use for LLM requests.     | No       | `gpt-4o-mini`        |
| `OPENAI_TEMPERATURE` | The temperature parameter for LLM requests.   | No       | `0.7`                |
| `TRANSCRIPTION_API_KEY` | API key for speech-to-text of voice daily reports (any OpenAI-compatible `/audio/transcriptions`: OpenAI, Groq, local Whisper). Empty = reuse `OPENAI_API_KEY`; no key at all = text-only reports. | No | |
| `TRANSCRIPTION_URL`  | Base URL of the speech-to-text API. Empty = `OPENAI_URL` when reusing the OpenAI key, else OpenAI. | No | `https://api.openai.com/v1` |
| `TRANSCRIPTION_MODEL` | Speech-to-text model.                        | No       | `whisper-1`          |
| `TRANSCRIPTION_LANGUAGE` | Optional ISO-639-1 language hint, e.g. `ru`. Empty = auto-detect. | No | |
| `TRANSCRIPTION_TIMEOUT_SECONDS` | Timeout for speech-to-text calls.  | No       | `60`                 |

## Free-text Assistant

With the Anthropic provider configured (`ANTHROPIC_API_KEY`), the admin can simply write to the bot in a private chat, in any language, instead of using commands, e.g. *"Пусть Ваня завтра дежурит, а Маше дай 2 свиньи"*. The bot (Claude with tool use) looks up users, chores and duties itself, then shows the planned actions with the exact commands and ✅ Confirm / ❌ Cancel buttons. Nothing is changed until Confirm. Interactive command flows keep working as before.

Participants can chat with the bot privately too, with their own limited set of abilities: ask about their chores and duties, mark their own chore as done, take over someone else's chore, volunteer for duty, or spend a sviniya — always with Confirm/Cancel buttons and only on their own behalf. `PARTICIPANT_ASSISTANT_DAILY_LIMIT` (default 20 messages per person per day) keeps costs in check; set it to `0` to turn participants off.

## Voice Messages and Photos

The bot reads everything it gets the way it reads text:

- **Voice messages, audio files and video notes** are transcribed (`TRANSCRIPTION_*`, or `OPENAI_API_KEY`); the bot replies "🎧 I heard: «…»".
- **Photos and image files** are described by Claude (Anthropic provider), including any text visible in them; the bot replies "🖼 I see: …". A caption is read too.

The result is then handled exactly like the same text typed by the sender: an answer in an interactive flow, a request to the admin or participant assistant, or a part of the daily report. Sending a voice message is also a quick check that the transcription key works. While the assistant waits for ✅ Confirm / ❌ Cancel, a short reply such as "да, подтверждаю" or "нет, не надо" (spoken or typed) works like the button.

In the group the bot only reacts to messages addressed to it: an `@mention` (in text or a caption) or a **reply to one of the bot's messages** — typed, voice or photo. Other group messages are ignored and never transcribed.

## Daily Reports (merits and failures)

Daily ratings come only from these reports. At 20:00 every rating participant gets a DM asking for today's report of merits and failures. They can answer with voice messages (one or several) or text, and press ✅ Done. `/report` opens a report at any time during the day. The LLM reads the report together with what the bot knows (duty, completed and overdue chores) and proposes a score from 1 (behaved badly) to 5 (excellent day) with a short analysis. The admin gets it with ✅ Confirm N and a row of score buttons `1 2 3 4 5 5👂`: Confirm saves the proposal, a score button saves another score, and `5👂` gives 5 with an ear. The buttons stay, so a score can be changed later (until the month's results are out). Once everyone has a score for the day the usual daily summary is posted to the group (again after a change).

At 20:45 the bot sends the admin every report still open: started but never sent ones as they are, and for participants who told nothing a "no report" assessment (the LLM judges the day from the facts alone, or proposes 3 without an LLM), so everything can be confirmed before 21:00. Without an LLM a sent report comes without a proposal and the admin picks a score.

## Running with Docker

The recommended way to run the Duty Assistant Bot is with Docker and Docker Compose.

1.  **Create a `.env` file** with the following content:

    ```
    TELEGRAM_APITOKEN=your_telegram_bot_token
    DNS_NAME=your_dns_name
    ```

2.  **Run the bot**:

    ```bash
    docker-compose -f deployments/docker-compose.yml up -d
    ```

## Building from Source

You can also build the bot from source.

1.  **Install Go**: Make sure you have Go 1.26.1 or higher installed.
2.  **Build the backend**:

    ```bash
    go build -o roster-bot ./cmd/roster-bot/
    ```

3.  **Run the bot**:

    ```bash
    GIN_MODE=release TELEGRAM_APITOKEN=your_telegram_bot_token DATABASE_PATH=./roster.db ./roster-bot
    ```

## Deployment

The project includes a `Dockerfile` and a `docker-compose.yml` file for easy deployment. The `Dockerfile` creates a minimal production image using a multi-stage build with Alpine Linux (includes `tzdata` for Berlin timezone support). The `docker-compose.yml` file defines the service and its dependencies.

### CI/CD

The project uses GitHub Actions for automated builds and deployments. On push to master, the workflow:
1. Builds a Docker image
2. Pushes to GitHub Container Registry
3. Triggers Portainer webhook to deploy on production server
## Bot Commands

### Channel Message Formats

Channel announcements use concise one-liner formats to reduce noise:

- Chore assignments: `🎯 <b>User</b>: <i>Description</i>`
- Chore completions: `✅ <b>User</b> completed: <i>Description</i>`
- Overdue report (admin `/overdue` on demand): `⚠️ Overdue chores:` with compact category headers
- Weekly stats: `N. Name — X done` (one line per user)

DM messages remain verbose and friendly with full sentences.

### User Commands
- `/start` - Register with the bot
- `/help` - Show available commands
- `/status` - View your duty statistics and queue status
- `/schedule` - View the current month's duty schedule
- `/volunteer` - Volunteer for duty (shows interactive day selection buttons)
- `/chore` - View your currently assigned active chores
- `/explain` - Explain how the most recent dish hero duty was assigned
- `/sviniya` - View all user sviniya balances
- `/spend [description]` - Spend 1 sviniya with a description (generates LLM announcement to group)

### Admin Commands
- `/assign` - Assign days to a user's admin queue (interactive user + days selection)
- `/chore <description> [/<name>] [/<N>d]` - Assign a one-off chore to a random user, or to a specific active user with a trailing `/<name>` (first name, case-insensitive; use `_` for spaces, e.g. `/chore Fix the door /Anna_Maria`). Add `/<N>d` to make it periodic every `N` days; combined with `/<name>` (e.g. `/chore Water plants /Alice /3d`) every occurrence goes to that user, otherwise each occurrence picks a random user. Interactive mode (`/chore` → Create) accepts the same suffixes.
- `/chore translate <id>` - Translate a recurring chore's description to English (admin only).
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
- `/set_sviniya_balance <name> <num>` - Set a user's sviniya balance to a specific number (admin only)

### Participant Rating Flow

- Scores are given with the buttons under each participant's daily report (see Daily Reports): integers from 1 to 5, or `5👂` to award an ear for exceptional performance beyond the max score — the score counts as 5, but the ear is tracked and displayed separately.
- Pressing another score later overwrites that day's rating instead of creating a duplicate.
- `/ratings` shows the current month from day 1 through today, with missing scores displayed as `-` and ear-awarded scores displayed as `5e`.
- At 21:00 Europe/Berlin on the last calendar day of the month, the bot posts the monthly participant totals and 1st, 2nd, and 3rd place winners to the group. The 1st place winner is automatically granted 1 sviniya.

### Interactive UX

All commands use **inline keyboard buttons** for a friendly user experience:
- **Day selection**: 1-7 buttons in grid layout + "Custom" option
- **User selection**: One button per user with status indicators (✅/❌) or emoji (👤)
- **Date selection**: Today + next 7 days with formatted labels
- **Progressive disclosure**: Commands show relevant options step-by-step
- **Real-time feedback**: Buttons update to show confirmation messages with ✅/❌ indicators

## Duty Assignment Logic

The system manages duty assignments through a queue-based system with three priority levels: **Voluntary**, **Admin**, and **Round-Robin**. Assignments are finalized daily at 07:30 AM Berlin time.

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

### 07:30 AM Daily Finalization (Berlin Time)
Every day at 07:30 AM, the bot:

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
