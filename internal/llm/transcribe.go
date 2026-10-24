package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTranscriptionURL   = "https://api.openai.com/v1"
	defaultTranscriptionModel = "whisper-1"
)

// Transcriber turns voice messages into text through an OpenAI-compatible
// /audio/transcriptions endpoint (OpenAI, Groq, a local whisper server, ...).
// The Claude API does not accept audio, so speech-to-text is separate from the
// LLM provider.
type Transcriber struct {
	apiKey     string
	baseURL    string
	model      string
	language   string
	httpClient *http.Client
}

// NewTranscriber creates a speech-to-text client. If apiKey is empty, it
// returns nil, which disables voice messages. language is an optional
// ISO-639-1 hint such as "ru"; empty lets the service detect it.
func NewTranscriber(apiKey, baseURL string, timeoutSeconds int, model, language string) *Transcriber {
	if apiKey == "" {
		return nil
	}
	if baseURL == "" {
		baseURL = defaultTranscriptionURL
	}
	if model == "" {
		model = defaultTranscriptionModel
	}
	return &Transcriber{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		language:   language,
		httpClient: &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

// Config returns the model and base URL used for transcription.
func (t *Transcriber) Config() (string, string) {
	if t == nil {
		return "", ""
	}
	return t.model, t.baseURL
}

// Transcribe sends the audio to the speech-to-text service and returns the
// recognized text. filename tells the service the format (e.g. "voice.ogg").
func (t *Transcriber) Transcribe(ctx context.Context, audio io.Reader, filename string) (string, error) {
	if t == nil {
		return "", fmt.Errorf("transcription is not configured")
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("transcription form error: %w", err)
	}
	if _, err := io.Copy(part, audio); err != nil {
		return "", fmt.Errorf("transcription read audio error: %w", err)
	}
	fields := map[string]string{"model": t.model, "response_format": "json"}
	if t.language != "" {
		fields["language"] = t.language
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return "", fmt.Errorf("transcription form error: %w", err)
		}
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("transcription form error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/audio/transcriptions", &body)
	if err != nil {
		return "", fmt.Errorf("transcription NewRequest error: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcription Do error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return "", fmt.Errorf("transcription non-200 status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var out struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("transcription decode error: %w", err)
	}
	return strings.TrimSpace(out.Text), nil
}
