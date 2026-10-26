package assistant

import (
	"context"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func toolDeps(t *testing.T) (deps, *mocks.MockStore, *fakeExecutor) {
	t.Helper()
	st := new(mocks.MockStore)
	st.On("ListAllUsers", mock.Anything).Return([]*store.User{
		{ID: 1, FirstName: "Ivan", IsActive: true},
		{ID: 2, FirstName: "Maria", IsActive: false},
	}, nil)
	ex := &fakeExecutor{reply: map[string]Result{}}
	return deps{
		store: st, executor: ex, chatID: 1, userID: 1,
		now: time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC), loc: time.UTC,
	}, st, ex
}

// prepareCommand prepares a tool call and returns the command it would run.
func prepareCommand(t *testing.T, d deps, ex *fakeExecutor, name, input string) (*preparedAction, string) {
	t.Helper()
	tool, ok := findTool(adminTools, name)
	require.True(t, ok, name)
	require.True(t, tool.mutating(), name)
	action, err := tool.prepare(context.Background(), d, []byte(input))
	require.NoError(t, err, name)
	if action.NoOp != "" {
		return action, ""
	}
	before := len(ex.commands)
	_, err = action.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, ex.commands, before+1, "action must run exactly one command")
	return action, ex.commands[before]
}

func TestTools_CommandsBuilt(t *testing.T) {
	d, _, ex := toolDeps(t)
	cases := []struct {
		tool, input, command string
	}{
		{"assign_chore", `{"description":"Take out trash"}`, "/chore Take out trash"},
		{"assign_chore", `{"description":"Feed cat","assignee_name":"IVAN"}`, "/chore Feed cat /Ivan"},
		{"assign_chore", `{"description":"Water plants","repeat_every_days":3}`, "/chore Water plants /3d"},
		{"assign_chore", `{"description":"Water plants","assignee_name":"maria","repeat_every_days":7}`, "/chore Water plants /Maria /7d"},
		{"assign_chore", `{"description":"Air the cellar","assignee_name":"Ivan","start_at":"2026-10-21 12:00"}`, "/chore Air the cellar /Ivan /from=2026-10-21_12:00"},
		{"assign_chore", `{"description":"Air the cellar","assignee_name":"Ivan","repeat_every_days":1,"start_at":"2026-10-21T12:00","end_date":"2026-10-30"}`, "/chore Air the cellar /Ivan /1d /from=2026-10-21_12:00 /until=2026-10-30"},
		{"assign_chore", `{"description":"Water plants","repeat_every_days":2,"end_date":"2026-10-30"}`, "/chore Water plants /2d /until=2026-10-30"},
		{"add_admin_queue_days", `{"user_name":"ivan","days":2}`, "/assign Ivan 2"},
		{"remove_admin_queue_days", `{"user_name":"Ivan","days":1}`, "/unassign Ivan 1"},
		{"set_volunteer_queue", `{"user_name":"Ivan","days":0}`, "/set_volunteer Ivan 0"},
		{"set_sviniya_balance", `{"user_name":"maria","balance":5}`, "/set_sviniya_balance Maria 5"},
		{"change_duty_assignee", `{"date":"2026-10-21","user_name":"Maria"}`, "/modify 2026-10-21 Maria"},
		{"set_off_duty", `{"user_name":"Ivan","start_date":"2026-10-21","end_date":"2026-10-25"}`, "/offduty Ivan 2026-10-21 2026-10-25"},
		{"set_user_active", `{"user_name":"Maria","active":true}`, "/toggle_active Maria"},
		{"set_vacation_mode", `{"enabled":true}`, "/vacation on"},
		{"set_vacation_mode", `{"enabled":false}`, "/vacation off"},
	}
	for _, c := range cases {
		t.Run(c.tool+" "+c.input, func(t *testing.T) {
			action, cmd := prepareCommand(t, d, ex, c.tool, c.input)
			assert.Equal(t, c.command, cmd)
			assert.Contains(t, action.Summary, "<code>"+c.command+"</code>")
		})
	}
}

func TestTools_AssignChoreScheduleSummary(t *testing.T) {
	d, _, ex := toolDeps(t)

	action, _ := prepareCommand(t, d, ex, "assign_chore",
		`{"description":"Air the cellar","assignee_name":"Ivan","repeat_every_days":1,"start_at":"2026-10-21 12:00","end_date":"2026-10-30"}`)
	assert.Contains(t, action.Summary, "first on <b>Wed 2026-10-21 12:00</b>")
	assert.Contains(t, action.Summary, "last by <b>Fri 2026-10-30</b> (<b>10</b> times)")

	action, _ = prepareCommand(t, d, ex, "assign_chore", `{"description":"Air the cellar","start_at":"2026-10-21 12:00"}`)
	assert.Contains(t, action.Summary, "Schedule one-off chore")
	assert.Contains(t, action.Summary, "on <b>Wed 2026-10-21 12:00</b>")
}

