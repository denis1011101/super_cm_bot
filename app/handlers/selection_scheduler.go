package handlers

import (
	"context"
	"database/sql"
	"log"
	"math/rand/v2"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	selectionMinInterval = 11 * time.Hour
	selectionJitter      = 2 * time.Hour
	selectionRetryDelay  = 30 * time.Minute
)

// SelectionScheduler coordinates automatic and manual selections for one chat.
// selectionMu protects the schedule together with the command execution.
type SelectionScheduler struct {
	chatID  int64
	next    *selectionCommand
	readyAt time.Time
	timerAt time.Time
	changed chan struct{}
}

func NewSelectionScheduler(chatID int64) *SelectionScheduler {
	scheduler := &SelectionScheduler{
		chatID:  chatID,
		next:    &gigaSelection,
		changed: make(chan struct{}, 1),
	}
	scheduler.plan(time.Now())
	return scheduler
}

func (s *SelectionScheduler) ChooseGiga(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	gigaSelection.runManual(update, bot, db, s)
}

func (s *SelectionScheduler) ChooseUnhandsome(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	unhSelection.runManual(update, bot, db, s)
}

// TryAutomatic takes a Gemini suggestion when its turn is ready.
func (s *SelectionScheduler) TryAutomatic(message tgbotapi.Message, command string, bot *tgbotapi.BotAPI, db *sql.DB) {
	selectionMu.Lock()
	defer selectionMu.Unlock()

	if message.Chat == nil || message.Chat.ID != s.chatID || command != s.next.name || time.Now().Before(s.readyAt) {
		return
	}
	s.runAutomatic(&message, bot, db)
	s.wake()
}

// Run waits for the timer or for a manual or Gemini selection to move it.
func (s *SelectionScheduler) Run(ctx context.Context, bot *tgbotapi.BotAPI, db *sql.DB) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		selectionMu.Lock()
		wait := time.Until(s.timerAt)
		selectionMu.Unlock()
		timer.Reset(wait)
		select {
		case <-ctx.Done():
			return
		case <-s.changed:
		case <-timer.C:
			s.runScheduled(bot, db)
		}
	}
}

func (s *SelectionScheduler) runScheduled(bot *tgbotapi.BotAPI, db *sql.DB) {
	selectionMu.Lock()
	defer selectionMu.Unlock()

	if time.Now().Before(s.timerAt) {
		return
	}
	s.runAutomatic(&tgbotapi.Message{Chat: &tgbotapi.Chat{ID: s.chatID}}, bot, db)
}

func (s *SelectionScheduler) runAutomatic(message *tgbotapi.Message, bot *tgbotapi.BotAPI, db *sql.DB) {
	completed, retryAt := s.next.run(message, bot, db, true)
	if completed {
		s.advance(time.Now())
		return
	}
	if retryAt.IsZero() {
		retryAt = time.Now().Add(selectionRetryDelay)
	}
	s.readyAt = retryAt
	if s.timerAt.Before(retryAt) {
		s.timerAt = retryAt
	}
	log.Printf("Next scheduled %s attempt at %s (retry)", s.next.name, s.timerAt.Format(time.RFC3339))
}

func (s *SelectionScheduler) advance(at time.Time) {
	if s.next == &gigaSelection {
		s.next = &unhSelection
	} else {
		s.next = &gigaSelection
	}
	s.plan(at)
}

func (s *SelectionScheduler) plan(at time.Time) {
	s.readyAt = at.Add(selectionMinInterval)
	s.timerAt = s.readyAt.Add(rand.N(selectionJitter))
	log.Printf("Next scheduled %s attempt at %s", s.next.name, s.timerAt.Format(time.RFC3339))
}

func (s *SelectionScheduler) wake() {
	select {
	case s.changed <- struct{}{}:
	default:
	}
}
