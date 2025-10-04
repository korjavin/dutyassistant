# Duty Assistant Bot

Duty Assistant Bot is a Telegram bot designed to help manage on-call duty rosters. It allows users to see the schedule, volunteer for duties, and for admins to assign and modify duties. The bot also provides a web interface for viewing the schedule.

## Features

*   **View Duty Schedule**: Users can view the duty schedule for the current month.
*   **Volunteer for Duties**: Users can volunteer for open duty slots.
*   **Admin-Only Commands**: Admins can assign, modify, and delete duties.
*   **User Management**: Admins can see a list of users and their activation status.
*   **Web Interface**: A web interface to view the duty schedule.

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

The project includes a `Dockerfile` and a `docker-compose.yml` file for easy deployment. The `Dockerfile` creates a minimal production image using a multi-stage build. The `docker-compose.yml` file defines the service and its dependencies.
