package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	defaultAnthropicModel  = "claude-opus-5-5"
	defaultAnthropicEffort = anthropic.OutputConfigEffortLow
	// Generous output ceiling: thinking tokens count against it, while the
	// visible messages we ask for are only a few sentences.
	anthropicMaxTokens = 16000
)

// NewAnthropicClient creates an LLM client backed by the Anthropic Messages API.
// If apiKey is empty, it returns nil, effectively disabling the LLM features.
// effort is one of low|medium|high|xhigh|max (default low: the bot only needs
// short, playful texts, so low effort keeps replies fast and cheap).
func NewAnthropicClient(apiKey, baseURL string, timeoutSeconds int, model, effort string) *Client {
	if apiKey == "" {
		return nil
	}
	if model == "" {
		model = defaultAnthropicModel
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithRequestTimeout(time.Duration(timeoutSeconds) * time.Second),
		// Timeouts are retried; keep the worst case bounded for chat handlers.
		option.WithMaxRetries(1),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(opts...)

	return &Client{
		baseURL:   baseURL,
		model:     model,
		anthropic: &client,
		effort:    parseEffort(effort),
	}
}

func parseEffort(effort string) anthropic.OutputConfigEffort {
	switch e := anthropic.OutputConfigEffort(strings.ToLower(strings.TrimSpace(effort))); e {
	case anthropic.OutputConfigEffortLow, anthropic.OutputConfigEffortMedium, anthropic.OutputConfigEffortHigh,
		anthropic.OutputConfigEffortXhigh, anthropic.OutputConfigEffortMax:
		return e
	default:
		return defaultAnthropicEffort
	}
}

// Effort returns the configured Anthropic effort level ("" for OpenAI clients).
func (c *Client) Effort() string {
	if c == nil {
		return ""
	}
	return string(c.effort)
}

// anthropicCompletion performs a single Messages API request and returns the
// concatenated text blocks of the response.
func (c *Client) anthropicCompletion(ctx context.Context, op, systemPrompt, userPrompt string) (string, error) {
	resp, err := c.anthropic.Messages.New(ctx, anthropic.MessageNewParams{
		Model:        anthropic.Model(c.model),
		MaxTokens:    anthropicMaxTokens,
		System:       []anthropic.TextBlockParam{{Text: systemPrompt}},
		OutputConfig: anthropic.OutputConfigParam{Effort: c.effort},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("LLM %s Anthropic request error: %w", op, err)
	}

	if resp.StopReason == anthropic.StopReasonRefusal {
		return "", fmt.Errorf("LLM %s Anthropic refused (category: %s)", op, resp.StopDetails.Category)
	}

	var sb strings.Builder
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			sb.WriteString(text.Text)
		}
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("LLM %s Anthropic returned no text (stop reason: %s)", op, resp.StopReason)
	}

	return sb.String(), nil
}
