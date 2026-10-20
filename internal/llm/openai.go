package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// ClientInterface defines the interface for LLM operations.
type ClientInterface interface {
	TranslateToEnglish(ctx context.Context, text string) (string, error)
	RefineMessage(ctx context.Context, intent, vanilla string) string
	ChoreNudge(ctx context.Context, userName, choresContext string) (string, error)
}

// Provider names accepted by LLM_PROVIDER.
const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
)

// Client is an adapter for calling an LLM. It talks either to an
// OpenAI-compatible chat completion API or to the Anthropic Messages API,
// depending on how it was constructed (NewClient / NewAnthropicClient).
type Client struct {
	apiKey      string
	baseURL     string
	model       string
	temperature float64
	httpClient  *http.Client

	// Set only for the Anthropic provider.
	anthropic *anthropic.Client
	effort    anthropic.OutputConfigEffort
}

// NewClient creates a new LLM client. If apiKey is empty, it returns nil,
// effectively disabling the LLM features.
func NewClient(apiKey, baseURL string, timeoutSeconds int, model string, temperature *float64) *Client {
	if apiKey == "" {
		return nil
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	temp := 0.7
	if temperature != nil {
		temp = *temperature
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	return &Client{
		apiKey:      apiKey,
		baseURL:     baseURL,
		model:       model,
		temperature: temp,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Config returns the current configuration of the LLM client.
func (c *Client) Config() (string, float64, string) {
	if c == nil {
		return "", 0, ""
	}
	return c.model, c.temperature, c.baseURL
}

// Provider returns which LLM API the client talks to.
func (c *Client) Provider() string {
	if c != nil && c.anthropic != nil {
		return ProviderAnthropic
	}
	return ProviderOpenAI
}

// chatMessage represents a single message in the chat API.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the JSON body for the chat completion request.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

// chatResponse represents the JSON response from the chat completion API.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

var allowedTelegramTagsRegex *regexp.Regexp

func init() {
	// Telegram supported tags list
	allowedTags := []string{
		"b", "strong", "i", "em", "u", "ins", "s", "strike", "del", "code", "pre",
	}

	var allowRegexParts []string
	for _, tag := range allowedTags {
		allowRegexParts = append(allowRegexParts, fmt.Sprintf("</?%s>", tag))
	}
	// Special handling for a href - only allow http/https URLs to prevent XSS
	allowRegexParts = append(allowRegexParts, `<a href="(https?:\/\/[^"]*)">`)
	allowRegexParts = append(allowRegexParts, `<a href='(https?:\/\/[^']*)'>`)
	allowRegexParts = append(allowRegexParts, `</a>`)
	// Special handling for tg-spoiler
	allowRegexParts = append(allowRegexParts, `<tg-spoiler>`)
	allowRegexParts = append(allowRegexParts, `</tg-spoiler>`)

	allowPattern := strings.Join(allowRegexParts, "|")
	allowedTelegramTagsRegex = regexp.MustCompile("(?i)(" + allowPattern + ")")
}

// SanitizeTelegramHTML ensures that only permitted Telegram HTML tags remain
// and that any raw `<` or `&` inside text nodes are properly escaped.
func SanitizeTelegramHTML(input string) string {
	// Split input by allowed tags
	parts := allowedTelegramTagsRegex.Split(input, -1)
	tags := allowedTelegramTagsRegex.FindAllString(input, -1)

	var result strings.Builder
	for i, part := range parts {
		// Escape &, <, > in text parts
		escapedPart := strings.ReplaceAll(part, "&", "&amp;")
		escapedPart = strings.ReplaceAll(escapedPart, "<", "&lt;")
		escapedPart = strings.ReplaceAll(escapedPart, ">", "&gt;")
		result.WriteString(escapedPart)

		// Re-append the allowed tag
		if i < len(tags) {
			result.WriteString(tags[i])
		}
	}

	return result.String()
}

// TranslateToEnglish translates the given text to English if it's not already.
// It returns the original text if the translation fails or if the client is nil.
func (c *Client) TranslateToEnglish(ctx context.Context, text string) (string, error) {
	if c == nil {
		return text, nil
	}

	systemPrompt := "You are a translator. Translate the given chore description to English. Be concise, do not be verbose. Return ONLY the translated text. Preserve emojis. If translation is not possible or text is already English, return original."

	// Lower temperature for translation
	translated, err := c.complete(ctx, "TranslateToEnglish", systemPrompt, text, 0.3)
	if err != nil {
		slog.Error(err.Error())
		return text, err
	}

	if translated == "" {
		return text, nil
	}

	return translated, nil
}

// RefineMessage sends the vanilla message and intent to the LLM to get a more
// fun/humorous formatted message. It returns the vanilla message if there's any error
// or if the client is nil.
func (c *Client) RefineMessage(ctx context.Context, intent, vanilla string) string {
	if c == nil {
		return vanilla
	}

	systemPrompt := `You are a fun, witty, and enthusiastic assistant managing household chores.
Your task is to reformat the given message based on the specified intent while keeping the core information intact.
Return ONLY the formatted text. Use emojis appropriately. Do NOT wrap the output in quotes or markdown code blocks.
If formatting text, ONLY use Telegram-supported HTML tags (<b>, <i>, <a>, <code>, <pre>). Do NOT use any unsupported HTML tags.`

	userPrompt := fmt.Sprintf("Intent: %s\nOriginal Message: %s", intent, vanilla)

	refined, err := c.complete(ctx, "RefineMessage", systemPrompt, userPrompt, c.temperature)
	if err != nil {
		slog.Error(err.Error())
		return vanilla
	}

	if refined == "" {
		return vanilla
	}

	return SanitizeTelegramHTML(refined)
}

// choreNudgeSystemPrompt instructs the LLM to write a carrot-and-stick reminder
// for someone who is putting off their chores.
const choreNudgeSystemPrompt = `You are a witty, warm, slightly dramatic household chore coach sending a private Telegram message to a family member who is putting off their chores.
You will receive the person's name and a list of their pending chores, including how long ago each was assigned and whether it is overdue.

Write a short (2-5 sentences), funny, motivating message urging them to finish the chores. Use a carrot-and-stick approach:
- Carrot: playfully hint at a small reward for finishing soon (a little treat, a surprise, bragging rights, eternal glory). Keep promises small and vague - never promise money or anything specific and expensive.
- Stick: jokingly threaten mild consequences for further delay, such as "reporting" them to the admin or getting extra duties assigned.
Scale the tone with the delay: gentle and cheerful when nothing is overdue, more theatrical and insistent the longer chores are overdue. Mention the chores and delays specifically.
Keep it kind and family-friendly - teasing, never insulting, scary or guilt-tripping. The recipient may be a child.

Return ONLY the message text. Use emojis. Do NOT list the chores as a numbered list (the list is appended separately). Do NOT wrap the output in quotes or markdown code blocks.
If formatting text, ONLY use Telegram-supported HTML tags (<b>, <i>, <u>, <s>). Do NOT use Markdown.`

// ChoreNudge asks the LLM for a funny carrot-and-stick motivation message
// for a person who is delaying their chores. choresContext is a plain-text
// description of the pending chores and their delays. The result is sanitized
// Telegram HTML. An error is returned if the client is nil or the call fails,
// so callers can fall back to a static message.
func (c *Client) ChoreNudge(ctx context.Context, userName, choresContext string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("LLM client not configured")
	}

	userPrompt := fmt.Sprintf("Name: %s\n\nPending chores:\n%s", userName, choresContext)

	content, err := c.complete(ctx, "ChoreNudge", choreNudgeSystemPrompt, userPrompt, c.temperature)
	if err != nil {
		return "", err
	}

	nudge := strings.TrimSpace(content)
	if nudge == "" {
		return "", fmt.Errorf("LLM ChoreNudge returned empty content")
	}
	return SanitizeTelegramHTML(nudge), nil
}

// complete sends a system + user prompt to the configured provider and
// returns the generated text. op is used to prefix error messages.
// temperature is ignored by the Anthropic provider.
func (c *Client) complete(ctx context.Context, op, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if c.anthropic != nil {
		return c.anthropicCompletion(ctx, op, systemPrompt, userPrompt)
	}
	return c.chatCompletion(ctx, op, []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, temperature)
}

// chatCompletion performs a single OpenAI-compatible chat completion request
// and returns the content of the first choice. op is used to prefix error messages.
func (c *Client) chatCompletion(ctx context.Context, op string, messages []chatMessage, temperature float64) (string, error) {
	jsonBody, err := json.Marshal(chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
	})
	if err != nil {
		return "", fmt.Errorf("LLM %s JSON marshal error: %w", op, err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("LLM %s NewRequest error: %w", op, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM %s Do error: %w", op, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		// Truncate body to avoid logging sensitive information
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		return "", fmt.Errorf("LLM %s non-200 status: %d, body: %s", op, resp.StatusCode, bodyStr)
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("LLM %s decode error: %w", op, err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM %s no choices returned", op)
	}

	return chatResp.Choices[0].Message.Content, nil
}
