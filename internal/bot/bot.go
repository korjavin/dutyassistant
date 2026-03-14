package bot

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/bot/fsm"
	"github.com/korjavin/dutyassistant/internal/bot/session"
	"github.com/korjavin/dutyassistant/internal/domain"
)

type Bot struct {
	api            *tgbotapi.BotAPI
	sessionManager *session.Manager
	dutyService    domain.DutyService
	choreService   domain.ChoreService
	ratingService  domain.RatingService
}

func NewBot(api *tgbotapi.BotAPI, ds domain.DutyService, cs domain.ChoreService, rs domain.RatingService) *Bot {
	return &Bot{
		api:            api,
		sessionManager: session.NewManager(),
		dutyService:    ds,
		choreService:   cs,
		ratingService:  rs,
	}
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
		b.handleCommand(msg)
		return
	}

	// Route to FSM if session exists
	sess := b.sessionManager.GetOrCreateSession(msg.From.ID, fsm.StateInit)
	if sess.FSM.CurrentState() != fsm.StateInit {
		b.processFSMState(msg, sess.FSM.CurrentState())
	}
}

func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	log.Printf("[CallbackQuery] %s", query.Data)
}
