package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestPollTelegramUpdatesSubscribesToReactions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	forms := make(chan *http.Request, 2)
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := `{"ok":true,"result":{"id":123456,"is_bot":true,"first_name":"TestBot"}}`
			if strings.HasSuffix(r.URL.Path, "/getUpdates") {
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					return nil, err
				}
				forms <- r
				if r.Form.Get("offset") != "1" {
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				body = `{"ok":true,"result":[
					{"update_id":41,"message":{"message_id":5,"text":"hello"}},
					{"update_id":42,"my_chat_member":{"chat":{"id":-1},
						"old_chat_member":{"status":"left","user":{"id":123456}},
						"new_chat_member":{"status":"member","user":{"id":123456}}}},
					{"update_id":43,"message_reaction":{"chat":{"id":-1},"message_id":6}},
					{"update_id":44,"message_reaction_count":{"chat":{"id":-1},"message_id":6}}
				]}`
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		}),
	}

	updates := make(chan *models.Update, 4)
	options := append(app.TelegramPollingOptions(), telegram.WithDefaultHandler(func(_ context.Context, _ *telegram.Bot, update *models.Update) {
		updates <- update
	}))
	bot := newMockTelegramBot(t, client, options...)
	done := make(chan struct{})
	go func() {
		defer close(done)
		bot.Start(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	deadline := time.After(3 * time.Second)
	var batch []*models.Update
	for len(batch) < 4 {
		select {
		case update := <-updates:
			batch = append(batch, update)
		case <-deadline:
			t.Fatal("poller did not deliver all four updates")
		}
	}
	if batch[0].Message == nil || batch[1].MyChatMember == nil || batch[2].MessageReaction == nil || batch[3].MessageReactionCount == nil {
		t.Fatalf("unexpected decoded updates: %+v", batch)
	}
	request := <-forms
	var allowed []string
	if err := json.Unmarshal([]byte(request.Form.Get("allowed_updates")), &allowed); err != nil ||
		!slices.Equal(allowed, []string{"message", "my_chat_member", "message_reaction", "message_reaction_count"}) {
		t.Fatalf("allowed updates = %v, error = %v", allowed, err)
	}
	select {
	case request = <-forms:
		if got := request.Form.Get("offset"); got != "45" {
			t.Fatalf("next getUpdates offset = %q, want 45", got)
		}
	case <-deadline:
		t.Fatal("poller did not acknowledge the batch")
	}
}

func namedReaction(t *testing.T, updateID int, chatID int64, messageID int, old, new string) models.Update {
	t.Helper()
	var update models.Update
	err := json.Unmarshal(fmt.Appendf(nil, `{"update_id":%d,"message_reaction":{
		"chat":{"id":%d},"message_id":%d,"date":%d,"user":{"id":42,"first_name":"Маша"},
		"old_reaction":%s,"new_reaction":%s}}`, updateID, chatID, messageID, time.Now().Unix(), old, new), &update)
	if err != nil {
		t.Fatal(err)
	}
	return update
}

func TestTelegramReactionMemory(t *testing.T) {
	db := setupBotDB(t)
	const chatID = -987654321
	app.EnableOutgoingMemory(db)
	t.Cleanup(func() { app.EnableOutgoingMemory(nil) })
	app.SendMessage(chatID, "Реплика бота", mockSendBot(t, true), 0)

	const fire = `[{"type":"emoji","emoji":"🔥"}]`
	const custom = `[{"type":"custom_emoji","custom_emoji_id":"123"}]`
	added := namedReaction(t, 100, chatID, 1, `[]`, fire)
	for _, update := range []models.Update{
		added,
		added,
		namedReaction(t, 101, chatID, 1, fire, custom),
		namedReaction(t, 102, chatID, 1, custom, `[]`),
		namedReaction(t, 103, chatID, 1, fire, fire),
		namedReaction(t, 104, chatID, 99, `[]`, fire),
		namedReaction(t, 105, chatID-1, 1, `[]`, fire),
	} {
		if err := app.RecordTelegramReaction(db, chatID, update); err != nil {
			t.Fatal(err)
		}
	}
	memory, err := app.LoadGeminiMemoryContext(db, chatID, 20, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	want := "bot: Реплика бота\n" +
		"Маша: [Реакция на сообщение бота #1] добавил 🔥 «Реплика бота»\n" +
		"Маша: [Реакция на сообщение бота #1] добавил кастомный эмодзи 123 и убрал 🔥 «Реплика бота»\n" +
		"Маша: [Реакция на сообщение бота #1] убрал кастомный эмодзи 123 «Реплика бота»"
	if memory != want {
		t.Fatalf("reaction memory:\n%s\nwant:\n%s", memory, want)
	}
	if _, err := deleteMyFacts(db, chatID, &models.User{ID: 42, FirstName: "Маша"}); err != nil {
		t.Fatal(err)
	}
	if memory, err = app.LoadGeminiMemoryContext(db, chatID, 20, time.Now().Add(-time.Hour)); err != nil || memory != "bot: Реплика бота" {
		t.Fatalf("memory after deleting the actor's data = %q, error = %v", memory, err)
	}
}

func TestTelegramAnonymousReactionWithoutMessageText(t *testing.T) {
	db := setupBotDB(t)
	const chatID = -987654321
	app.EnableOutgoingMemory(db)
	t.Cleanup(func() { app.EnableOutgoingMemory(nil) })
	app.SendMessageWithoutMemory(chatID, "приватный ответ /myfacts", mockSendBot(t, true), 0)

	var update models.Update
	if err := json.Unmarshal(fmt.Appendf(nil, `{"update_id":200,"message_reaction_count":{
		"chat":{"id":%d},"message_id":1,"date":%d,
		"reactions":[{"type":{"type":"emoji","emoji":"👍"},"total_count":3}]}}`, chatID, time.Now().Unix()), &update); err != nil {
		t.Fatal(err)
	}
	if err := app.RecordTelegramReaction(db, chatID, update); err != nil {
		t.Fatal(err)
	}
	want := "Telegram: [Анонимные реакции на сообщение бота #1] 👍 × 3"
	if memory, err := app.LoadGeminiMemoryContext(db, chatID, 20, time.Now().Add(-time.Hour)); err != nil || memory != want {
		t.Fatalf("anonymous reaction memory = %q, want %q, error = %v", memory, want, err)
	}
}

func TestTelegramReactionToGeminiReplyInNextPrompt(t *testing.T) {
	db := setupBotDB(t)
	synctest.Test(t, func(t *testing.T) {
		const chatID = -987654321
		prompts := make(chan string, 2)
		client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			var request app.GeminiRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				return nil, err
			}
			prompts <- request.Contents[0].Parts[0].Text
			body := `{"candidates":[{"content":{"parts":[{"text":"Ответ Gemini"}]}}]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
			DB: db, Bot: mockSendBot(t, true), Client: client,
			APIKey: "test-key", APIBaseURL: "https://gemini.test", DisableSearch: true,
		})
		if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
			t.Fatal("first message was not processed")
		}
		if got := waitForGeminiReply(t, db, chatID); got != "Ответ Gemini" {
			t.Fatalf("first reply = %q", got)
		}
		<-prompts
		update := namedReaction(t, 201, chatID, 1, `[]`, `[{"type":"emoji","emoji":"🔥"}]`)
		if err := app.RecordTelegramReaction(db, chatID, update); err != nil {
			t.Fatal(err)
		}
		if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
			t.Fatal("next message was not processed")
		}
		select {
		case prompt := <-prompts:
			if !strings.Contains(prompt, "Маша: [Реакция на сообщение бота #1] добавил 🔥 «Ответ Gemini»") {
				t.Fatalf("reaction missing from the next prompt: %q", prompt)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Gemini did not receive a second prompt")
		}
	})
}
