package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/notification"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type morningDutyNotification struct {
	recipient, status, detail string
}

type morningDutyTestStore struct {
	notification.MorningDutyReader
	readErr       error
	assigneeReads int
	logs          []morningDutyNotification
}

func (s *morningDutyTestStore) GetUserByID(ctx context.Context, id int64) (*store.User, error) {
	s.assigneeReads++
	if s.readErr != nil {
		return nil, s.readErr
	}
	return s.MorningDutyReader.GetUserByID(ctx, id)
}

func (s *morningDutyTestStore) LogNotification(_ context.Context, _ time.Time, _ int64, recipient, status, detail string) error {
	s.logs = append(s.logs, morningDutyNotification{recipient, status, detail})
	return nil
}

// newMorningDutyLLM mirrors the messages request and choices[].message.content
// response from the OpenAI Chat Completions API.
// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create
func newMorningDutyLLM(t *testing.T, status int, reply func(prompt string) string) (*llm.Client, <-chan string) {
	t.Helper()
	prompts := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		var req struct {
			Messages []struct{ Content string }
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) != 2 {
			t.Errorf("unexpected LLM request: %+v, %v", req, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		prompt := req.Messages[1].Content
		prompts <- prompt
		content := "<b>Go team!</b> <uhoh> & cheer!"
		if reply != nil {
			content = reply(prompt)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		response := map[string]any{"choices": []any{
			map[string]any{"message": map[string]string{"role": "assistant", "content": content}},
		}}
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	t.Cleanup(server.Close)
	return llm.NewClient("test-key", server.URL, 5, "test-model", nil), prompts
}

func TestRunMorningDutyPersonalizesBothRecipients(t *testing.T) {
	ctx := t.Context()
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	// The previous two calendar days span the switch to daylight saving time.
	today := time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name          string
		reason        store.AssignmentType
		preassigned   bool
		wantBehind    string
		wantAhead     string
		volunteerDays int
		adminDays     int
	}{
		{"new volunteer assignment", store.AssignmentTypeVoluntary, false, "2", "5", 3, 2},
		{"existing admin assignment", store.AssignmentTypeAdmin, true, "2", "3", 0, 2},
		{"existing round-robin assignment with zero counts", store.AssignmentTypeRoundRobin, true, "0", "0", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := sqlite.New(ctx, ":memory:")
			require.NoError(t, err)
			assignee := &store.User{TelegramUserID: 101, FirstName: "Ana & <b>*_O'Neil</b>", IsActive: true, VolunteerQueueDays: tc.volunteerDays, AdminQueueDays: tc.adminDays}
			require.NoError(t, db.CreateUser(ctx, assignee))
			duty := &store.Duty{UserID: assignee.ID, User: assignee, DutyDate: today, AssignmentType: tc.reason, CreatedAt: today}
			if tc.wantBehind != "0" {
				// The gap at -3 stops the streak. The future admin row is independent of the queue.
				for _, offset := range []int{-4, -2, -1, 2} {
					row := &store.Duty{UserID: assignee.ID, DutyDate: today.AddDate(0, 0, offset), AssignmentType: store.AssignmentTypeAdmin, CreatedAt: today}
					require.NoError(t, db.CreateDuty(ctx, row))
				}
			}
			if tc.preassigned {
				require.NoError(t, db.CreateDuty(ctx, duty))
			}
			assign := func(ctx context.Context) (*store.Duty, error, error) {
				if tc.preassigned {
					// GetDutyByDate does not populate queue counters.
					existing, err := db.GetDutyByDate(ctx, today)
					return existing, nil, err
				}
				if err := db.CreateDuty(ctx, duty); err != nil {
					return nil, nil, err
				}
				// The scheduler returns a user record read before the decrement.
				return duty, db.DecrementVolunteerQueue(ctx, assignee.ID), nil
			}
			st := &morningDutyTestStore{MorningDutyReader: db}
			client, prompts := newMorningDutyLLM(t, http.StatusOK, nil)
			var recipients []int64
			send := func(chatID int64, text string) error {
				recipients = append(recipients, chatID)
				assert.Equal(t, "<b>Go team!</b> &lt;uhoh&gt; &amp; cheer!", text)
				return nil
			}

			runMorningDuty(ctx, assign, st, client, send, -200, berlin)

			assert.Equal(t, []int64{101, -200}, recipients)
			assert.Equal(t, []morningDutyNotification{{"DM", "SUCCESS", ""}, {"GROUP", "SUCCESS", ""}}, st.logs)
			assert.Equal(t, 1, st.assigneeReads)
			require.Len(t, prompts, 2)
			for _, audience := range []string{"private Telegram DM", "Telegram group announcement"} {
				prompt := <-prompts
				assert.Contains(t, prompt, audience)
				assert.Contains(t, prompt, `Assignee name: "Ana & <b>*_O'Neil</b>"`)
				assert.Contains(t, prompt, "Assignment reason: "+string(tc.reason))
				assert.Contains(t, prompt, "Days behind: "+tc.wantBehind)
				assert.Contains(t, prompt, "Days ahead: "+tc.wantAhead)
				assert.Contains(t, prompt, "both counts into a friendly joke and encouragement")
			}
		})
	}
}

func TestRunMorningDutyFallback(t *testing.T) {
	ctx := t.Context()
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	db, err := sqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	assignee := &store.User{TelegramUserID: 101, FirstName: "Ana & <b>*_O'Neil</b>", IsActive: true}
	require.NoError(t, db.CreateUser(ctx, assignee))
	duty := &store.Duty{
		UserID: assignee.ID, User: assignee, AssignmentType: store.AssignmentTypeVoluntary,
		DutyDate: time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC),
	}
	for _, tc := range []struct {
		name        string
		llmStatus   int
		queueErr    error
		readErr     error
		dmErr       error
		wantReads   int
		wantPrompts int
	}{
		{name: "disabled LLM"},
		{name: "provider error", llmStatus: http.StatusServiceUnavailable, wantReads: 1, wantPrompts: 2},
		{name: "queue write error", llmStatus: http.StatusOK, queueErr: errors.New("queue unavailable")},
		{name: "context read error and failed DM", llmStatus: http.StatusOK, readErr: errors.New("read failed"), dmErr: errors.New("DM refused"), wantReads: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &morningDutyTestStore{MorningDutyReader: db, readErr: tc.readErr}
			var client *llm.Client
			var prompts <-chan string
			if tc.llmStatus != 0 {
				client, prompts = newMorningDutyLLM(t, tc.llmStatus, nil)
			}
			assign := func(context.Context) (*store.Duty, error, error) { return duty, tc.queueErr, nil }
			var recipients []int64
			send := func(chatID int64, text string) error {
				recipients = append(recipients, chatID)
				const escapedName = "<b>Ana &amp; &lt;b&gt;*_O&#39;Neil&lt;/b&gt;</b>"
				if chatID == 101 {
					assert.Equal(t, "Congratulations, "+escapedName+"! You're on duty today because of your volunteer request. Thanks for helping with chores!", text)
					return tc.dmErr
				}
				assert.Equal(t, "Today's duty goes to "+escapedName+" (volunteered). Let's cheer them on!", text)
				return nil
			}

			runMorningDuty(ctx, assign, st, client, send, -200, berlin)

			assert.Equal(t, []int64{101, -200}, recipients)
			assert.Equal(t, tc.wantReads, st.assigneeReads)
			assert.Len(t, prompts, tc.wantPrompts)
			dmLog := morningDutyNotification{"DM", "SUCCESS", ""}
			if tc.dmErr != nil {
				dmLog = morningDutyNotification{"DM", "FAILED", tc.dmErr.Error()}
			}
			assert.Equal(t, []morningDutyNotification{dmLog, {"GROUP", "SUCCESS", ""}}, st.logs)
		})
	}
}

