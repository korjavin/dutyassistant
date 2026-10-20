# Duty Assistant Bot - AI Knowledge Base

This file contains project knowledge for AI agents and developers working on this codebase.

## Project Structure

- `cmd/roster-bot/` - Main application entry point
- `internal/telegram/handlers/` - Telegram bot command handlers
- `internal/notification/` - Notification system for daily/weekly messages
- `internal/store/` - Database layer (SQLite)
- `internal/http/` - Web interface handlers
- `deployments/` - Docker and deployment configurations

## Rating System

Monthly participant rating contest where admins submit daily scores (1-5) for each participant. The system supports an "ear" modifier: "5e" means a score of 5 with an ear award, indicating exceptional performance beyond the max score. Ears are tracked separately - displayed as "5e" in calendars/summaries and counted per participant in monthly digests.

Key files:
- `internal/telegram/handlers/ratings.go` - Score parsing (including "5e"), calendar display, daily/monthly summaries
- `internal/store/sqlite/ratings.go` - DB queries for saving/loading ratings
- `internal/store/store.go` - `ParticipantDailyRating` (HasEar bool) and `ParticipantMonthlyTotal` (EarCount int) structs

## Bot Commands & Scopes

The bot calls Telegram's `setMyCommands` API at startup to publish autocomplete suggestions. Registration happens in `internal/telegram/bot.go` (`NewBot`), which calls `registerCommands` defined in `internal/telegram/commands.go`. Per-scope failures are logged via `slog.Warn` and do not block startup — autocomplete is a nice-to-have.

Three scopes are registered:

- **All private chats** (`BotCommandScopeAllPrivateChats`) — user commands shown to any user in DM (`userCommands`).
- **Admin's private chat** (`BotCommandScopeChat(adminID)`) — full union of user + admin-only commands (`adminCommands`). Skipped when `adminID == 0`.
- **Main group chat** (`BotCommandScopeChat(groupID)`) — narrow read-only subset (`groupCommands`) to keep group autocomplete clean. Skipped when `groupID == 0`.

Important Telegram rule: scope precedence **replaces** rather than merges. `BotCommandScopeChat(adminID)` overrides `BotCommandScopeAllPrivateChats` for that user, so the admin scope must include user commands too — otherwise the admin would lose autocomplete for `/schedule`, `/volunteer`, etc.

When adding or removing a command, update both the dispatch switch in `internal/telegram/bot.go` (`handleCommand`) and the relevant slice in `internal/telegram/commands.go`. Keep `userHelpMessage` / `adminHelpMessageSection` in `internal/telegram/handlers/commands.go` in sync so `/help` output and Telegram autocomplete agree.

## Sviniya Award System

The bot includes a "sviniya" award system where:
- Monthly rating winner automatically receives 1 sviniya
- Users can view balances via `/sviniya` command
- Users can spend sviniyas via `/spend` command with a description (generates LLM announcement to group)
- Admins can set balances via `/set_sviniya_balance <name> <num>` command

Key files:
- `internal/telegram/handlers/sviniya.go` - Sviniya command handlers
- `internal/store/sqlite/sviniya.go` - Sviniya store implementation

## LLM Integration

`internal/llm` supports two providers behind the same `llm.Client` / `llm.ClientInterface`:
- **OpenAI-compatible** (`NewClient`, raw HTTP to `/chat/completions`) — `OPENAI_*` env vars.
- **Anthropic** (`NewAnthropicClient`, official `anthropic-sdk-go`) — `ANTHROPIC_*` env vars. Default model `claude-opus-5-5`, effort `low`; no temperature is sent (Opus 5.5 rejects sampling params).

`LLM_PROVIDER` picks one explicitly; when empty, `resolveLLMProvider` in `cmd/roster-bot/main.go` chooses Anthropic if `ANTHROPIC_API_KEY` is set, else OpenAI. All LLM calls go through `Client.complete`, so a new LLM feature only needs a prompt and a call to `complete`. Every LLM feature must degrade gracefully (fall back to static text) when the client is nil or the call fails.

**Chore nudges:** the periodic 16:00–19:00 chore reminder (`internal/notification/periodic_chore_reminder.go`) asks `ChoreNudge` for a funny carrot-and-stick message (small vague rewards vs. "telling the admin" / extra duties) built from `DescribeChoreDelays` (time since assignment, overdue time). The factual chore list is always appended deterministically. `notification.ComposeChoreReminder` builds the message (and returns the LLM error alongside the static fallback). Admins can preview it with `/text_candidate_chore <name>` (`internal/telegram/handlers/chore_nudge_preview.go`), which runs the same code path and replies to the admin only, plus the LLM context and provider/model used.

## Admin Assistant (free-text admin requests)

When the Anthropic provider is configured, the admin can write to the bot in free text (any language) in a private chat instead of using /commands. Claude parses the intent and acts through tools.

