package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/store/mocks"
	"github.com/stretchr/testify/assert"
)

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func TestTranslateIfNonLatin(t *testing.T) {
	ctx := context.Background()

	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "Wash the dishes"}}},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mockHandler)
	defer server.Close()

	llmClient := llm.NewClient("test-key", server.URL, 10, "", nil)
	mockStore := new(mocks.MockStore)

	h := NewWithAdminID(mockStore, nil, 0, 123, llmClient)

	tests := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name:        "Pure English",
			description: "Wash the dishes",
			expected:    "Wash the dishes", // Should not trigger translation
		},
		{
			name:        "Pure Russian",
			description: "Помыть посуду",
			expected:    "Wash the dishes", // Should trigger translation
		},
		{
			name:        "Mixed English and Russian",
			description: "Wash the посуду",
			expected:    "Wash the dishes", // Should trigger translation
		},
		{
			name:        "English with Emojis",
			description: "Wash the dishes 🍽️",
			expected:    "Wash the dishes 🍽️", // Should not trigger translation
		},
		{
			name:        "Russian with Emojis",
			description: "Помыть посуду 🍽️",
			expected:    "Wash the dishes", // Should trigger translation, mock returns "Wash the dishes"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.translateIfNonLatin(ctx, tt.description)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslateIfNonLatin_NoClient(t *testing.T) {
	ctx := context.Background()
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	result := h.translateIfNonLatin(ctx, "Помыть посуду")
	assert.Equal(t, "Помыть посуду", result)
}

func TestTranslateIfNonLatin_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	// Mock server that returns an error
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer errorServer.Close()

	llmClient := llm.NewClient("test-key", errorServer.URL, 10, "", nil)
	mockStore := new(mocks.MockStore)

	h := NewWithAdminID(mockStore, nil, 0, 123, llmClient)

	// Should fallback to original text when API fails
	result := h.translateIfNonLatin(ctx, "Помыть посуду")
	assert.Equal(t, "Помыть посуду", result)
}

func TestTranslateIfNonLatin_TimeoutHandling(t *testing.T) {
	ctx := context.Background()

	timeoutServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer timeoutServer.Close()

	llmClient := llm.NewClient("test-key", timeoutServer.URL, 1, "", nil)
	mockStore := new(mocks.MockStore)

	h := NewWithAdminID(mockStore, nil, 0, 123, llmClient)

	// Should fallback to original text on timeout
	result := h.translateIfNonLatin(ctx, "Помыть посуду")
	assert.Equal(t, "Помыть посуду", result)
}