func TestTools_ValidationErrors(t *testing.T) {
	d, _, _ := toolDeps(t)
	cases := []struct {
		tool, input, err string
	}{
		{"assign_chore", `{"description":"  "}`, "description is required"},
		{"assign_chore", `{"description":"Clean /Ivan"}`, "must not contain words starting with '/'"},
		{"assign_chore", `{"description":"x","repeat_every_days":400}`, "between 1 and 365"},
		{"assign_chore", `{"description":"x","assignee_name":"Petya"}`, `no user named "Petya"`},
		{"assign_chore", `{"description":"x","start_at":"2026-10-20 17:00"}`, "not in the future"},
		{"assign_chore", `{"description":"x","start_at":"21.10.2026 12:00"}`, "YYYY-MM-DD HH:MM"},
		{"assign_chore", `{"description":"x","end_date":"2026-10-30"}`, "only applies to recurring chores"},
		{"assign_chore", `{"description":"x","repeat_every_days":1,"end_date":"30.10.2026"}`, "end_date must be YYYY-MM-DD"},
		{"assign_chore", `{"description":"x","repeat_every_days":1,"start_at":"2026-10-25 12:00","end_date":"2026-10-24"}`, "before the first occurrence"},
		{"assign_chore", `{"description":"x","repeat_every_days":1,"end_date":"2026-10-19"}`, "before the first occurrence"},
		{"add_admin_queue_days", `{"user_name":"Ivan","days":0}`, "between 1 and 30"},
		{"set_volunteer_queue", `{"user_name":"Ivan","days":8}`, "between 0 and 7"},
		{"set_sviniya_balance", `{"user_name":"Ivan","balance":-1}`, "between 0 and 1000"},
		{"change_duty_assignee", `{"date":"21.10.2026","user_name":"Ivan"}`, "YYYY-MM-DD"},
		{"set_off_duty", `{"user_name":"Ivan","start_date":"2026-10-25","end_date":"2026-10-21"}`, "must not be before"},
		{"set_sviniya_balance", `not json`, "invalid tool input"},
	}
	for _, c := range cases {
		t.Run(c.tool+" "+c.input, func(t *testing.T) {
			tool, _ := findTool(adminTools, c.tool)
			_, err := tool.prepare(context.Background(), d, []byte(c.input))
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.err)
		})
	}
}

func TestTools_SetUserActiveNoOp(t *testing.T) {
	d, _, ex := toolDeps(t)
	action, cmd := prepareCommand(t, d, ex, "set_user_active", `{"user_name":"Ivan","active":true}`)
	assert.Empty(t, cmd)
	assert.Equal(t, "Ivan is already active; nothing to change.", action.NoOp)
}

func TestTools_ChoreCallbacks(t *testing.T) {
	d, st, ex := toolDeps(t)
	st.On("GetActiveChores", mock.Anything).Return([]*store.Chore{
		{ID: 7, Description: "Dishes <now>", ReminderID: "r7", User: &store.User{FirstName: "Ivan"}},
	}, nil)

	complete, _ := findTool(adminTools, "complete_chore")
	action, err := complete.prepare(context.Background(), d, []byte(`{"chore_id":7}`))
	require.NoError(t, err)
	assert.Contains(t, action.Summary, "chore #7 <i>Dishes &lt;now&gt;</i> (Ivan)")
	_, _ = action.Execute(context.Background())

	cancel, _ := findTool(adminTools, "cancel_chore")
	action, err = cancel.prepare(context.Background(), d, []byte(`{"chore_id":7}`))
	require.NoError(t, err)
	_, _ = action.Execute(context.Background())

	assert.Equal(t, []string{"complete_chore:r7", "cancel_assignment_confirm:A7"}, ex.callbacks)

	_, err = complete.prepare(context.Background(), d, []byte(`{"chore_id":99}`))
	assert.ErrorContains(t, err, "no active chore with chore_id=99")
}

func TestTools_CancelRecurringChore(t *testing.T) {
	d, st, ex := toolDeps(t)
	st.On("GetRecurringChore", mock.Anything, int64(3)).Return(&store.RecurringChore{ID: 3, Description: "Vacuum", Interval: 7, IsActive: true}, nil)
	st.On("GetRecurringChore", mock.Anything, int64(4)).Return(&store.RecurringChore{ID: 4, IsActive: false}, nil)

	action, cmd := prepareCommand(t, d, ex, "cancel_recurring_chore", `{"recurring_chore_id":3}`)
	assert.Equal(t, "/cancel chore 3", cmd)
	assert.Contains(t, action.Summary, "<i>Vacuum</i> (every 7 day(s))")

	tool, _ := findTool(adminTools, "cancel_recurring_chore")
	_, err := tool.prepare(context.Background(), d, []byte(`{"recurring_chore_id":4}`))
	assert.ErrorContains(t, err, "no active recurring chore")
}

