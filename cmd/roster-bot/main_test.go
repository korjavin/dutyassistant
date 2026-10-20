package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateHTTPServer(t *testing.T) {
	handler := http.NewServeMux()
	srv := createHTTPServer(handler)

	assert.NotNil(t, srv)
	assert.Equal(t, ":8080", srv.Addr)
	assert.Equal(t, handler, srv.Handler)
	assert.Equal(t, 10*time.Second, srv.ReadHeaderTimeout)
	assert.Equal(t, 30*time.Second, srv.ReadTimeout)
	assert.Equal(t, 30*time.Second, srv.WriteTimeout)
	assert.Equal(t, 60*time.Second, srv.IdleTimeout)
	assert.Equal(t, 1<<20, srv.MaxHeaderBytes)
}

func TestResolveLLMProvider(t *testing.T) {
	tests := []struct {
		name         string
		explicit     string
		anthropicKey string
		want         string
	}{
		{"default is openai", "", "", "openai"},
		{"anthropic key auto-selects anthropic", "", "sk-ant", "anthropic"},
		{"explicit openai wins over anthropic key", "openai", "sk-ant", "openai"},
		{"explicit anthropic", " Anthropic ", "", "anthropic"},
		{"unknown value falls back to auto", "gemini", "sk-ant", "anthropic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveLLMProvider(tt.explicit, tt.anthropicKey); got != tt.want {
				t.Errorf("resolveLLMProvider(%q, %q) = %q, want %q", tt.explicit, tt.anthropicKey, got, tt.want)
			}
		})
	}
}
