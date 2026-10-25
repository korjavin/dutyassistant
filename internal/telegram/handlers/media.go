package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Media the bot turns into text, so it can be handled like a typed message:
// voice messages / audio / video notes are transcribed, images described.

// SpeechTranscriber turns a voice message into text.
type SpeechTranscriber interface {
	Transcribe(ctx context.Context, audio io.Reader, filename string) (string, error)
}

// ImageReader describes an image as text (including any text shown in it).
type ImageReader interface {
	DescribeImage(ctx context.Context, mediaType string, data []byte, caption string) (string, error)
}

const (
	// maxVoiceBytes is the largest file the Telegram Bot API lets bots download.
	maxVoiceBytes = 20 << 20
	// maxImageBytes is the largest image the Claude API accepts.
	maxImageBytes = 5 << 20
)

// mediaFile is a downloadable attachment of a message.
type mediaFile struct {
	FileID   string
	FileName string
	Size     int
}

// audioOf returns the voice message, audio file or video note of m, if any.
func audioOf(m *tgbotapi.Message) *mediaFile {
	switch {
	case m.Voice != nil:
		return &mediaFile{FileID: m.Voice.FileID, FileName: "voice.ogg", Size: m.Voice.FileSize}
	case m.Audio != nil:
		name := m.Audio.FileName
		if name == "" {
			name = "audio.mp3"
		}
		return &mediaFile{FileID: m.Audio.FileID, FileName: name, Size: m.Audio.FileSize}
	case m.VideoNote != nil:
		return &mediaFile{FileID: m.VideoNote.FileID, FileName: "video_note.mp4", Size: m.VideoNote.FileSize}
	}
	return nil
}

// imageOf returns the photo of m (its largest size that fits the image
// limit) or an image sent as a file, if any.
func imageOf(m *tgbotapi.Message) *mediaFile {
	if len(m.Photo) > 0 {
		// Sizes come smallest first.
		for i := len(m.Photo) - 1; i >= 0; i-- {
			if p := m.Photo[i]; p.FileSize <= maxImageBytes {
				return &mediaFile{FileID: p.FileID, FileName: "photo.jpg", Size: p.FileSize}
			}
		}
		return &mediaFile{FileID: m.Photo[0].FileID, FileName: "photo.jpg", Size: m.Photo[0].FileSize}
	}
	if d := m.Document; d != nil && strings.HasPrefix(d.MimeType, "image/") {
		return &mediaFile{FileID: d.FileID, FileName: d.FileName, Size: d.FileSize}
	}
	return nil
}

// HasVoice reports whether m carries a voice message, audio file or video note.
func HasVoice(m *tgbotapi.Message) bool {
	return audioOf(m) != nil
}

// HasImage reports whether m carries a photo or an image file.
func HasImage(m *tgbotapi.Message) bool {
	return imageOf(m) != nil
}

// VoiceEnabled reports whether voice messages can be transcribed.
func (h *Handlers) VoiceEnabled() bool {
	return h.Transcriber != nil
}

// ImagesEnabled reports whether images can be described.
func (h *Handlers) ImagesEnabled() bool {
	return h.ImageReader != nil
}

// TranscribeVoice downloads and transcribes the voice message, audio file or
// video note of m.
func (h *Handlers) TranscribeVoice(ctx context.Context, m *tgbotapi.Message) (string, error) {
	audio := audioOf(m)
	switch {
	case audio == nil:
		return "", fmt.Errorf("the message has no voice")
	case h.Transcriber == nil:
		return "", fmt.Errorf("transcription is not configured")
	case audio.Size > maxVoiceBytes:
		return "", fmt.Errorf("the recording is too large (%d bytes)", audio.Size)
	}
	body, err := h.fetch(ctx, audio.FileID)
	if err != nil {
		return "", err
	}
	defer func() { _ = body.Close() }()
	return h.Transcriber.Transcribe(ctx, io.LimitReader(body, maxVoiceBytes), audio.FileName)
}

// DescribeImage downloads the photo or image file of m and describes it as
// text; caption is the text sent with it.
func (h *Handlers) DescribeImage(ctx context.Context, m *tgbotapi.Message, caption string) (string, error) {
	img := imageOf(m)
	switch {
	case img == nil:
		return "", fmt.Errorf("the message has no image")
	case h.ImageReader == nil:
		return "", fmt.Errorf("reading images is not configured")
	case img.Size > maxImageBytes:
		return "", fmt.Errorf("the image is too large (%d bytes)", img.Size)
	}
	body, err := h.fetch(ctx, img.FileID)
	if err != nil {
		return "", err
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(io.LimitReader(body, maxImageBytes+1))
	if err != nil {
		return "", fmt.Errorf("could not read the image: %w", err)
	}
	if len(data) > maxImageBytes {
		return "", fmt.Errorf("the image is too large")
	}
	mediaType := http.DetectContentType(data)
	switch mediaType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
	default:
		return "", fmt.Errorf("unsupported image type %s", mediaType)
	}
	return h.ImageReader.DescribeImage(ctx, mediaType, data, caption)
}

func (h *Handlers) fetch(ctx context.Context, fileID string) (io.ReadCloser, error) {
	if h.FetchFile != nil {
		return h.FetchFile(ctx, fileID)
	}
	return h.downloadTelegramFile(ctx, fileID)
}

// downloadTelegramFile streams a file the bot received.
func (h *Handlers) downloadTelegramFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	if h.Bot == nil {
		return nil, fmt.Errorf("bot not configured")
	}
	fileURL, err := h.Bot.GetFileDirectURL(fileID)
	if err != nil {
		return nil, fmt.Errorf("could not get the file from Telegram: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build the download request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// The URL contains the bot token: keep it out of errors and logs.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("could not download the file: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("could not download the file: status %d", resp.StatusCode)
	}
	return resp.Body, nil
}
