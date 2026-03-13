package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("", "", 10)
	if client != nil {
		t.Errorf("Expected nil client when API key is empty")
	}

	client = NewClient("some-key", "", 10)
	if client == nil {
		t.Errorf("Expected non-nil client with valid key")
	}
	if client.baseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected default baseURL, got: %s", client.baseURL)
	}

	client = NewClient("some-key", "http://localhost:1234", 10)
	if client.baseURL != "http://localhost:1234" {
		t.Errorf("Expected custom baseURL, got: %s", client.baseURL)
	}
}

func TestSanitizeTelegramHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Clean HTML", "<b>Hello</b> <i>World</i>", "<b>Hello</b> <i>World</i>"},
		{"Unescaped ampersand", "A & B", "A &amp; B"},
		{"Disallowed tag", "<b>Hello</b> <script>alert(1)</script>", "<b>Hello</b> &lt;script&gt;alert(1)&lt;/script&gt;"},
		{"Valid links", `<a href="http://example.com">Link</a>`, `<a href="http://example.com">Link</a>`},
		{"Multiple tags", "<s><b>test</b></s> & <pre>code</pre>", "<s><b>test</b></s> &amp; <pre>code</pre>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := SanitizeTelegramHTML(tt.input)
			if res != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, res)
			}
		})
	}
}

func TestRefineMessage(t *testing.T) {
	ctx := context.Background()
	intent := "be funny"
	vanilla := "You have to do the dishes"

	// Test 1: Nil client returns vanilla
	var client *Client
	if res := client.RefineMessage(ctx, intent, vanilla); res != vanilla {
		t.Errorf("Expected vanilla message from nil client, got: %s", res)
	}

	// Mock server
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Expected /chat/completions, got %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("Expected Bearer test-key, got %s", auth)
		}

		w.WriteHeader(http.StatusOK)
		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "🍽️ Get ready for splash mountain! You're on dish duty <script>bad</script> & it's fun!"}}},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mockHandler)
	defer server.Close()

	// Test 2: Successful response + Sanitization
	c := NewClient("test-key", server.URL, 10)
	res := c.RefineMessage(ctx, intent, vanilla)
	expected := "🍽️ Get ready for splash mountain! You're on dish duty &lt;script&gt;bad&lt;/script&gt; &amp; it's fun!"
	if res != expected {
		t.Errorf("Expected refined message %q, got: %q", expected, res)
	}
}

func TestRefineMessage_ErrorCases(t *testing.T) {
	ctx := context.Background()
	intent := "be funny"
	vanilla := "You have to do the dishes"

	// Mock server that returns a 500
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer errorServer.Close()

	c := NewClient("test-key", errorServer.URL, 10)
	if res := c.RefineMessage(ctx, intent, vanilla); res != vanilla {
		t.Errorf("Expected vanilla message on 500 error, got: %s", res)
	}

	// Mock server that hangs (timeout test)
	timeoutServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer timeoutServer.Close()

	cTimeout := NewClient("test-key", timeoutServer.URL, 1)
	if res := cTimeout.RefineMessage(ctx, intent, vanilla); res != vanilla {
		t.Errorf("Expected vanilla message on timeout, got: %s", res)
	}
}
