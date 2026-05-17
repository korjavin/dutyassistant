package telegram

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func assertCommandList(t *testing.T, label string, cmds []tgbotapi.BotCommand) {
	t.Helper()
	if len(cmds) == 0 {
		t.Fatalf("%s: expected non-empty command list", label)
	}
	seen := make(map[string]bool, len(cmds))
	for i, c := range cmds {
		if c.Command == "" {
			t.Errorf("%s[%d]: empty Command", label, i)
		}
		if len(c.Command) > 32 {
			t.Errorf("%s[%d]: Command %q exceeds 32-char Telegram limit", label, i, c.Command)
		}
		if c.Description == "" {
			t.Errorf("%s[%d] (%s): empty Description", label, i, c.Command)
		}
		if len(c.Description) > 256 {
			t.Errorf("%s[%d] (%s): description exceeds 256-char Telegram limit", label, i, c.Command)
		}
		if seen[c.Command] {
			t.Errorf("%s: duplicate command %q", label, c.Command)
		}
		seen[c.Command] = true
	}
}

func TestUserCommandsCatalog(t *testing.T) {
	assertCommandList(t, "userCommands", userCommands)
}

func TestAdminCommandsCatalog(t *testing.T) {
	assertCommandList(t, "adminCommands", adminCommands)

	// Sanity: adminCommands must be a strict superset of userCommands.
	adminSet := make(map[string]bool, len(adminCommands))
	for _, c := range adminCommands {
		adminSet[c.Command] = true
	}
	for _, u := range userCommands {
		if !adminSet[u.Command] {
			t.Errorf("adminCommands missing user command %q (Telegram scope precedence replaces, does not merge)", u.Command)
		}
	}
}

func TestGroupCommandsCatalog(t *testing.T) {
	assertCommandList(t, "groupCommands", groupCommands)
}

// commandsRoundTripFunc adapts a plain func into an http.RoundTripper so
// tests can hand-craft Telegram API responses per request without pulling
// in the handler test package's helpers.
type commandsRoundTripFunc func(req *http.Request) *http.Response

func (f commandsRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// capturedRequest is one decoded setMyCommands call.
type capturedRequest struct {
	scope    tgbotapi.BotCommandScope
	commands []tgbotapi.BotCommand
}

// captureSetMyCommands builds a *tgbotapi.BotAPI whose HTTP client records
// every setMyCommands POST into the returned slice. The forceStatus callback
// can be used to override the HTTP status code per request (return 0 to keep
// the default 200 OK response).
func captureSetMyCommands(t *testing.T, forceStatus func(scopeType string, chatID int64) int) (*tgbotapi.BotAPI, *[]capturedRequest) {
	t.Helper()

	var captured []capturedRequest

	rt := commandsRoundTripFunc(func(req *http.Request) *http.Response {
		reqURL := req.URL.String()
		if strings.Contains(reqURL, "/getMe") {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Bot","username":"bot"}}`)),
				Header:     make(http.Header),
			}
		}
		if strings.Contains(reqURL, "/setMyCommands") {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read setMyCommands body: %v", err)
			}
			form, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("parse setMyCommands form: %v", err)
			}

			var scope tgbotapi.BotCommandScope
			if s := form.Get("scope"); s != "" {
				if err := json.Unmarshal([]byte(s), &scope); err != nil {
					t.Fatalf("unmarshal scope %q: %v", s, err)
				}
			}
			var cmds []tgbotapi.BotCommand
			if c := form.Get("commands"); c != "" {
				if err := json.Unmarshal([]byte(c), &cmds); err != nil {
					t.Fatalf("unmarshal commands %q: %v", c, err)
				}
			}

			captured = append(captured, capturedRequest{scope: scope, commands: cmds})

			status := 200
			if forceStatus != nil {
				if s := forceStatus(scope.Type, scope.ChatID); s != 0 {
					status = s
				}
			}
			respBody := `{"ok":true,"result":true}`
			if status >= 400 {
				respBody = `{"ok":false,"error_code":` + strconv.Itoa(status) + `,"description":"forced failure"}`
			}
			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(bytes.NewBufferString(respBody)),
				Header:     make(http.Header),
			}
		}
		return &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			Header:     make(http.Header),
		}
	})

	client := &http.Client{Transport: rt}
	api, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, client)
	if err != nil {
		t.Fatalf("create bot api: %v", err)
	}
	return api, &captured
}