func TestTools_ReadActiveChoresAndDuties(t *testing.T) {
	d, st, _ := toolDeps(t)
	st.On("GetActiveChores", mock.Anything).Return([]*store.Chore{{
		ID: 5, Description: "Trash", User: &store.User{FirstName: "Ivan"},
		AssignedAt: d.now.Add(-48 * time.Hour), DeadlineAt: d.now.Add(-24 * time.Hour),
	}}, nil)
	st.On("GetFutureDuties", mock.Anything, mock.Anything).Return([]*store.Duty{
		{DutyDate: d.now.AddDate(0, 0, 1), User: &store.User{FirstName: "Maria"}, AssignmentType: store.AssignmentTypeAdmin},
		{DutyDate: d.now.AddDate(0, 0, 30), UserID: 9, AssignmentType: store.AssignmentTypeRoundRobin},
	}, nil)

	chores, err := readActiveChores(context.Background(), d, nil)
	require.NoError(t, err)
	assert.Equal(t, "chore_id=5 | Ivan | \"Trash\" | assigned 2026-10-18 18:00 | deadline 2026-10-19 18:00 (OVERDUE)\n", chores)

	duties, err := readUpcomingDuties(context.Background(), d, []byte(`{"days":7}`))
	require.NoError(t, err)
	assert.Equal(t, "2026-10-21 | Maria | admin\n", duties)

	duties, err = readUpcomingDuties(context.Background(), d, []byte(`{}`))
	require.NoError(t, err)
	assert.NotContains(t, duties, "user #9", "default window is 14 days")
}

func TestTools_NamesUniqueAndDescribed(t *testing.T) {
	for _, list := range [][]toolDef{adminTools, participantTools} {
		assertToolList(t, list)
	}
}

func assertToolList(t *testing.T, list []toolDef) {
	t.Helper()
	seen := map[string]bool{}
	for _, tool := range list {
		assert.False(t, seen[tool.name], "duplicate tool %s", tool.name)
		seen[tool.name] = true
		assert.NotEmpty(t, tool.description, tool.name)
		kinds := 0
		for _, set := range []bool{tool.read != nil, tool.prepare != nil, tool.apply != nil} {
			if set {
				kinds++
			}
		}
		assert.Equal(t, 1, kinds, "%s must be exactly one of read/prepare/apply", tool.name)
		for _, r := range tool.required {
			assert.Contains(t, tool.properties, r, "%s requires undeclared %s", tool.name, r)
		}
	}
}

func TestParticipantTools(t *testing.T) {
	d, st, ex := toolDeps(t)
	d.userID = 555
	st.On("GetActiveChores", mock.Anything).Return([]*store.Chore{
		{ID: 1, Description: "Feed cat", ReminderID: "r1", User: &store.User{ID: 1, TelegramUserID: 555, FirstName: "Kid"}},
		{ID: 2, Description: "Dishes", ReminderID: "r2", User: &store.User{ID: 2, TelegramUserID: 777, FirstName: "Ivan"}},
	}, nil)
	find := func(name string) toolDef {
		tool, ok := findTool(participantTools, name)
		require.True(t, ok, name)
		return tool
	}

	mine, err := readMyChores(context.Background(), d, nil)
	require.NoError(t, err)
	assert.Contains(t, mine, `chore_id=1 | "Feed cat"`)
	assert.NotContains(t, mine, "Dishes")

	takeable, err := readTakeableChores(context.Background(), d, nil)
	require.NoError(t, err)
	assert.Contains(t, takeable, `chore_id=2 | assigned to Ivan | "Dishes"`)
	assert.NotContains(t, takeable, "Feed cat")

	run := func(name, input string) *preparedAction {
		action, err := find(name).prepare(context.Background(), d, []byte(input))
		require.NoError(t, err, name)
		if action.NoOp == "" {
			_, err = action.Execute(context.Background())
			require.NoError(t, err)
		}
		return action
	}
	run("complete_my_chore", `{"chore_id":1}`)
	run("take_chore", `{"chore_id":2}`)
	assert.Equal(t, "Chore #1 is already yours.", run("take_chore", `{"chore_id":1}`).NoOp)
	assert.Equal(t, []string{"chore_done:r1", "vc_confirm:2"}, ex.callbacks)

	run("volunteer_for_duty", `{"days":3}`)
	spend := run("spend_sviniya", `{"description":"  skip   dishes "}`)
	assert.Contains(t, spend.Summary, "<code>/spend skip dishes</code>")
	assert.Equal(t, []string{"/volunteer 3", "/spend skip dishes"}, ex.commands)

	_, err = find("complete_my_chore").prepare(context.Background(), d, []byte(`{"chore_id":2}`))
	assert.ErrorContains(t, err, "not yours")
	_, err = find("volunteer_for_duty").prepare(context.Background(), d, []byte(`{"days":9}`))
	assert.ErrorContains(t, err, "between 1 and 7")
	_, err = find("spend_sviniya").prepare(context.Background(), d, []byte(`{"description":""}`))
	assert.ErrorContains(t, err, "description is required")
}

func TestParticipantTools_NoAdminCapabilities(t *testing.T) {
	for _, tool := range participantTools {
		_, isAdminTool := findTool(adminTools, tool.name)
		if isAdminTool {
			// Shared read tools are fine; shared data-changing tools are not.
			assert.False(t, tool.mutating() || tool.apply != nil, "%s is an admin action exposed to participants", tool.name)
		}
	}
}
