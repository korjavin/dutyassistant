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

## Daily Reports (merits and failures)

At 20:40 Berlin (cron in `main.go`, 10 min before the admin's 20:50 rating prompt) `SendDailyReportRequests` DMs every rating participant (same set as `filterRatingParticipants`) asking for today's "report of merits and failures". Code: `internal/telegram/handlers/daily_report.go`; table `daily_reports` (one row per participant per day; status `collecting` → `submitted` → `proposed` → `approved`/`rejected`).
- Routing (`bot.go` → `handleMessage`, no active session): `HandleDailyReportMessage` takes a participant's **voice / audio / video note** in a private chat at any time (opens today's report if needed) and **text only while the report is `collecting`**; anything else falls through to the assistant. `/report` opens the report manually.
- Voice is downloaded from Telegram and transcribed by `llm.Transcriber` (OpenAI-compatible `/audio/transcriptions`, `TRANSCRIPTION_*` env, falls back to `OPENAI_API_KEY`/`OPENAI_URL`): the Claude API has no audio input. Transcription runs in a goroutine, serialized per report; the participant sees "I heard: …".
- ✅ Done (`rep_done:<id>`, owner only) → `submitted` → `assessReport`: `llm.Client.AssessDailyReport` (prompt + `complete`, JSON `{score, analysis}`) gets the report plus facts (duty, chores completed today, pending/overdue chores). The admin gets the analysis with ✅ Confirm N / ❌ Reject (`rep_ok:<id>` / `rep_no:<id>`, admin only). Without an LLM (or on error) the report is sent unscored, no buttons.
- Confirm saves only that participant's score (`SaveParticipantRating`, unlike `SaveDailyParticipantRatings` which replaces the whole day) and, once every participant is rated for the day, posts the usual daily + monthly summary to the group. Confirm is refused after 21:00 on the month's last day (winners already announced).
- 21:30 cron `FinalizeDailyReports` submits reports with content whose authors never pressed Done.

## Registration (admin approval)

