package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// deps is what a tool needs to read data or prepare an action for one admin chat.
type deps struct {
	store    store.Store
	executor Executor
	chatID   int64
	adminID  int64
	now      time.Time
	loc      *time.Location
}

// preparedAction is a validated data-changing action waiting for the admin's
// confirmation. Summary is HTML shown on the confirmation message.
type preparedAction struct {
	Summary string
	Execute func(ctx context.Context) (Result, error)
	// NoOp, when set, means nothing needs to change; it is returned to the
	// model as the tool result and no confirmation is asked.
	NoOp string
}

// toolDef describes one tool exposed to the model. Exactly one of read or
// prepare is set: read tools run immediately, prepare tools need confirmation.
type toolDef struct {
	name        string
	description string
	properties  map[string]any
	required    []string
	read        func(ctx context.Context, d deps, input []byte) (string, error)
	prepare     func(ctx context.Context, d deps, input []byte) (*preparedAction, error)
}

func (t toolDef) mutating() bool { return t.prepare != nil }

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

func integer(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

const dateLayout = "2006-01-02"

// tools is the registry of everything the admin assistant can do. Data-changing
// tools run the bot's own admin commands, so validation, notifications and
// group announcements behave exactly as when the admin types the command.
var tools = []toolDef{
	{
		name:        "list_users",
		description: "List all users with active/inactive status, admin and volunteer queue days, off-duty periods, upcoming duties, and whether vacation mode is on. Use it to resolve the exact user names other tools need.",
		read: func(ctx context.Context, d deps, _ []byte) (string, error) {
			return d.runText(ctx, "/users")
		},
	},
	{
		name:        "list_active_chores",
		description: "List currently assigned (not yet completed) one-off chores with their chore_id, assignee, assignment time and deadline.",
		read:        readActiveChores,
	},
	{
		name:        "list_recurring_chores",
		description: "List active recurring (periodic) chores with their recurring_chore_id, interval, fixed assignee (if any) and next run time.",
		read: func(ctx context.Context, d deps, _ []byte) (string, error) {
			return d.runText(ctx, "/list chore")
		},
	},
	{
		name:        "list_upcoming_duties",
		description: "List the daily duty assignments already scheduled from today onwards.",
		properties: map[string]any{
			"days": integer("How many days ahead to include (1-60, default 14)."),
		},
		read: readUpcomingDuties,
	},
	{
		name:        "get_sviniya_balances",
		description: "Show every user's sviniya (award) balance.",
		read: func(ctx context.Context, d deps, _ []byte) (string, error) {
			return d.runText(ctx, "/sviniya")
		},
	},
	{
		name:        "get_chore_stats",
		description: "Show the most overdue chores and the users who completed the most chores.",
		read: func(ctx context.Context, d deps, _ []byte) (string, error) {
			return d.runText(ctx, "/chore_stats")
		},
	},
	{
		name:        "get_ratings",
		description: "Show this month's participant rating calendar (daily scores 1-5; 'e' marks an ear award).",
		read: func(ctx context.Context, d deps, _ []byte) (string, error) {
			return d.runText(ctx, "/ratings")
		},
	},
	{
		name: "assign_chore",
		description: "Create a chore. Without repeat_every_days it is a one-off chore assigned now (to assignee_name, or to a random active user when omitted); " +
			"the assignee gets a DM and the group is notified. With repeat_every_days it becomes a recurring chore.",
		properties: map[string]any{
			"description":       str("What has to be done, e.g. 'Take out the trash'."),
			"assignee_name":     str("Exact user name to assign to. Omit for a random user."),
			"repeat_every_days": integer("Make it recurring every N days (1-365). Omit for a one-off chore."),
		},
		required: []string{"description"},
		prepare:  prepareAssignChore,
	},
	{
		name:        "complete_chore",
		description: "Mark an active one-off chore as completed on the assignee's behalf (the group is notified).",
		properties: map[string]any{
			"chore_id": integer("chore_id from list_active_chores."),
		},
		required: []string{"chore_id"},
		prepare:  prepareCompleteChore,
	},
	{
		name:        "cancel_chore",
		description: "Cancel (delete) an active one-off chore without completing it.",
		properties: map[string]any{
			"chore_id": integer("chore_id from list_active_chores."),
		},
		required: []string{"chore_id"},
		prepare:  prepareCancelChore,
	},
	{
		name:        "cancel_recurring_chore",
		description: "Stop a recurring chore so no new occurrences are created.",
		properties: map[string]any{
			"recurring_chore_id": integer("recurring_chore_id from list_recurring_chores."),
		},
		required: []string{"recurring_chore_id"},
		prepare:  prepareCancelRecurringChore,
	},
	{
		name:        "add_admin_queue_days",
		description: "Force a user to take the daily duty for the next N assignment days (adds days to their admin queue).",
		properties: map[string]any{
			"user_name": str("Exact user name."),
			"days":      integer("Number of days to add (1-30)."),
		},
		required: []string{"user_name", "days"},
		prepare: func(ctx context.Context, d deps, input []byte) (*preparedAction, error) {
			return prepareUserDays(ctx, d, input, "/assign", 1, 30, "Add <b>%d</b> admin-queue duty day(s) to <b>%s</b>")
		},
	},
	{
		name:        "remove_admin_queue_days",
		description: "Remove days from a user's admin queue.",
		properties: map[string]any{
			"user_name": str("Exact user name."),
			"days":      integer("Number of days to remove (1-30)."),
		},
		required: []string{"user_name", "days"},
		prepare: func(ctx context.Context, d deps, input []byte) (*preparedAction, error) {
			return prepareUserDays(ctx, d, input, "/unassign", 1, 30, "Remove <b>%d</b> admin-queue day(s) from <b>%s</b>")
		},
	},
	{
		name:        "set_volunteer_queue",
		description: "Set how many upcoming duty days a user has volunteered for (0-7).",
		properties: map[string]any{
			"user_name": str("Exact user name."),
			"days":      integer("Volunteer queue length (0-7)."),
		},
		required: []string{"user_name", "days"},
		prepare: func(ctx context.Context, d deps, input []byte) (*preparedAction, error) {
			return prepareUserDays(ctx, d, input, "/set_volunteer", 0, 7, "Set volunteer queue to <b>%d</b> day(s) for <b>%s</b>")
		},
	},
	{
		name:        "set_sviniya_balance",
		description: "Set a user's sviniya (award) balance to an exact number.",
		properties: map[string]any{
			"user_name": str("Exact user name."),
			"balance":   integer("New balance (0-1000)."),
		},
		required: []string{"user_name", "balance"},
		prepare:  prepareSetSviniya,
	},
	{
		name:        "change_duty_assignee",
		description: "Change who has the daily duty on a given date (today or future).",
		properties: map[string]any{
			"date":      str("Date in YYYY-MM-DD."),
			"user_name": str("Exact user name of the new assignee."),
		},
		required: []string{"date", "user_name"},
		prepare:  prepareChangeDuty,
	},
	{
		name:        "set_off_duty",
		description: "Mark a user as off-duty (excluded from duty assignment) for a date range, inclusive.",
		properties: map[string]any{
			"user_name":  str("Exact user name."),
			"start_date": str("First off-duty day, YYYY-MM-DD."),
			"end_date":   str("Last off-duty day, YYYY-MM-DD."),
		},
		required: []string{"user_name", "start_date", "end_date"},
		prepare:  prepareOffDuty,
	},
	{
		name:        "set_user_active",
		description: "Include (active=true) or exclude (active=false) a user from the duty rotation and chores.",
		properties: map[string]any{
			"user_name": str("Exact user name."),
			"active":    boolean("Whether the user should be active."),
		},
		required: []string{"user_name", "active"},
		prepare:  prepareSetActive,
	},
	{
		name:        "set_vacation_mode",
		description: "Turn household vacation mode on or off. While on, no daily duties are assigned.",
		properties: map[string]any{
			"enabled": boolean("true to turn vacation mode on, false to turn it off."),
		},
		required: []string{"enabled"},
		prepare:  prepareVacation,
	},
}

func findTool(name string) (toolDef, bool) {
	for _, t := range tools {
		if t.name == name {
			return t, true
		}
	}
	return toolDef{}, false
}

// runText runs a read-only bot command and returns its text.
func (d deps) runText(ctx context.Context, commandLine string) (string, error) {
	res, err := d.executor.RunCommand(ctx, d.chatID, d.adminID, commandLine)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

// commandAction builds a confirmed action that runs a bot command.
func (d deps) commandAction(summary, commandLine string) *preparedAction {
	return &preparedAction{
		Summary: summary + "\n   <code>" + html.EscapeString(commandLine) + "</code>",
		Execute: func(ctx context.Context) (Result, error) {
			return d.executor.RunCommand(ctx, d.chatID, d.adminID, commandLine)
		},
	}
}

// callbackAction builds a confirmed action that triggers a bot button callback.
func (d deps) callbackAction(summary, data string) *preparedAction {
	return &preparedAction{
		Summary: summary,
		Execute: func(ctx context.Context) (Result, error) {
			return d.executor.RunCallback(ctx, d.chatID, d.adminID, data)
		},
	}
}

func decode(input []byte, v any) error {
	if len(input) == 0 {
		input = []byte("{}")
	}
	if err := json.Unmarshal(input, v); err != nil {
		return fmt.Errorf("invalid tool input: %w", err)
	}
	return nil
}

// resolveUser finds a user by name, case-insensitively, so commands get the
// exact stored name. The error lists known users to help the model retry.
func resolveUser(ctx context.Context, s store.Store, name string) (*store.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("user_name is required")
	}
	users, err := s.ListAllUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not list users: %w", err)
	}
	names := make([]string, 0, len(users))
	for _, u := range users {
		if strings.EqualFold(u.FirstName, name) {
			if strings.ContainsAny(u.FirstName, " \t\n") {
				return nil, fmt.Errorf("user %q has whitespace in their name and cannot be addressed by bot commands", u.FirstName)
			}
			return u, nil
		}
		names = append(names, u.FirstName)
	}
	return nil, fmt.Errorf("no user named %q; known users: %s", name, strings.Join(names, ", "))
}

