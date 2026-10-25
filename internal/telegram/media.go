package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
)

const (
	voiceUnavailableMessage      = "🎙 I can't listen to voice messages yet — please type instead."
	adminVoiceUnavailableMessage = "🎙 Voice messages need speech-to-text: set OPENAI_API_KEY (or TRANSCRIPTION_API_KEY) and restart the bot."
	imageUnavailableMessage      = "🖼 I can't look at pictures yet — please describe it in words."
	adminImageUnavailableMessage = "🖼 Reading pictures needs the Anthropic LLM provider: set ANTHROPIC_API_KEY and restart the bot."
	// mediaReadTimeout bounds downloading and transcribing/describing one message.
	mediaReadTimeout = 3 * time.Minute
)

// withCaptionAsText returns m with its caption (and caption entities) as the
// text when it has no text, so photos and voice messages with a caption are
// read, and mention the bot, like typed messages.
func withCaptionAsText(m *tgbotapi.Message) *tgbotapi.Message {
	if m.Text != "" || m.Caption == "" {
		return m
	}
	c := *m
	c.Text, c.Entities = m.Caption, m.CaptionEntities
	return &c
}

// hasMedia reports whether m carries something the bot turns into text.
func hasMedia(m *tgbotapi.Message) bool {
	return handlers.HasVoice(m) || handlers.HasImage(m)
}

// handleMedia handles a voice message, audio file, video note or image: it
// is transcribed or described and then handled exactly like the resulting
// text typed by the sender (session input, assistant request, report part).
func (b *Bot) handleMedia(m *tgbotapi.Message) {
	voice := handlers.HasVoice(m)
	if voice && !b.handlers.VoiceEnabled() || !voice && !b.handlers.ImagesEnabled() {
		switch {
		case strings.TrimSpace(m.Text) != "":
			// The caption alone still says something.
			response, err := b.routeMessage(m)
			b.respond(m.Chat.ID, response, err)
		case m.Chat.IsPrivate():
			b.respond(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, b.mediaUnavailableMessage(m.From.ID, voice)), nil)
		}
		return
	}
	// Downloading and reading takes a while; don't block other updates.
	go b.readAndRoute(context.Background(), m)
}

func (b *Bot) mediaUnavailableMessage(userID int64, voice bool) string {
	admin := b.handlers.IsAdmin(userID)
	switch {
	case voice && admin:
		return adminVoiceUnavailableMessage
	case voice:
		return voiceUnavailableMessage
	case admin:
		return adminImageUnavailableMessage
	}
	return imageUnavailableMessage
}

// readAndRoute turns m into text, shows the sender what was heard or seen
// and routes the text. Media messages of one chat are handled in order.
func (b *Bot) readAndRoute(ctx context.Context, m *tgbotapi.Message) {
	lock, _ := b.mediaLocks.LoadOrStore(m.Chat.ID, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	reply := func(text string) {
		msg := tgbotapi.NewMessage(m.Chat.ID, text)
		if !m.Chat.IsPrivate() {
			msg.ReplyToMessageID = m.MessageID
		}
		b.respond(m.Chat.ID, msg, nil)
	}

	caption := strings.TrimSpace(m.Text)
	voice := handlers.HasVoice(m)
	rctx, cancel := context.WithTimeout(ctx, mediaReadTimeout)
	var content string
	var err error
	if voice {
		content, err = b.handlers.TranscribeVoice(rctx, m)
	} else {
		content, err = b.handlers.DescribeImage(rctx, m, caption)
	}
	cancel()
	content = strings.TrimSpace(content)

	var text string
	switch {
	case err != nil && voice:
		slog.Error(fmt.Sprintf("[Media] Failed to transcribe a message from %d: %v", m.From.ID, err))
		reply("😕 I couldn't make out that voice message. Please try again or type it.")
		return
	case err != nil:
		slog.Error(fmt.Sprintf("[Media] Failed to read an image from %d: %v", m.From.ID, err))
		reply("😕 I couldn't look at that picture. Please try again or describe it in words.")
		return
	case content == "" && voice:
		reply("🤔 I didn't hear any words in that recording. Please try again.")
		return
	case voice:
		reply("🎧 I heard: «" + handlers.TruncateRunes(content, 1500) + "»")
		text = joinNonEmpty(caption, content)
	case content == "":
		reply("🤔 I couldn't tell what's in that picture.")
		return
	default:
		reply("🖼 I see: " + handlers.TruncateRunes(content, 1500))
		text = joinNonEmpty(caption, "[Photo: "+content+"]")
	}
	slog.Info(fmt.Sprintf("[Media] Read a message from %d as %d chars of text", m.From.ID, len(text)))

	// The caption, if any, comes first, so its entities (mentions) still fit.
	typed := *m
	typed.Text = text
	typed.Voice, typed.Audio, typed.VideoNote, typed.Photo, typed.Document = nil, nil, nil, nil, nil
	response, err := b.routeMessage(&typed)
	b.respond(m.Chat.ID, response, err)
}

func joinNonEmpty(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n")
}
