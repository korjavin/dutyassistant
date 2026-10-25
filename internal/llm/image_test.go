package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDescribeImage(t *testing.T) {
	var body map[string]any
	server := anthropicServer(t, http.StatusOK, messageReply("end_turn", `{"type":"text","text":" Чистая комната, кровать заправлена. "}`), &body)
	defer server.Close()

	c := NewAnthropicClient("test-key", server.URL, 10, "", "")
	got, err := c.DescribeImage(context.Background(), "image/jpeg", []byte("JPEG"), "вот моя комната")
	require.NoError(t, err)
	assert.Equal(t, "Чистая комната, кровать заправлена.", got)

	raw, _ := json.Marshal(body)
	assert.Contains(t, string(raw), `"media_type":"image/jpeg"`)
	assert.Contains(t, string(raw), `"data":"SlBFRw=="`) // base64("JPEG")
	assert.Contains(t, string(raw), "Caption: вот моя комната")
	assert.Contains(t, string(raw), "Transcribe any readable text")
}

func TestDescribeImage_NeedsAnthropic(t *testing.T) {
	_, err := NewClient("key", "", 5, "", nil).DescribeImage(context.Background(), "image/jpeg", []byte("x"), "")
	assert.ErrorContains(t, err, "Anthropic")

	var nilClient *Client
	_, err = nilClient.DescribeImage(context.Background(), "image/jpeg", []byte("x"), "")
	assert.Error(t, err)
}