func TestRegisterCommands_AllScopes(t *testing.T) {
	api, captured := captureSetMyCommands(t, nil)

	registerCommands(api, 555, -100123)

	if len(*captured) != 3 {
		t.Fatalf("expected 3 setMyCommands requests, got %d", len(*captured))
	}

	// Request 0: all_private_chats with userCommands.
	if got := (*captured)[0].scope.Type; got != "all_private_chats" {
		t.Errorf("captured[0] scope type = %q, want all_private_chats", got)
	}
	assertCommandsEqual(t, "all_private_chats", (*captured)[0].commands, userCommands)

	// Request 1: admin chat with adminCommands.
	if got := (*captured)[1].scope.Type; got != "chat" {
		t.Errorf("captured[1] scope type = %q, want chat", got)
	}
	if got := (*captured)[1].scope.ChatID; got != 555 {
		t.Errorf("captured[1] scope chat_id = %d, want 555", got)
	}
	assertCommandsEqual(t, "admin_chat", (*captured)[1].commands, adminCommands)

	// Request 2: group chat with groupCommands.
	if got := (*captured)[2].scope.Type; got != "chat" {
		t.Errorf("captured[2] scope type = %q, want chat", got)
	}
	if got := (*captured)[2].scope.ChatID; got != -100123 {
		t.Errorf("captured[2] scope chat_id = %d, want -100123", got)
	}
	assertCommandsEqual(t, "group_chat", (*captured)[2].commands, groupCommands)
}

func TestRegisterCommands_NoAdminOrGroup(t *testing.T) {
	api, captured := captureSetMyCommands(t, nil)

	registerCommands(api, 0, 0)

	if len(*captured) != 1 {
		t.Fatalf("expected 1 setMyCommands request, got %d", len(*captured))
	}
	if got := (*captured)[0].scope.Type; got != "all_private_chats" {
		t.Errorf("scope type = %q, want all_private_chats", got)
	}
	assertCommandsEqual(t, "all_private_chats", (*captured)[0].commands, userCommands)
}

func TestRegisterCommands_AdminOnly(t *testing.T) {
	api, captured := captureSetMyCommands(t, nil)

	registerCommands(api, 555, 0)

	if len(*captured) != 2 {
		t.Fatalf("expected 2 setMyCommands requests, got %d", len(*captured))
	}
	if got := (*captured)[0].scope.Type; got != "all_private_chats" {
		t.Errorf("captured[0] scope type = %q, want all_private_chats", got)
	}
	if got := (*captured)[1].scope.Type; got != "chat" || (*captured)[1].scope.ChatID != 555 {
		t.Errorf("captured[1] scope = %+v, want chat/555", (*captured)[1].scope)
	}
	for _, c := range *captured {
		if c.scope.Type == "chat" && c.scope.ChatID != 555 {
			t.Errorf("unexpected group-scope request: %+v", c.scope)
		}
	}
}

func TestRegisterCommands_AdminScopeFails(t *testing.T) {
	api, captured := captureSetMyCommands(t, func(scopeType string, chatID int64) int {
		if scopeType == "chat" && chatID == 555 {
			return 500
		}
		return 0
	})

	registerCommands(api, 555, -100123)

	// All three calls must have still been attempted even though the
	// middle one failed — the helper logs and continues.
	if len(*captured) != 3 {
		t.Fatalf("expected 3 setMyCommands attempts, got %d", len(*captured))
	}
	if (*captured)[0].scope.Type != "all_private_chats" {
		t.Errorf("captured[0] scope type = %q, want all_private_chats", (*captured)[0].scope.Type)
	}
	if (*captured)[1].scope.Type != "chat" || (*captured)[1].scope.ChatID != 555 {
		t.Errorf("captured[1] scope = %+v, want chat/555 (failing admin call still attempted)", (*captured)[1].scope)
	}
	if (*captured)[2].scope.Type != "chat" || (*captured)[2].scope.ChatID != -100123 {
		t.Errorf("captured[2] scope = %+v, want chat/-100123 (group call after admin failure)", (*captured)[2].scope)
	}
}

func assertCommandsEqual(t *testing.T, label string, got, want []tgbotapi.BotCommand) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: command count = %d, want %d", label, len(got), len(want))
		return
	}
	for i := range want {
		if got[i].Command != want[i].Command {
			t.Errorf("%s[%d]: Command = %q, want %q", label, i, got[i].Command, want[i].Command)
		}
		if got[i].Description != want[i].Description {
			t.Errorf("%s[%d] (%s): Description = %q, want %q", label, i, want[i].Command, got[i].Description, want[i].Description)
		}
	}
}
