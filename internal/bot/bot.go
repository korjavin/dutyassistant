package bot

import (
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/bot/fsm"
	"github.com/korjavin/dutyassistant/internal/bot/session"
	"github.com/korjavin/dutyassistant/internal/domain"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
)

type Bot struct {
	api            *tgbotapi.BotAPI
	sessionManager *session.Manager
	repo           domain.Repository
	dutyService    domain.DutyService
	choreService   domain.ChoreService
	ratingService  domain.RatingService
	legacyHandlers *handlers.Handlers
}

func NewBot(api *tgbotapi.BotAPI, repo domain.Repository, ds domain.DutyService, cs domain.ChoreService, rs domain.RatingService) *Bot {
	return &Bot{
		api:            api,
		sessionManager: session.NewManager(),
		repo:           repo,
		dutyService:    ds,
		choreService:   cs,
		ratingService:  rs,
	}
}

func NewBotWithLegacy(api *tgbotapi.BotAPI, repo domain.Repository, ds domain.DutyService, cs domain.ChoreService, rs domain.RatingService, lh *handlers.Handlers) *Bot {
	b := NewBot(api, repo, ds, cs, rs)
	b.legacyHandlers = lh
	lh.SetBot(api)
	return b
}

func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			b.handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			b.handleCallbackQuery(update.CallbackQuery)
		}
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	log.Printf("[%s] %s", msg.From.UserName, msg.Text)

	if msg.IsCommand() {
		// A bare cancel command has no arguments
		isBareCancel := msg.Command() == "cancel" && strings.TrimSpace(msg.CommandArguments()) == ""

		// If in a legacy session, intercept bare /cancel to allow the session to abort gracefully
		if b.legacyHandlers != nil {
			_, inSession := b.legacyHandlers.SessionManager.GetSession(msg.Chat.ID)
			if inSession && isBareCancel {
				b.handleLegacyMessage(msg)
				return
			}
		}

		b.handleCommand(msg)
		return
	}

	sess := b.sessionManager.GetOrCreateSession(msg.From.ID, fsm.StateInit)
	if sess.FSM.CurrentState() != fsm.StateInit {
		b.processFSMState(msg, sess.FSM.CurrentState())
		return
	}

	b.handleLegacyMessage(msg)
}

func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	log.Printf("[CallbackQuery] %s", query.Data)
	b.handleLegacyCallback(query)
}
