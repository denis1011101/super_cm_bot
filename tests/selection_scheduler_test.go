package tests

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	"github.com/denis1011101/super_cm_bot/app/handlers"
	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type selectionTestChat struct {
	db        *sql.DB
	bot       *telegram.Bot
	update    models.Update
	repliesMu sync.Mutex
	replies   []string
}

func newSelectionTestChat(t *testing.T) *selectionTestChat {
	t.Helper()
	const chatID = -123
	chat := &selectionTestChat{
		db: setupBotDB(t),
		update: models.Update{Message: &models.Message{
			From: &models.User{ID: 1}, Chat: models.Chat{ID: chatID},
		}},
	}
	insertPen(t, chat.db, 1, chatID, nil)
	insertPen(t, chat.db, 2, chatID, nil)

	chat.bot = newMockTelegramBot(t, &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if strings.HasSuffix(r.URL.Path, "/sendMessage") {
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					return nil, err
				}
				chat.repliesMu.Lock()
				chat.replies = append(chat.replies, r.Form.Get("text"))
				chat.repliesMu.Unlock()
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{}}`))}, nil
		}),
	})
	return chat
}

func (c *selectionTestChat) expectReplies(t *testing.T, want int) []string {
	t.Helper()
	synctest.Wait()
	c.repliesMu.Lock()
	defer c.repliesMu.Unlock()
	if len(c.replies) != want {
		t.Fatalf("got %d replies %q, want %d", len(c.replies), c.replies, want)
	}
	return slices.Clone(c.replies)
}

func TestSelectionSchedulerAlternatesTimerAndGemini(t *testing.T) {
	chat := newSelectionTestChat(t)
	synctest.Test(t, func(t *testing.T) {
		chatID := chat.update.Message.Chat.ID
		scheduler := handlers.NewSelectionScheduler(chatID)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go scheduler.Run(ctx, chat.bot, chat.db)

		message := *chat.update.Message
		scheduler.TryAutomatic(message, "/giga", chat.bot, chat.db)
		chat.expectReplies(t, 0)

		time.Sleep(13 * time.Hour)
		chat.expectReplies(t, 1)
		firstGiga, err := app.GetGigaLastUpdateTime(chat.db, chatID)
		if err != nil || firstGiga.IsZero() {
			t.Fatalf("first Giga update = %v, error = %v", firstGiga, err)
		}

		time.Sleep(time.Until(firstGiga.Add(11*time.Hour)) + time.Nanosecond)
		scheduler.TryAutomatic(message, "/giga", chat.bot, chat.db)
		scheduler.TryAutomatic(message, "/unh", chat.bot, chat.db)
		scheduler.TryAutomatic(message, "/unh", chat.bot, chat.db)
		chat.expectReplies(t, 2)
		unh, err := app.GetUnhandsomeLastUpdateTime(chat.db, chatID)
		if err != nil || unh.Before(firstGiga.Add(11*time.Hour)) {
			t.Fatalf("UNH update = %v after %v, error = %v", unh, firstGiga, err)
		}

		time.Sleep(13 * time.Hour)
		chat.expectReplies(t, 3)
		lastGiga, err := app.GetGigaLastUpdateTime(chat.db, chatID)
		if err != nil || lastGiga.Before(unh.Add(11*time.Hour)) {
			t.Fatalf("second Giga update = %v after %v, error = %v", lastGiga, unh, err)
		}
	})
}

func TestSelectionSchedulerCreditsManualTurn(t *testing.T) {
	chat := newSelectionTestChat(t)
	synctest.Test(t, func(t *testing.T) {
		chatID := chat.update.Message.Chat.ID
		scheduler := handlers.NewSelectionScheduler(chatID)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go scheduler.Run(ctx, chat.bot, chat.db)

		scheduler.ChooseUnhandsome(chat.update, chat.bot, chat.db)
		time.Sleep(3 * time.Hour)
		scheduler.ChooseGiga(chat.update, chat.bot, chat.db)
		gigaAt := time.Now()
		chat.expectReplies(t, 2)

		// The old timer would have fired by now.
		time.Sleep(10 * time.Hour)
		chat.expectReplies(t, 2)
		time.Sleep(3 * time.Hour)
		chat.expectReplies(t, 3)
		unh, err := app.GetUnhandsomeLastUpdateTime(chat.db, chatID)
		if err != nil || unh.Before(gigaAt.Add(11*time.Hour)) {
			t.Fatalf("UNH update = %v after manual Giga at %v, error = %v", unh, gigaAt, err)
		}
	})
}

func TestSelectionSchedulerRetriesCooldownSilently(t *testing.T) {
	chat := newSelectionTestChat(t)
	synctest.Test(t, func(t *testing.T) {
		scheduler := handlers.NewSelectionScheduler(chat.update.Message.Chat.ID)
		time.Sleep(11 * time.Hour)
		if _, err := chat.db.Exec(`UPDATE pens SET handsome_last_update_at = ?`, time.Now().Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go scheduler.Run(ctx, chat.bot, chat.db)
		scheduler.TryAutomatic(*chat.update.Message, "/giga", chat.bot, chat.db)
		chat.expectReplies(t, 0)

		scheduler.ChooseGiga(chat.update, chat.bot, chat.db)
		replies := chat.expectReplies(t, 1)
		if !strings.Contains(replies[0], "Приходи позже") {
			t.Fatalf("manual command sent %q, want a cooldown reply", replies[0])
		}

		time.Sleep(3*time.Hour - time.Second)
		chat.expectReplies(t, 1)
		time.Sleep(time.Second)
		replies = chat.expectReplies(t, 2)
		if strings.Contains(replies[1], "Приходи позже") {
			t.Fatalf("automatic retry sent a cooldown reply: %q", replies[1])
		}
	})
}