func readActiveChores(ctx context.Context, d deps, _ []byte) (string, error) {
	chores, err := d.store.GetActiveChores(ctx)
	if err != nil {
		return "", err
	}
	if len(chores) == 0 {
		return "No active chores.", nil
	}
	var sb strings.Builder
	for _, c := range chores {
		assignee := "unassigned"
		if c.User != nil {
			assignee = c.User.FirstName
		}
		fmt.Fprintf(&sb, "chore_id=%d | %s | %q | assigned %s", c.ID, assignee, c.Description, c.AssignedAt.In(d.loc).Format("2006-01-02 15:04"))
		if !c.DeadlineAt.IsZero() {
			fmt.Fprintf(&sb, " | deadline %s", c.DeadlineAt.In(d.loc).Format("2006-01-02 15:04"))
			if d.now.After(c.DeadlineAt) {
				sb.WriteString(" (OVERDUE)")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

func readUpcomingDuties(ctx context.Context, d deps, input []byte) (string, error) {
	var in struct {
		Days int `json:"days"`
	}
	if err := decode(input, &in); err != nil {
		return "", err
	}
	if in.Days <= 0 {
		in.Days = 14
	}
	if in.Days > 60 {
		in.Days = 60
	}
	today := time.Date(d.now.Year(), d.now.Month(), d.now.Day(), 0, 0, 0, 0, d.loc)
	duties, err := d.store.GetFutureDuties(ctx, today)
	if err != nil {
		return "", err
	}
	until := today.AddDate(0, 0, in.Days)
	var sb strings.Builder
	for _, duty := range duties {
		if duty.DutyDate.After(until) {
			continue
		}
		who := fmt.Sprintf("user #%d", duty.UserID)
		if duty.User != nil {
			who = duty.User.FirstName
		}
		fmt.Fprintf(&sb, "%s | %s | %s\n", duty.DutyDate.Format(dateLayout), who, duty.AssignmentType)
	}
	if sb.Len() == 0 {
		return fmt.Sprintf("No duties scheduled in the next %d days.", in.Days), nil
	}
	return sb.String(), nil
}

func prepareAssignChore(ctx context.Context, d deps, input []byte) (*preparedAction, error) {
	var in struct {
		Description     string `json:"description"`
		AssigneeName    string `json:"assignee_name"`
		RepeatEveryDays int    `json:"repeat_every_days"`
	}
	if err := decode(input, &in); err != nil {
		return nil, err
	}
	desc := strings.Join(strings.Fields(in.Description), " ")
	if desc == "" {
		return nil, fmt.Errorf("description is required")
	}
	for _, word := range strings.Fields(desc) {
		if strings.HasPrefix(word, "/") {
			return nil, fmt.Errorf("description must not contain words starting with '/'")
		}
	}
	if in.RepeatEveryDays < 0 || in.RepeatEveryDays > 365 {
		return nil, fmt.Errorf("repeat_every_days must be between 1 and 365")
	}

	line := "/chore " + desc
	summary := fmt.Sprintf("Assign one-off chore <i>%s</i>", html.EscapeString(desc))
	if in.RepeatEveryDays > 0 {
		summary = fmt.Sprintf("Create recurring chore <i>%s</i> every <b>%d</b> day(s)", html.EscapeString(desc), in.RepeatEveryDays)
	}
	if strings.TrimSpace(in.AssigneeName) != "" {
		u, err := resolveUser(ctx, d.store, in.AssigneeName)
		if err != nil {
			return nil, err
		}
		line += " /" + u.FirstName
		summary += fmt.Sprintf(" to <b>%s</b>", html.EscapeString(u.FirstName))
	} else {
		summary += " to a <b>random</b> user"
	}
	if in.RepeatEveryDays > 0 {
		line += fmt.Sprintf(" /%dd", in.RepeatEveryDays)
	}
	return d.commandAction(summary, line), nil
}

// findActiveChore looks up an active one-off chore by ID.
func findActiveChore(ctx context.Context, d deps, input []byte) (*store.Chore, error) {
	var in struct {
		ChoreID int64 `json:"chore_id"`
	}
	if err := decode(input, &in); err != nil {
		return nil, err
	}
	chores, err := d.store.GetActiveChores(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range chores {
		if c.ID == in.ChoreID {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no active chore with chore_id=%d; call list_active_chores", in.ChoreID)
}

func choreLabel(c *store.Chore) string {
	who := "unassigned"
	if c.User != nil {
		who = c.User.FirstName
	}
	return fmt.Sprintf("chore #%d <i>%s</i> (%s)", c.ID, html.EscapeString(c.Description), html.EscapeString(who))
}

func prepareCompleteChore(ctx context.Context, d deps, input []byte) (*preparedAction, error) {
	c, err := findActiveChore(ctx, d, input)
	if err != nil {
		return nil, err
	}
	if c.ReminderID == "" {
		return nil, fmt.Errorf("chore #%d has no reminder id and cannot be completed from here", c.ID)
	}
	return d.callbackAction("Mark "+choreLabel(c)+" as <b>completed</b>", "complete_chore:"+c.ReminderID), nil
}

func prepareCancelChore(ctx context.Context, d deps, input []byte) (*preparedAction, error) {
	c, err := findActiveChore(ctx, d, input)
	if err != nil {
		return nil, err
	}
	return d.callbackAction("<b>Cancel</b> "+choreLabel(c), fmt.Sprintf("cancel_assignment_confirm:A%d", c.ID)), nil
}

func prepareCancelRecurringChore(ctx context.Context, d deps, input []byte) (*preparedAction, error) {
	var in struct {
		ID int64 `json:"recurring_chore_id"`
	}
	if err := decode(input, &in); err != nil {
		return nil, err
	}
	rc, err := d.store.GetRecurringChore(ctx, in.ID)
	if err != nil || rc == nil || !rc.IsActive {
		return nil, fmt.Errorf("no active recurring chore with recurring_chore_id=%d; call list_recurring_chores", in.ID)
	}
	summary := fmt.Sprintf("<b>Stop</b> recurring chore #%d <i>%s</i> (every %d day(s))", rc.ID, html.EscapeString(rc.Description), rc.Interval)
	return d.commandAction(summary, fmt.Sprintf("/cancel chore %d", rc.ID)), nil
}

func prepareUserDays(ctx context.Context, d deps, input []byte, command string, minDays, maxDays int, summaryFmt string) (*preparedAction, error) {
	var in struct {
		UserName string `json:"user_name"`
		Days     int    `json:"days"`
	}
	if err := decode(input, &in); err != nil {
		return nil, err
	}
	if in.Days < minDays || in.Days > maxDays {
		return nil, fmt.Errorf("days must be between %d and %d", minDays, maxDays)
	}
	u, err := resolveUser(ctx, d.store, in.UserName)
	if err != nil {
		return nil, err
	}
	summary := fmt.Sprintf(summaryFmt, in.Days, html.EscapeString(u.FirstName))
	return d.commandAction(summary, fmt.Sprintf("%s %s %d", command, u.FirstName, in.Days)), nil
}

func prepareSetSviniya(ctx context.Context, d deps, input []byte) (*preparedAction, error) {
	var in struct {
		UserName string `json:"user_name"`
		Balance  int    `json:"balance"`
	}
	if err := decode(input, &in); err != nil {
		return nil, err
	}
	if in.Balance < 0 || in.Balance > 1000 {
		return nil, fmt.Errorf("balance must be between 0 and 1000")
	}
	u, err := resolveUser(ctx, d.store, in.UserName)
	if err != nil {
		return nil, err
	}
	summary := fmt.Sprintf("Set sviniya balance of <b>%s</b> to <b>%d</b>", html.EscapeString(u.FirstName), in.Balance)
	return d.commandAction(summary, fmt.Sprintf("/set_sviniya_balance %s %d", u.FirstName, in.Balance)), nil
}

func parseDate(field, value string) (time.Time, error) {
	t, err := time.Parse(dateLayout, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD, got %q", field, value)
	}
	return t, nil
}

func prepareChangeDuty(ctx context.Context, d deps, input []byte) (*preparedAction, error) {
	var in struct {
		Date     string `json:"date"`
		UserName string `json:"user_name"`
	}
	if err := decode(input, &in); err != nil {
		return nil, err
	}
	date, err := parseDate("date", in.Date)
	if err != nil {
		return nil, err
	}
	u, err := resolveUser(ctx, d.store, in.UserName)
	if err != nil {
		return nil, err
	}
	day := date.Format(dateLayout)
	summary := fmt.Sprintf("Give the duty on <b>%s</b> to <b>%s</b>", day, html.EscapeString(u.FirstName))
	return d.commandAction(summary, fmt.Sprintf("/modify %s %s", day, u.FirstName)), nil
}

func prepareOffDuty(ctx context.Context, d deps, input []byte) (*preparedAction, error) {
	var in struct {
		UserName  string `json:"user_name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := decode(input, &in); err != nil {
		return nil, err
	}
	start, err := parseDate("start_date", in.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := parseDate("end_date", in.EndDate)
	if err != nil {
		return nil, err
	}
	if end.Before(start) {
		return nil, fmt.Errorf("end_date must not be before start_date")
	}
	u, err := resolveUser(ctx, d.store, in.UserName)
	if err != nil {
		return nil, err
	}
	s, e := start.Format(dateLayout), end.Format(dateLayout)
	summary := fmt.Sprintf("Mark <b>%s</b> off-duty from <b>%s</b> to <b>%s</b>", html.EscapeString(u.FirstName), s, e)
	return d.commandAction(summary, fmt.Sprintf("/offduty %s %s %s", u.FirstName, s, e)), nil
}

func prepareSetActive(ctx context.Context, d deps, input []byte) (*preparedAction, error) {
	var in struct {
		UserName string `json:"user_name"`
		Active   bool   `json:"active"`
	}
	if err := decode(input, &in); err != nil {
		return nil, err
	}
	u, err := resolveUser(ctx, d.store, in.UserName)
	if err != nil {
		return nil, err
	}
	state := "inactive"
	if in.Active {
		state = "active"
	}
	if u.IsActive == in.Active {
		return &preparedAction{NoOp: fmt.Sprintf("%s is already %s; nothing to change.", u.FirstName, state)}, nil
	}
	summary := fmt.Sprintf("Make <b>%s</b> <b>%s</b>", html.EscapeString(u.FirstName), state)
	// /toggle_active flips the flag; the check above makes the flip land on the requested state.
	return d.commandAction(summary, "/toggle_active "+u.FirstName), nil
}

func prepareVacation(_ context.Context, d deps, input []byte) (*preparedAction, error) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := decode(input, &in); err != nil {
		return nil, err
	}
	if in.Enabled {
		return d.commandAction("Turn vacation mode <b>ON</b> (no duties will be assigned)", "/vacation on"), nil
	}
	return d.commandAction("Turn vacation mode <b>OFF</b>", "/vacation off"), nil
}
