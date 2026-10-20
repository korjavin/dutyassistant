package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anthropicServer returns a fake Messages API that records the last request
// body and replies with the given JSON.
func anthropicServer(t *testing.T, status int, reply string, gotBody *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Expected /v1/messages, got %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Api-Key"); got != "test-key" {
			t.Errorf("Expected x-api-key test-key, got %q", got)
		}
		if gotBody != nil {
			if err := json.NewDecoder(r.Body).Decode(gotBody); err != nil {
				t.Errorf("Failed to decode request body: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
}

func messageReply(stopReason string, blocks string) string {
	return `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5",` +
		`"content":[` + blocks + `],"stop_reason":"` + stopReason + `",` +
		`"usage":{"input_tokens":10,"output_tokens":5}}`
}

func TestNewAnthropicClient(t *testing.T) {
	if c := NewAnthropicClient("", "", 10, "", ""); c != nil {
		t.Error("Expected nil client when API key is empty")
	}

	c := NewAnthropicClient("key", "", 10, "", "")
	if c.Provider() != ProviderAnthropic {
		t.Errorf("Expected anthropic provider, got %s", c.Provider())
	}
	if c.model != "claude-opus-5-5" {
		t.Errorf("Expected default model, got %s", c.model)
	}
	if c.Effort() != "low" {
		t.Errorf("Expected default effort low, got %s", c.Effort())
	}

	c = NewAnthropicClient("key", "", 10, "claude-sonnet-5-5", "HIGH")
	if c.model != "claude-sonnet-5-5" || c.Effort() != "high" {
		t.Errorf("Expected custom model/effort, got %s/%s", c.model, c.Effort())
	}

	if c := NewAnthropicClient("key", "", 10, "", "bogus"); c.Effort() != "low" {
		t.Errorf("Expected invalid effort to fall back to low, got %s", c.Effort())
	}

	if c := NewClient("key", "", 10, "", nil); c.Provider() != ProviderOpenAI {
		t.Errorf("Expected openai provider, got %s", c.Provider())
	}
}

func TestAnthropic_AllMethods(t *testing.T) {
	ctx := context.Background()
	var body map[string]any
	server := anthropicServer(t, http.StatusOK,
		messageReply("end_turn", `{"type":"thinking","thinking":"","signature":"sig"},{"type":"text","text":"Hello <b>Anna</b> & co"}`),
		&body)
	defer server.Close()

	c := NewAnthropicClient("test-key", server.URL, 10, "", "medium")

	nudge, err := c.ChoreNudge(ctx, "Anna", "1. \"Trash\" - OVERDUE by 2 days\n")
	if err != nil {
		t.Fatalf("ChoreNudge error: %v", err)
	}
	if nudge != "Hello <b>Anna</b> &amp; co" {
		t.Errorf("Unexpected nudge: %q", nudge)
	}

	if body["model"] != "claude-opus-5-5" {
		t.Errorf("Expected model claude-opus-5-5, got %v", body["model"])
	}
	if _, ok := body["temperature"]; ok {
		t.Error("Temperature must not be sent to Anthropic")
	}
	if oc, _ := body["output_config"].(map[string]any); oc["effort"] != "medium" {
		t.Errorf("Expected effort medium, got %v", body["output_config"])
	}
	system, _ := json.Marshal(body["system"])
	if !strings.Contains(string(system), "carrot") {
		t.Errorf("Expected chore nudge system prompt, got %s", system)
	}
	msgs, _ := json.Marshal(body["messages"])
	if !strings.Contains(string(msgs), "OVERDUE by 2 days") {
		t.Errorf("Expected chore context in user message, got %s", msgs)
	}

	if got := c.RefineMessage(ctx, "be funny", "vanilla"); got != "Hello <b>Anna</b> &amp; co" {
		t.Errorf("Unexpected RefineMessage result: %q", got)
	}

	translated, err := c.TranslateToEnglish(ctx, "Вынести мусор")
	if err != nil || translated != "Hello <b>Anna</b> & co" {
		t.Errorf("Unexpected TranslateToEnglish result: %q, %v", translated, err)
	}
}

func TestAnthropic_ErrorCases(t *testing.T) {
	ctx := context.Background()

	refusal := anthropicServer(t, http.StatusOK,
		messageReply("refusal", `{"type":"text","text":"partial"}`), nil)
	defer refusal.Close()
	c := NewAnthropicClient("test-key", refusal.URL, 10, "", "")
	if _, err := c.ChoreNudge(ctx, "Anna", "chores"); err == nil {
		t.Error("Expected error on refusal")
	}
	if got := c.RefineMessage(ctx, "be funny", "vanilla"); got != "vanilla" {
		t.Errorf("Expected vanilla on refusal, got %q", got)
	}

	noText := anthropicServer(t, http.StatusOK,
		messageReply("max_tokens", `{"type":"thinking","thinking":"","signature":"sig"}`), nil)
	defer noText.Close()
	c = NewAnthropicClient("test-key", noText.URL, 10, "", "")
	if got, err := c.TranslateToEnglish(ctx, "текст"); err == nil || got != "текст" {
		t.Errorf("Expected original text and error when no text returned, got %q, %v", got, err)
	}

	badRequest := anthropicServer(t, http.StatusBadRequest,
		`{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}`, nil)
	defer badRequest.Close()
	c = NewAnthropicClient("test-key", badRequest.URL, 10, "", "")
	if _, err := c.ChoreNudge(ctx, "Anna", "chores"); err == nil {
		t.Error("Expected error on 400 response")
	}
}

// recordingTransport captures request URLs instead of hitting the network.
type recordingTransport struct{ urls []string }

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.urls = append(r.urls, req.URL.String())
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(messageReply("end_turn", `{"type":"text","text":"ok"}`))),
		Request:    req,
	}, nil
}

// Regression: docker-compose sets ANTHROPIC_BASE_URL to an empty string when it
// is not configured, and the SDK used it verbatim ("unsupported protocol scheme").
func TestNewAnthropicClient_EmptyBaseURLEnvUsesProduction(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "")
	rec := &recordingTransport{}
	orig := http.DefaultTransport
	http.DefaultTransport = rec
	t.Cleanup(func() { http.DefaultTransport = orig })

	c := NewAnthropicClient("test-key", "", 10, "", "")
	_, err := c.ChoreNudge(context.Background(), "Anna", "chores")

	require.NoError(t, err)
	require.Len(t, rec.urls, 1)
	assert.Equal(t, "https://api.anthropic.com/v1/messages", rec.urls[0])
}

func TestNewAnthropicClient_ExplicitBaseURLWinsOverEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "https://env.example.com")
	rec := &recordingTransport{}
	orig := http.DefaultTransport
	http.DefaultTransport = rec
	t.Cleanup(func() { http.DefaultTransport = orig })

	c := NewAnthropicClient("test-key", "https://proxy.example.com", 10, "", "")
	_, err := c.ChoreNudge(context.Background(), "Anna", "chores")

	require.NoError(t, err)
	require.Len(t, rec.urls, 1)
	assert.Equal(t, "https://proxy.example.com/v1/messages", rec.urls[0])
}
