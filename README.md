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
*   **Web Interface**: View duty schedule and queue status in browser

## Environment Variables

To run the Duty Assistant Bot, you need to set the following environment variables:

| Variable             | Purpose                               | Required | Default Value        |
| -------------------- | ------------------------------------- | -------- | -------------------- |
| `GIN_MODE`           | The mode for the Gin web framework.   | No       | `debug`              |
| `TELEGRAM_APITOKEN`  | The Telegram Bot API token.           | Yes      |                      |
| `DATABASE_PATH`      | The path to the SQLite database file. | No       | `/app/data/roster.db` |
| `DNS_NAME`           | The DNS name for the web interface.   | No       |                      |

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

### Admin Commands
- `/assign` - Assign days to a user's admin queue (interactive user + days selection)
- `/modify` or `/change` - Change duty assignment for a date (interactive date + user selection)
- `/offduty` - Set off-duty period for a user (interactive user selection, text date input)
- `/toggleactive` - Toggle user active/inactive status (interactive user selection with status indicators)
- `/users` - List all users with their queues and status

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

- **11:00 AM Daily** - Assign today's duty based on queue priority
- **21:00 PM Daily** - Mark today's duty as completed
- **21:10 PM Sunday** - Send weekly duty statistics report (TODO: implement)

## Database Schema

See [logic.md](logic.md) for complete database schema and assignment logic details.