Who can talk to the bot at all is decided by `checkAccess` in `internal/telegram/bot.go`: the owner (`ADMIN_ID`) and members of `DISH_GROUP` (everyone, if no group is configured). Group messages that are not addressed to the bot (no command, no mention, not the sender's own session) are dropped before the access check (`addressedToBot`), so chatter never gets an "Access denied" reply. Messages from inside `DISH_GROUP` need no `getChatMember` lookup. When Telegram upgrades the group to a supergroup (new ID, e.g. after making the bot admin), the admin is DMed the new ID to put in `DISH_GROUP` (`handleGroupMigration` / `reportGroupMigration`), and an admin mention in a chat that isn't `DISH_GROUP` is answered with that chat's ID. On top of that, `/start` from an unknown Telegram user does **not** create a user: `HandleStart` → `requestRegistration` (`internal/telegram/handlers/registration.go`) stores a `registration_requests` row (`pending`) and DMs the admin the person's name, @username and Telegram ID with ✅ Approve / ❌ Reject buttons (`reg_ok:<tgID>` / `reg_no:<tgID>`, admin-only).
- Approve creates the user as an active participant and DMs them the welcome; reject marks the request `rejected` and the person is told so on later /start (the admin is not notified again).
- Repeated /start while pending only re-notifies the admin after 1 hour.
- The configured admin is created directly; with `ADMIN_ID` unset, users are created directly as before (nobody could approve).

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

## Recurring and Scheduled Chores

`recurring_chores` holds both periodic chores and one-off chores scheduled for later (`interval_days = 0`). `ProcessRecurringChores` (`internal/telegram/handlers/chore_cron.go`) runs **every minute** from the cron in `main.go` and assigns everything whose `next_run_at` has passed (DM + group notice), so a chore is assigned at its exact time. A one-off is deactivated after its run; a periodic chore with `ends_at` is deactivated once its next run would be after `ends_at` (`RecurringChore.RunsLeft` counts the remaining runs).

`/chore` accepts two extra suffixes, kept out of `/help` and used by the admin assistant (`assign_chore` `start_at` / `end_date`):
- `/from=YYYY-MM-DD_HH:MM` — assign at that time instead of now (first run of a recurring chore). A bare date means 10:00.
- `/until=YYYY-MM-DD` — recurring only; last day an occurrence may run (inclusive).

e.g. `/chore Air the cellar /Vladimir /1d /from=2026-10-21_12:00 /until=2026-10-30` → daily at 12:00, 10 times. Times are in `CHORE_TIMEZONE`.

## Free-text Assistants (admin and participants)

When the Anthropic provider is configured, people can write to the bot in free text (any language) in a private chat instead of using /commands. Claude parses the intent and acts through tools. There are two profiles (`assistant.AdminProfile()` / `assistant.ParticipantProfile()`), each with its own tools, system prompt and button prefix (`aa` / `ap`):

- **Admin** (`adminTools` in `tools.go`): manages the whole household (see below).
- **Participant** (`participantTools` in `participant_tools.go`): acts only as themselves — read own status/chores, chores they could take, upcoming duties, sviniya balances, explain last duty; with confirmation: complete own chore (`chore_done:<reminderID>`), take someone's chore (`vc_confirm:<id>`), `/volunteer N`, `/spend <desc>`. Tools run with the participant's Telegram ID, and the bot's handlers check ownership again. Never add admin data-changing tools here (a test enforces it). Limited by `PARTICIPANT_ASSISTANT_DAILY_LIMIT` messages per person per day (default 20; `0` disables participants). Unregistered users are asked to /start.

- `internal/assistant/agent.go` — `Agent`: manual Messages API tool-use loop, one conversation per chat (kept 30 min), at most 8 model calls per message, optional daily message limit. Thinking blocks are preserved by appending `resp.ToParam()` (append-only history).
- `internal/assistant/tools.go` — admin tool registry. **Read tools** (`list_users`, `list_active_chores`, `list_recurring_chores`, `list_upcoming_duties`, `get_sviniya_balances`, `get_chore_stats`, `get_ratings`) run immediately. **Data-changing tools** only *prepare* an action (validate input, resolve user names case-insensitively); the admin gets a summary with the exact command plus ✅ Confirm / ❌ Cancel buttons (callback data `aa:y:<id>` / `aa:n:<id>`, valid 15 min). Nothing changes until Confirm.
- Data-changing tools run the bot's **own commands** (e.g. `/chore desc /Ivan /3d`, `/assign`, `/modify`, `/offduty`, `/vacation on`) or the existing button callbacks (`complete_chore:<reminderID>`, `cancel_assignment_confirm:A<id>`) through `internal/telegram/assistant.go` (`assistantExecutor`), so validation, DMs and group announcements are identical to manual use. To add a capability: add a `toolDef` that builds a command line, and make sure the command accepts all its arguments non-interactively.
- Routing (`internal/telegram/bot.go` → `handleAssistantMessage`): non-command text in a private chat with no active interactive session (or a group mention, below) goes to the admin or participant assistant (in a goroutine; the update loop is not blocked). Sessions keep priority. Without Anthropic, the admin gets a hint and participants are ignored as before.
- **Group mentions** (`isGroupMention` / `stripMentions` in `internal/telegram/assistant.go`): a message in `DISH_GROUP` that mentions the bot (`@username`, case-insensitive, or a text mention of its account) also goes to the assistant, chosen by **who wrote it** — the admin gets the admin profile, everyone else the participant profile (unregistered → asked to /start). The mention is stripped and the model sees `(Group chat message from <name>; everyone in the group sees your reply.)`. Answers reply to the person's message. Mentions in other chats are ignored; a group session only takes priority for its own owner. Telegram delivers mentions to the bot only if privacy mode allows it (otherwise disable privacy mode in BotFather).
- Conversations are kept per **(chat, person)**, so people in the group don't share history; the daily limit is per person across chats. Confirm/Cancel buttons in the group react only to the person who asked (`Agent.PendingOwner`); others' taps are ignored and the buttons stay.
- Env: `ANTHROPIC_ASSISTANT_EFFORT` (default `medium`), `PARTICIPANT_ASSISTANT_DAILY_LIMIT` (default `20`).

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
- `daily_reports` - Participants' evening reports (text/transcripts), status, LLM-proposed score and analysis (UNIQUE user_id, report_date)
- `registration_requests` - Pending/rejected `/start` requests awaiting admin approval (telegram_user_id PK, names, status, requested_at)

## Timezone Handling

All scheduled operations use Europe/Berlin timezone:
- Daily assignment: 07:30 AM
- Daily completion: 21:00 PM
- Weekly stats: Sunday 21:10 PM
- Daily report requests: 20:40 PM (unsent reports forwarded at 21:30)
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
