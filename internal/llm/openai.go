package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Client is an adapter for calling the OpenAI chat completion API.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new LLM client. If apiKey is empty, it returns nil,
// effectively disabling the LLM features.
func NewClient(apiKey, baseURL string, timeoutSeconds int) *Client {
	if apiKey == "" {
		return nil
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
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

// RefineMessage sends the vanilla message and intent to the LLM to get a more
// fun/humorous formatted message. It returns the vanilla message if there's any error
// or if the client is nil.
func (c *Client) RefineMessage(ctx context.Context, intent, vanilla string) string {
	if c == nil {
		return vanilla
	}

	systemPrompt := `You are a fun, witty, and enthusiastic assistant managing household chores.
Your task is to reformat the given message based on the specified intent while keeping the core information intact.
Return ONLY the formatted text. Use emojis appropriately. Do NOT wrap the output in quotes or markdown code blocks.`

	userPrompt := fmt.Sprintf("Intent: %s\nOriginal Message: %s", intent, vanilla)

	reqBody := chatRequest{
		Model: "gpt-4o-mini", // default fast/cheap model
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.7,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("LLM RefineMessage JSON marshal error: %v", err)
		return vanilla
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("LLM RefineMessage NewRequest error: %v", err)
		return vanilla
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("LLM RefineMessage Do error: %v", err)
		return vanilla
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("LLM RefineMessage non-200 status: %d, body: %s", resp.StatusCode, string(bodyBytes))
		return vanilla
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		log.Printf("LLM RefineMessage decode error: %v", err)
		return vanilla
	}

	if len(chatResp.Choices) == 0 {
		log.Printf("LLM RefineMessage no choices returned")
		return vanilla
	}

	refined := chatResp.Choices[0].Message.Content
	if refined == "" {
		return vanilla
	}

	return refined
}