func TestRunMorningDutyFallsBackOnMalformedHTML(t *testing.T) {
	ctx := t.Context()
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	db, err := sqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	assignee := &store.User{TelegramUserID: 101, FirstName: "Ana & Co", IsActive: true}
	require.NoError(t, db.CreateUser(ctx, assignee))
	duty := &store.Duty{
		UserID: assignee.ID, User: assignee, AssignmentType: store.AssignmentTypeVoluntary,
		DutyDate: time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC),
	}
	const (
		validDM       = "<i><strong>Your turn!</strong></i>"
		validGroup    = `<a href='https://example.com/?day=1&who=ana'><b>Go Ana!</b></a>`
		fallbackDM    = "Congratulations, <b>Ana &amp; Co</b>! You're on duty today because of your volunteer request. Thanks for helping with chores!"
		fallbackGroup = "Today's duty goes to <b>Ana &amp; Co</b> (volunteered). Let's cheer them on!"
	)
	for _, tc := range []struct {
		name              string
		dm, group         string
		wantDM, wantGroup string
	}{
		{"unclosed DM tag", "<b>Go Ana!", validGroup, fallbackDM, validGroup},
		{"misnested group tags", validDM, "<b><i>Go Ana!</b></i>", validDM, fallbackGroup},
		{"balanced nested DM links", `<a href="https://example.com"><a href="https://example.org">Go Ana!</a></a>`, validGroup, fallbackDM, validGroup},
		{"balanced style within group code", validDM, "<code><b>Go Ana!</b></code>", validDM, fallbackGroup},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &morningDutyTestStore{MorningDutyReader: db}
			client, prompts := newMorningDutyLLM(t, http.StatusOK, func(prompt string) string {
				if strings.Contains(prompt, "private Telegram DM") {
					return tc.dm
				}
				assert.Contains(t, prompt, "Telegram group announcement")
				return tc.group
			})
			assign := func(context.Context) (*store.Duty, error, error) { return duty, nil, nil }
			type sentMessage struct {
				chatID int64
				text   string
			}
			var sent []sentMessage
			send := func(chatID int64, text string) error {
				sent = append(sent, sentMessage{chatID, text})
				return nil
			}
			runMorningDuty(ctx, assign, st, client, send, -200, berlin)

			assert.Equal(t, []sentMessage{{101, tc.wantDM}, {-200, tc.wantGroup}}, sent)
			assert.Equal(t, []morningDutyNotification{{"DM", "SUCCESS", ""}, {"GROUP", "SUCCESS", ""}}, st.logs)
			assert.Len(t, prompts, 2)
		})
	}
}