- `internal/assistant/agent.go` — `Agent`: manual Messages API tool-use loop, one conversation per chat (kept 30 min), at most 8 model calls per message. Thinking blocks are preserved by appending `resp.ToParam()` (append-only history).
- `internal/assistant/tools.go` — tool registry. **Read tools** (`list_users`, `list_active_chores`, `list_recurring_chores`, `list_upcoming_duties`, `get_sviniya_balances`, `get_chore_stats`, `get_ratings`) run immediately. **Data-changing tools** only *prepare* an action (validate input, resolve user names case-insensitively); the admin gets a summary with the exact command plus ✅ Confirm / ❌ Cancel buttons (callback data `aa:y:<id>` / `aa:n:<id>`, valid 15 min). Nothing changes until Confirm.
- Data-changing tools run the bot's **own admin commands** (e.g. `/chore desc /Ivan /3d`, `/assign`, `/modify`, `/offduty`, `/vacation on`) or the existing button callbacks (`complete_chore:<reminderID>`, `cancel_assignment_confirm:A<id>`) through `internal/telegram/assistant.go` (`assistantExecutor`), so validation, DMs and group announcements are identical to manual use. To add a capability: add a `toolDef` that builds a command line, and make sure the command accepts all its arguments non-interactively.
- Routing (`internal/telegram/bot.go`): non-command text from the admin's private chat with no active interactive session goes to the assistant (in a goroutine; the update loop is not blocked). Sessions keep priority. Without Anthropic, the admin gets a hint instead.
- Env: `ANTHROPIC_ASSISTANT_EFFORT` (default `medium`).

## Message Formatting Patterns

### Channel Messages vs DM Messages

The bot uses different message formats for channel announcements vs direct messages:

**Channel messages**: Use concise one-liner formats to reduce channel noise
- Assignment: `🎯 <b>%s</b>: <i>%s</i>` (name, description)
- Completion: `✅ <b>%s</b> completed: <i>%s</i>` (name, description)
- Daily summary headers: `⚠️ Overdue chores:`, `🔴 Critical (3+d):`, `🟠 Overdue (1-2d):`, `🟢 Due today:`
- Weekly stats: `N. Name — X done` (one line per user)

**DM messages**: Can be more verbose and friendly with full sentences
- Initial DM: Multi-line with congratulations and instructions
- Reminders: Friendly reminders with action buttons

### HTML Escaping

- All user-provided content must be escaped using `html.EscapeString()` at display time
- Store descriptions unescaped in database to prevent double-escaping
- Use `tgbotapi.ModeHTML` parse mode for formatted messages
- Example from `chore_reminder.go`:
  ```go
  escapedName := html.EscapeString(assignment.UserName)
  escapedDesc := html.EscapeString(assignment.Description)
  msg := fmt.Sprintf("✅ <b>%s</b> completed: <i>%s</i>", escapedName, escapedDesc)
  ```

## Testing Message Formats

### Test Pattern

- Use table-driven tests for message format verification
- Test both positive cases (correct format present) and negative cases (old format not present)
- Use `assert.Contains()` and `assert.NotContains()` for format validation
- Mock HTTP client to capture Telegram API requests and verify payload content

### Mocking Telegram API

The project uses a custom `NewTestClient` function to mock HTTP requests:

```go
client := NewTestClient(func(req *http.Request) *http.Response {
    if strings.Contains(req.URL.String(), "getMe") {
        return &http.Response{
            StatusCode: 200,
            Body: io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"id": 123456}}`)),
            Header: make(http.Header),
        }
    }
    if strings.Contains(req.URL.String(), "sendMessage") {
        // Capture the request body for verification
        bodyBytes, _ := io.ReadAll(req.Body)
        form, _ := url.ParseQuery(string(bodyBytes))
        sendMessageForm = form
        return &http.Response{
            StatusCode: 200,
            Body: io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"message_id": 1}}`)),
            Header: make(http.Header),
        }
    }
    return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(`{}`))}
})
bot, _ := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, client)
```

### Error Path Testing

Always test error paths for functions that interact with external services:
- Nil/unconfigured bot
- Zero/invalid GroupID
- Network failures (403, 500 errors)
- Verify critical behavior (e.g., assignment not stored after failed DM)

## Build and Test Commands

### Running Tests
```bash
go test ./...                    # Run all tests
go test ./internal/telegram/...  # Run tests for specific package
go test -v ./...                 # Run with verbose output
```

### Building
```bash
go build -o roster-bot ./cmd/roster-bot/
```

### Linting
```bash
go vet ./...
```

## Database Schema

The bot uses SQLite with the following key tables:
- `users` - User registry with active/inactive status
- `duty_queue` - Duty assignments and queue management
- `chores` - One-off and recurring chores
- `chore_assignments` - Active chore assignments
- `participant_ratings` - Daily participant scores with optional ear modifier (`has_ear` column)
- `sviniya_balances` - Sviniya award balances (user_id PK, balance INTEGER DEFAULT 0)

## Timezone Handling

All scheduled operations use Europe/Berlin timezone:
- Daily assignment: 07:30 AM
- Daily completion: 21:00 PM
- Weekly stats: Sunday 21:10 PM
- Rating prompts: Daily 20:50 PM

## Security Considerations

- All user input is HTML-escaped before display
- HTTP timeout protections are configured
- Rate limiting is implemented for HTTP endpoints
- Security headers (CSP, X-Frame-Options) are set
- HMAC authentication is used for HTTP API

## Deployment

The project uses GitHub Actions for CI/CD:
1. Builds Docker image on push to master
2. Pushes to GitHub Container Registry
3. Triggers Portainer webhook for deployment
