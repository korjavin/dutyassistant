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

## Environment Variables

To run the Duty Assistant Bot, you need to set the following environment variables:

| Variable             | Purpose                               | Required | Default Value        |
| -------------------- | ------------------------------------- | -------- | -------------------- |
| `GIN_MODE`           | The mode for the Gin web framework.   | No       | `debug`              |
| `TELEGRAM_APITOKEN`  | The Telegram Bot API token.           | Yes      |                      |
| `ADMIN_ID`          | Telegram user ID allowed to run admin-only commands and receive daily participant rating prompts. | No       | `0` |
| `DATABASE_PATH`      | The path to the SQLite database file. | No       | `/app/data/roster.db` |
| `DISH_GROUP`        | Telegram chat ID for the main group, used for group duty and month-end participant rating announcements. | No       | `0` |
| `DNS_NAME`           | The DNS name for the web interface.   | No       |                      |
| `OPENAI_API_KEY`     | The OpenAI API key for LLM-refined messages.  | No       |                      |
| `OPENAI_URL`         | The OpenAI API base URL (can be customized).  | No       | `https://api.openai.com/v1` |
| `OPENAI_TIMEOUT_SECONDS` | Timeout in seconds for LLM API calls.     | No       | `10`                 |
| `OPENAI_MODEL`       | The OpenAI model to use for LLM requests.     | No       | `gpt-4o-mini`        |
| `OPENAI_TEMPERATURE` | The temperature parameter for LLM requests.   | No       | `0.7`                |

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

1.  **Install Go**: Make sure you have Go 1.23 or higher installed.
2.  **Install Node.js and npm**: These are required to build the frontend.
3.  **Build the frontend**:

    ```bash
    cd web
    npm install
    npm run build
    cd ..
    ```

4.  **Build the backend**:

    ```bash
    go build -mod=vendor -o roster-bot ./cmd/roster-bot/
    ```

5.  **Run the bot**:

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
- `/list` - View all active periodic chores or regular tasks (`/list chore` or `/list task`).
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
- At 21:00 Europe/Berlin on the last calendar day of the month, the bot posts the monthly participant totals and 1st, 2nd, and 3rd place winners to `DISH_GROUP`.

### Interactive UX

All commands use **inline keyboard buttons** for a friendly user experience:
- **Day selection**: 1-7 buttons in grid layout + "Custom" option
- **User selection**: One button per user with status indicators (✅/❌) or emoji (👤)
- **Date selection**: Today + next 7 days with formatted labels
- **Progressive disclosure**: Commands show relevant options step-by-step
- **Real-time feedback**: Buttons update to show confirmation messages with ✅/❌ indicators

## Queue System

The bot uses a queue-based system with three priority levels:

1. **Volunteer Queue** (Highest Priority)
   - Users add days via `/volunteer` command
   - Interactive button selection (1-7 days or custom amount)
   - Decremented by 1 each day when assigned

2. **Admin Queue** (Second Priority)
   - Admin assigns days via `/assign` command
   - Interactive user and day selection
   - Decremented by 1 each day when assigned

3. **Round-Robin** (Fallback)
   - Automatic when no queue entries exist
   - Based on fairness (last 14 days of completed duties)
   - Excludes admin-assigned duties from fairness calculation
   - Excludes off-duty users

## Automated Tasks

All times in **Europe/Berlin timezone**:

- **11:00 AM Daily** - Assign today's duty based on queue priority and process due periodic chores.
- **21:00 PM Daily** - Mark today's duty as completed
- **20:50 PM Daily** - Send the admin the participant rating prompt when active non-admin participants exist
- **21:00 PM Daily (last calendar day only)** - Publish monthly participant rating winners and totals to the main group
- **21:10 PM Sunday** - Send weekly chore statistics report, including a summary of top overdue chores, top performers (with bar chart visualizations of completed chores, execution times, and lateness), and a "winner of the week".

## Acceptance Verification

Verified on 2026-03-13 for the monthly participant rating flow:

- Stable daily prompt and admin reply parsing are covered by `TestStartDailyRatingsSession_BuildsStablePrompt` and `TestHandleDailyRatingsInteractive_ValidSubmission`, including the `5 2 1`-style score submission flow.
- Same-day resubmission replacement is covered by `TestHandleDailyRatingsInteractive_OverwriteCorrection` and `TestSaveDailyParticipantRatings_CreateAndUpdate`, confirming ratings are overwritten instead of duplicated.
- The month-to-date calendar from day 1 through today is covered by `TestHandleRatingsCalendar_PopulatedMonth` and `TestHandleRatingsCalendar_EmptyMonth`.
- The month-end totals and top-three winner announcement are covered by `TestBuildMonthlyRatingsWinnersAnnouncement_LastDayFormatting` and `TestBuildMonthlyRatingsWinnersAnnouncement_NotLastDaySkips`.
- Full automated validation passed with `go test ./...`.
- No standard project lint command is currently defined in `README.md`, `.github/workflows/ci-cd.yml`, or top-level task/build files, so no separate lint run was available for this verification task.
- Rating-specific automated coverage meets the project target for the new entry points: `PrepareDailyRatingsReminder` 88.2%, `StartDailyRatingsSession` 83.3%, `HandleDailyRatingsInteractive` 92.6%, `HandleRatingsCalendar` 81.2%, `BuildMonthlyRatingsWinnersAnnouncement` 87.5%, and `SaveDailyParticipantRatings` 81.8%.

### Explanation System

The `/explain` command provides transparency into the bot's assignment logic. It shows:
- The assigned user and timestamp of the assignment
- The list of candidates considered
- Users excluded and why (e.g., off-duty, recently assigned)
- The final rule/tie-breaker used for the decision

## Database Schema

See [logic.md](logic.md) for complete database schema and assignment logic details.

### Chore Notifications Configuration
To enable chore notification digests:
- Set `CHORE_TIMEZONE` (default: `Europe/Berlin`) to define when daily 16:00 reports are sent.

### `/explain` - Explain Last Assignment
Explain how the most recent dish hero duty was assigned. Shows the logic behind the choice (e.g., candidate availability, cooldowns, queues, and final criteria).
