package handlers

import (
	"context"
	"io"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingImageReader struct {
	mediaType string
	data      string
}

func (r *recordingImageReader) DescribeImage(ctx context.Context, mediaType string, data []byte, caption string) (string, error) {
	r.mediaType, r.data = mediaType, string(data)
	return "a clean room", nil
}

func TestDescribeImage(t *testing.T) {
	ctx := context.Background()
	reader := &recordingImageReader{}
	files := map[string]string{
		"big":   "\xff\xd8\xff\xe0big-jpeg",
		"small": "\xff\xd8\xff\xe0small-jpeg",
		"png":   "\x89PNG\r\n\x1a\nimage",
		"pdf":   "%PDF-1.7",
	}
	var fetched []string
	h := NewWithAdminID(nil, nil, 0, 1, nil)
	h.ImageReader = reader
	h.FetchFile = func(ctx context.Context, fileID string) (io.ReadCloser, error) {
		fetched = append(fetched, fileID)
		return io.NopCloser(strings.NewReader(files[fileID])), nil
	}

	// The largest photo size that fits the limit is used.
	photo := &tgbotapi.Message{Photo: []tgbotapi.PhotoSize{
		{FileID: "small", FileSize: 1000}, {FileID: "big", FileSize: 200000}, {FileID: "huge", FileSize: maxImageBytes + 1},
	}}
	assert.True(t, HasImage(photo))
	got, err := h.DescribeImage(ctx, photo, "")
	require.NoError(t, err)
	assert.Equal(t, "a clean room", got)
	assert.Equal(t, []string{"big"}, fetched)
	assert.Equal(t, "image/jpeg", reader.mediaType)

	// Images sent as files count too; the type comes from the content.
	doc := &tgbotapi.Message{Document: &tgbotapi.Document{FileID: "png", MimeType: "image/png"}}
	_, err = h.DescribeImage(ctx, doc, "")
	require.NoError(t, err)
	assert.Equal(t, "image/png", reader.mediaType)

	// Other files are not images; fake image files are rejected.
	assert.False(t, HasImage(&tgbotapi.Message{Document: &tgbotapi.Document{FileID: "pdf", MimeType: "application/pdf"}}))
	_, err = h.DescribeImage(ctx, &tgbotapi.Message{Document: &tgbotapi.Document{FileID: "pdf", MimeType: "image/jpeg"}}, "")
	assert.ErrorContains(t, err, "unsupported image type")

	_, err = h.DescribeImage(ctx, &tgbotapi.Message{Text: "hi"}, "")
	assert.Error(t, err)

	h.ImageReader = nil
	assert.False(t, h.ImagesEnabled())
	_, err = h.DescribeImage(ctx, photo, "")
	assert.Error(t, err)
}
