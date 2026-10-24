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
	// voiceTranscribeTimeout bounds downloading and transcribing one message.
	voiceTranscribeTimeout = 3 * time.Minute
)

// handleVoice handles a voice message, audio file or video note: it is
// transcribed and then handled exactly like the same text typed by the
// sender (session input, assistant request, daily report, ...).
func (b *Bot) handleVoice(m *tgbotapi.Message) {
	if !b.handlers.VoiceEnabled() {
		if m.Chat.IsPrivate() {
			text := voiceUnavailableMessage
			if b.handlers.IsAdmin(m.From.ID) {
				text = adminVoiceUnavailableMessage
			}
			b.respond(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, text), nil)
		}
		return
	}
	// Downloading and transcribing takes a while; don't block other updates.
	go b.transcribeAndRoute(context.Background(), m)
}

// transcribeAndRoute transcribes m, shows the sender what was heard and
// routes the text. Voice messages of one chat are handled in order.
func (b *Bot) transcribeAndRoute(ctx context.Context, m *tgbotapi.Message) {
	lock, _ := b.voiceLocks.LoadOrStore(m.Chat.ID, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	reply := func(text string) {
		msg := tgbotapi.NewMessage(m.Chat.ID, text)
		if !m.Chat.IsPrivate() {
			msg.ReplyToMessageID = m.MessageID
		}
		b.respond(m.Chat.ID, msg, nil)
	}

	tctx, cancel := context.WithTimeout(ctx, voiceTranscribeTimeout)
	text, err := b.handlers.TranscribeVoice(tctx, m)
	cancel()
	if err != nil {
		slog.Error(fmt.Sprintf("[Voice] Failed to transcribe a message from %d: %v", m.From.ID, err))
		reply("😕 I couldn't make out that voice message. Please try again or type it.")
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		reply("🤔 I didn't hear any words in that recording. Please try again.")
		return
	}
	slog.Info(fmt.Sprintf("[Voice] Transcribed a message from %d (%d chars)", m.From.ID, len(text)))
	reply("🎧 I heard: «" + handlers.TruncateRunes(text, 1500) + "»")

	typed := *m
	typed.Text = text
	typed.Voice, typed.Audio, typed.VideoNote = nil, nil, nil
	response, err := b.routeMessage(&typed)
	b.respond(m.Chat.ID, response, err)
}
