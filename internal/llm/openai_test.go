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
	client := NewClient("", "", 10, "", nil)
	if client != nil {
		t.Errorf("Expected nil client when API key is empty")
	}

	client = NewClient("some-key", "", 10, "", nil)
	if client == nil {
		t.Errorf("Expected non-nil client with valid key")
	}
	if client.baseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected default baseURL, got: %s", client.baseURL)
	}
	if client.model != "gpt-4o-mini" {
		t.Errorf("Expected default model, got: %s", client.model)
	}
	if client.temperature != 0.7 {
		t.Errorf("Expected default temperature, got: %f", client.temperature)
	}

	customTemp := 0.5
	client = NewClient("some-key", "http://localhost:1234", 10, "gpt-4", &customTemp)
	if client.baseURL != "http://localhost:1234" {
		t.Errorf("Expected custom baseURL, got: %s", client.baseURL)
	}
	if client.model != "gpt-4" {
		t.Errorf("Expected custom model, got: %s", client.model)
	}
	if client.temperature != 0.5 {
		t.Errorf("Expected custom temperature, got: %f", client.temperature)
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
	c := NewClient("test-key", server.URL, 10, "", nil)
	res := c.RefineMessage(ctx, intent, vanilla)
	expected := "🍽️ Get ready for splash mountain! You're on dish duty &lt;script&gt;bad&lt;/script&gt; &amp; it's fun!"
	if res != expected {
		t.Errorf("Expected refined message %q, got: %q", expected, res)
	}
}

func TestRefineMessage_CustomConfig(t *testing.T) {
	ctx := context.Background()
	intent := "be funny"
	vanilla := "You have to do the dishes"

	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if req.Model != "custom-model" {
			t.Errorf("Expected model custom-model, got: %s", req.Model)
		}
		if req.Temperature != 0.9 {
			t.Errorf("Expected temperature 0.9, got: %f", req.Temperature)
		}

		w.WriteHeader(http.StatusOK)
		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "Refined message"}}},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mockHandler)
	defer server.Close()

	customTemp := 0.9
	c := NewClient("test-key", server.URL, 10, "custom-model", &customTemp)
	res := c.RefineMessage(ctx, intent, vanilla)
	if res != "Refined message" {
		t.Errorf("Expected 'Refined message', got: %s", res)
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

	c := NewClient("test-key", errorServer.URL, 10, "", nil)
	if res := c.RefineMessage(ctx, intent, vanilla); res != vanilla {
		t.Errorf("Expected vanilla message on 500 error, got: %s", res)
	}

	// Mock server that hangs (timeout test)
	timeoutServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer timeoutServer.Close()

	cTimeout := NewClient("test-key", timeoutServer.URL, 1, "", nil)
	if res := cTimeout.RefineMessage(ctx, intent, vanilla); res != vanilla {
		t.Errorf("Expected vanilla message on timeout, got: %s", res)
	}
}

func TestNewClient_ZeroTemperature(t *testing.T) {
	zeroTemp := 0.0
	client := NewClient("some-key", "", 10, "", &zeroTemp)
	if client.temperature != 0.0 {
		t.Errorf("Expected custom temperature 0.0, got: %f", client.temperature)
	}
}