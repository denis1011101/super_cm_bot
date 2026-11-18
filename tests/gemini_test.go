package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	_ "unsafe"

	"github.com/denis1011101/super_cm_bot/app"
	"github.com/denis1011101/super_cm_bot/app/agentbus"
	"github.com/denis1011101/super_cm_bot/tests/testutils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

//go:linkname parseTextFromGenericResponse github.com/denis1011101/super_cm_bot/app.parseTextFromGenericResponse
func parseTextFromGenericResponse(b []byte) string

//go:linkname getPersonas github.com/denis1011101/super_cm_bot/app.getPersonas
func getPersonas(userText string) (string, string)

//go:linkname memorySpeakerName github.com/denis1011101/super_cm_bot/app.memorySpeakerName
func memorySpeakerName(user *tgbotapi.User) string

//go:linkname extractGeminiSaveFacts github.com/denis1011101/super_cm_bot/app.extractGeminiSaveFacts
func extractGeminiSaveFacts(raw string) (string, []app.GeminiUserFact)

//go:linkname extractGeminiAutoCommand github.com/denis1011101/super_cm_bot/app.extractGeminiAutoCommand
func extractGeminiAutoCommand(raw string) (string, string)

//go:linkname randomGeminiResponseDelay github.com/denis1011101/super_cm_bot/app.randomGeminiResponseDelay
func randomGeminiResponseDelay() time.Duration

//go:linkname normalizeGeminiModel github.com/denis1011101/super_cm_bot/app.normalizeGeminiModel
func normalizeGeminiModel(model string) string

//go:linkname resolveGeminiModel github.com/denis1011101/super_cm_bot/app.resolveGeminiModel
func resolveGeminiModel(raw string) string

//go:linkname resolveGeminiSearchModel github.com/denis1011101/super_cm_bot/app.resolveGeminiSearchModel
func resolveGeminiSearchModel(raw string, disabled bool) string

//go:linkname geminiAgentCallLLM github.com/denis1011101/super_cm_bot/app.(*GeminiAgent).callLLM
func geminiAgentCallLLM(a *app.GeminiAgent, ctx context.Context, systemPrompt, userPrompt, searchQuery string, useSearch bool) (string, error)

//go:linkname defaultGeminiFallbacks github.com/denis1011101/super_cm_bot/app.defaultGeminiFallbacks
func defaultGeminiFallbacks(primary string) []string

//go:linkname normalizeGeminiFallbackModels github.com/denis1011101/super_cm_bot/app.normalizeGeminiFallbackModels
func normalizeGeminiFallbackModels(primary string, models []string) []string

//go:linkname geminiAgentCanExecuteAutoCommand github.com/denis1011101/super_cm_bot/app.(*GeminiAgent).canExecuteAutoCommand
func geminiAgentCanExecuteAutoCommand(a *app.GeminiAgent, chatID int64, cmd string) bool

func setupGeminiDB(t *testing.T) *sql.DB {
	t.Helper()

	_, teardown := testutils.SetupTestEnvironment(t, false)
	t.Cleanup(teardown)

	db, err := app.InitDB()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Error closing database: %v", err)
		}
	})

	return db
}

func TestParseTextFromGenericResponse_VariousFormats(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"text_field", `{"text":"hello"}`, "hello"},
		{"choices_text", `{"choices":[{"text":"choice text"}]}`, "choice text"},
		{"candidates_parts", `{"candidates":[{"content":{"parts":[{"text":"candidate text"}]}}]}`, "candidate text"},
		{"candidates_output_string", `{"candidates":[{"output":"outstr"}]}`, "outstr"},
		{"output_content_format", `{"output":[{"content":[{"text":"out content"}]}]}`, "out content"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseTextFromGenericResponse([]byte(c.in))
			if got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestParseTextFromGenericResponse_ExtraAndFallbackCases(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"candidates_content_string", `{"candidates":[{"content":"just text"}]}`, "just text"},
		{"trimmed_text_field", `{"text":"   padded   "}`, "padded"},
		{"raw_fallback_for_unknown_shape", `{"foo":"bar"}`, `{"foo":"bar"}`},
		{"invalid_json", `{bad json`, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseTextFromGenericResponse([]byte(c.in))
			if got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestGetPersonas_FlirtyOnly(t *testing.T) {
	sys, user := getPersonas("Привет")
	if sys == "" {
		t.Fatalf("system instruction should not be empty")
	}
	if user != "Привет" {
		t.Fatalf("user text should be preserved, got %q", user)
	}
	if !strings.Contains(sys, "flirty") {
		t.Fatalf("expected flirty persona, got: %q", sys)
	}
	if !strings.Contains(sys, "NSFW") {
		t.Fatalf("system instruction should include safety rules, got: %q", sys)
	}
}

func TestMemorySpeakerName(t *testing.T) {
	cases := []struct {
		name string
		user *tgbotapi.User
		want string
	}{
		{
			name: "first_name_preferred",
			user: &tgbotapi.User{FirstName: "Денис", UserName: "denis1011101"},
			want: "Денис",
		},
		{
			name: "username_fallback",
			user: &tgbotapi.User{UserName: "dima"},
			want: "dima",
		},
		{
			name: "sanitizes_role",
			user: &tgbotapi.User{FirstName: "  Вася:\nПупкин  "},
			want: "Вася Пупкин",
		},
		{
			name: "nil_user_fallback",
			user: nil,
			want: "user",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := memorySpeakerName(c.user); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestExtractGeminiSaveFacts(t *testing.T) {
	raw := "Красавчик, поздравляю! 🔥 [SAVE: Denis — сдал экзамен по Go] [SAVE: Дима — фанат заднего привода]"

	cleaned, facts := extractGeminiSaveFacts(raw)

	if cleaned != "Красавчик, поздравляю! 🔥" {
		t.Fatalf("cleaned reply: got %q", cleaned)
	}

	want := []app.GeminiUserFact{
		{UserName: "Denis", Fact: "сдал экзамен по Go"},
		{UserName: "Дима", Fact: "фанат заднего привода"},
	}
	if len(facts) != len(want) {
		t.Fatalf("facts count: got %d want %d", len(facts), len(want))
	}
	for i := range want {
		if facts[i] != want[i] {
			t.Fatalf("fact %d: got %+v want %+v", i, facts[i], want[i])
		}
	}
}

func TestExtractGeminiAutoCommand(t *testing.T) {
	raw := "Ну что, погнали 😉 [CMD: /giga]"

	cleaned, cmd := extractGeminiAutoCommand(raw)

	if cleaned != "Ну что, погнали 😉" {
		t.Fatalf("cleaned reply: got %q", cleaned)
	}
	if cmd != "/giga" {
		t.Fatalf("cmd: got %q want %q", cmd, "/giga")
	}
}

func TestGeminiAgentAutoCommand_UsesConfiguredCooldown(t *testing.T) {
	db := setupGeminiDB(t)
	now := time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	lastUpdate := now.Add(-5 * time.Second).Format("2006-01-02 15:04:05Z07:00")
	chatID := int64(100042)

	_, err := db.Exec(`
		INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length, handsome_count, handsome_last_update_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "tester", 12345, chatID, 5, 0, lastUpdate)
	if err != nil {
		t.Fatalf("insert test command timestamp: %v", err)
	}

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:  db,
		Now: func() time.Time { return now },
	})
	if geminiAgentCanExecuteAutoCommand(agent, chatID, "/giga") {
		t.Fatal("default production cooldown should block a command run 5 seconds ago")
	}

	agent.SetSpinCooldown(4 * time.Second)
	if !geminiAgentCanExecuteAutoCommand(agent, chatID, "/giga") {
		t.Fatal("configured development cooldown should allow a command run 5 seconds ago")
	}
}

func TestGeminiAgentTryRespond_SetsRandomCooldown(t *testing.T) {
	db := setupGeminiDB(t)
	agent := app.NewGeminiAgent(db, nil)

	chatID := int64(99999)
	now := time.Now()

	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			From: &tgbotapi.User{IsBot: false},
			Text: "hello",
			Date: int(now.Unix()),
		},
	}

	ok := agent.TryRespond(upd, chatID)
	if !ok {
		t.Fatalf("TryRespond should have started processing")
	}
	if agent.TryRespond(upd, chatID) {
		t.Fatalf("expected second TryRespond call to be blocked by cooldown")
	}
}

func TestRandomGeminiResponseDelayRange(t *testing.T) {
	for i := 0; i < 1000; i++ {
		delay := randomGeminiResponseDelay()
		if delay < 2*time.Second || delay > 4*time.Second {
			t.Fatalf("delay %v is outside 2-4 second range", delay)
		}
	}
}

func TestGeminiAgentTryRespond_DebouncesAndCombinesMessages(t *testing.T) {
	db := setupGeminiDB(t)
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ"}]}}]}`))
	}))
	defer server.Close()

	now := time.Now()
	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:         db,
		Client:     server.Client(),
		Now:        time.Now,
		APIKey:     "test-key",
		APIBaseURL: server.URL,
		ResponseDelay: func() time.Duration {
			return 40 * time.Millisecond
		},
	})

	chatID := int64(100001)
	user := &tgbotapi.User{ID: 42, FirstName: "Денис"}
	first := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 1,
		Chat:      &tgbotapi.Chat{ID: chatID},
		From:      user,
		Text:      "первая часть",
		Date:      int(now.Unix()),
	}}
	second := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 2,
		Chat:      &tgbotapi.Chat{ID: chatID},
		From:      user,
		Text:      "вторая часть",
		Date:      int(now.Unix()),
	}}

	if !agent.TryRespond(first, chatID) {
		t.Fatal("first message should schedule a response")
	}
	time.Sleep(10 * time.Millisecond)
	if !agent.TryRespond(second, chatID) {
		t.Fatal("second message should update the pending response")
	}

	deadline := time.Now().Add(2 * time.Second)
	var savedText string
	for time.Now().Before(deadline) {
		err := db.QueryRow(
			"SELECT content FROM gemini_memories WHERE chat_id = ? AND role = ?",
			chatID,
			"Денис",
		).Scan(&savedText)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if savedText != "первая часть\nвторая часть" {
		t.Fatalf("saved prompt: got %q", savedText)
	}
	if requests.Load() != 1 {
		t.Fatalf("expected one Gemini request, got %d", requests.Load())
	}
}

func TestGeminiAgentTryRespondImmediate_CancelsPendingButKeepsCooldown(t *testing.T) {
	db := setupGeminiDB(t)
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ"}]}}]}`))
	}))
	defer server.Close()

	now := time.Now()
	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:         db,
		Client:     server.Client(),
		Now:        time.Now,
		APIKey:     "test-key",
		APIBaseURL: server.URL,
		ResponseDelay: func() time.Duration {
			return 50 * time.Millisecond
		},
	})

	chatID := int64(100002)
	user := &tgbotapi.User{ID: 7, FirstName: "Денис"}
	scheduled := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 1,
		Chat:      &tgbotapi.Chat{ID: chatID},
		From:      user,
		Text:      "обычное сообщение",
		Date:      int(now.Unix()),
	}}

	if !agent.TryRespond(scheduled, chatID) {
		t.Fatal("first message should schedule a response")
	}

	mention := tgbotapi.Message{
		MessageID: 2,
		Chat:      &tgbotapi.Chat{ID: chatID},
		From:      user,
		Text:      "@my_bot привет",
		Date:      int(now.Unix()),
	}
	if !agent.TryRespondImmediate(mention) {
		t.Fatal("mention should trigger an immediate response")
	}

	time.Sleep(300 * time.Millisecond)

	if got := requests.Load(); got != 1 {
		t.Fatalf("expected only the immediate Gemini request, got %d", got)
	}

	next := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 3,
		Chat:      &tgbotapi.Chat{ID: chatID},
		From:      user,
		Text:      "ещё одно обычное сообщение",
		Date:      int(time.Now().Unix()),
	}}
	if agent.TryRespond(next, chatID) {
		t.Fatal("cooldown should survive the canceled pending response")
	}
}

func TestGeminiAgentTryRespond_GuardChecks(t *testing.T) {
	db := setupGeminiDB(t)
	agent := app.NewGeminiAgent(db, &tgbotapi.BotAPI{
		Self: tgbotapi.User{UserName: "my_bot"},
	})
	chatID := int64(777)
	oldDate := int(time.Now().Add(-10 * time.Minute).Unix())

	cases := []struct {
		name       string
		update     tgbotapi.Update
		targetChat int64
		want       bool
	}{
		{"nil_message", tgbotapi.Update{}, chatID, false},
		{
			"wrong_chat",
			tgbotapi.Update{Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: chatID + 1}, From: &tgbotapi.User{}, Text: "hello", Date: oldDate}},
			chatID,
			false,
		},
		{
			"from_bot",
			tgbotapi.Update{Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: chatID}, From: &tgbotapi.User{IsBot: true}, Text: "hello", Date: oldDate}},
			chatID,
			false,
		},
		{
			"empty_text",
			tgbotapi.Update{Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: chatID}, From: &tgbotapi.User{IsBot: false}, Text: "   ", Date: oldDate}},
			chatID,
			false,
		},
		{
			"command",
			tgbotapi.Update{Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: chatID}, From: &tgbotapi.User{IsBot: false}, Text: "/help", Date: oldDate}},
			chatID,
			false,
		},
		{
			"mention_other_user",
			tgbotapi.Update{Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: chatID}, From: &tgbotapi.User{IsBot: false}, Text: "@someone hello", Date: oldDate}},
			chatID,
			false,
		},
		{
			"mention_own_bot",
			tgbotapi.Update{Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: chatID}, From: &tgbotapi.User{IsBot: false}, Text: "@my_bot hi", Date: oldDate}},
			chatID,
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := agent.TryRespond(c.update, c.targetChat)
			if got != c.want {
				t.Fatalf("case %s: got %v want %v", c.name, got, c.want)
			}
		})
	}
}

func TestGeminiAgentTryRespondImmediate_BasicChecks(t *testing.T) {
	db := setupGeminiDB(t)
	agent := app.NewGeminiAgent(db, nil)
	orig := os.Getenv("GEMINI_API_KEY")
	defer func() { _ = os.Setenv("GEMINI_API_KEY", orig) }()
	_ = os.Unsetenv("GEMINI_API_KEY")

	now := time.Now()

	cases := []struct {
		name string
		msg  tgbotapi.Message
		want bool
	}{
		{"from_bot", tgbotapi.Message{From: &tgbotapi.User{IsBot: true}, Text: "hi", Date: int(now.Unix())}, false},
		{"empty_text", tgbotapi.Message{From: &tgbotapi.User{IsBot: false}, Text: "   ", Date: int(now.Unix())}, false},
		{"command", tgbotapi.Message{From: &tgbotapi.User{IsBot: false}, Text: "/start", Date: int(now.Unix())}, false},
		{"old_message", tgbotapi.Message{From: &tgbotapi.User{IsBot: false}, Text: "hey", Date: int(now.Add(-10 * time.Minute).Unix())}, false},
		{"valid", tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 42}, From: &tgbotapi.User{IsBot: false}, Text: "hello", Date: int(now.Unix())}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := agent.TryRespondImmediate(c.msg)
			if got != c.want {
				t.Fatalf("case %s: got %v want %v", c.name, got, c.want)
			}
		})
	}
}

func TestSaveAndLoadGeminiMemoryContext(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(111)
	now := time.Now()

	if err := app.SaveGeminiMemory(db, chatID, 1, "user", "first", now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("save first memory: %v", err)
	}
	if err := app.SaveGeminiMemory(db, chatID, 0, "assistant", "second", now.Add(-time.Hour)); err != nil {
		t.Fatalf("save second memory: %v", err)
	}
	if err := app.SaveGeminiMemory(db, chatID+1, 1, "user", "other chat", now); err != nil {
		t.Fatalf("save other chat memory: %v", err)
	}

	got, err := app.LoadGeminiMemoryContext(db, chatID, 10, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("load memory context: %v", err)
	}

	want := "user: first\nassistant: second"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLoadGeminiMemoryContext_RespectsLimitAndSince(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(222)
	now := time.Now()

	if err := app.SaveGeminiMemory(db, chatID, 1, "user", "expired", now.Add(-48*time.Hour)); err != nil {
		t.Fatalf("save expired memory: %v", err)
	}
	if err := app.SaveGeminiMemory(db, chatID, 0, "assistant", "keep-1", now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("save keep-1 memory: %v", err)
	}
	if err := app.SaveGeminiMemory(db, chatID, 1, "user", "keep-2", now.Add(-time.Hour)); err != nil {
		t.Fatalf("save keep-2 memory: %v", err)
	}

	got, err := app.LoadGeminiMemoryContext(db, chatID, 2, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("load limited memory context: %v", err)
	}

	want := "assistant: keep-1\nuser: keep-2"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDeleteAllGeminiMemories(t *testing.T) {
	db := setupGeminiDB(t)

	if err := app.SaveGeminiMemory(db, 1, 1, "user", "hello", time.Now()); err != nil {
		t.Fatalf("save memory: %v", err)
	}
	if err := app.DeleteAllGeminiMemories(db); err != nil {
		t.Fatalf("delete memories: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM gemini_memories").Scan(&count); err != nil {
		t.Fatalf("count memories: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 memories after cleanup, got %d", count)
	}
}

func TestSaveAndLoadGeminiUserFacts(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(333)
	now := time.Now()

	if err := app.SaveGeminiUserFact(db, chatID, 101, "Denis", "сдал экзамен по Go", now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("save fact 1: %v", err)
	}
	if err := app.SaveGeminiUserFact(db, chatID, 102, "Дима", "фанат заднего привода", now.Add(-time.Hour)); err != nil {
		t.Fatalf("save fact 2: %v", err)
	}
	if err := app.SaveGeminiUserFact(db, chatID, 101, "Denis", "сдал экзамен по Go", now); err != nil {
		t.Fatalf("save duplicate fact: %v", err)
	}
	if err := app.SaveGeminiUserFact(db, chatID+1, 103, "Other", "чужой факт", now); err != nil {
		t.Fatalf("save other chat fact: %v", err)
	}

	facts, err := app.LoadRandomGeminiUserFacts(db, chatID, 10)
	if err != nil {
		t.Fatalf("load random facts: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("expected 2 unique facts, got %d", len(facts))
	}

	got := make(map[string]string, len(facts))
	for _, fact := range facts {
		got[fact.UserName] = fact.Fact
	}

	if got["Denis"] != "сдал экзамен по Go" {
		t.Fatalf("missing Denis fact: %+v", got)
	}
	if got["Дима"] != "фанат заднего привода" {
		t.Fatalf("missing Дима fact: %+v", got)
	}
}

func TestNextGeminiMemoryCleanupAt(t *testing.T) {
	loc := time.FixedZone("UTC+5", 5*60*60)

	morning := time.Date(2026, time.March, 9, 1, 0, 0, 0, loc)
	gotMorning := app.NextGeminiMemoryCleanupAt(morning)
	wantMorning := time.Date(2026, time.March, 9, 3, 0, 0, 0, loc)
	if !gotMorning.Equal(wantMorning) {
		t.Fatalf("morning cleanup time: got %v want %v", gotMorning, wantMorning)
	}

	afterNight := time.Date(2026, time.March, 9, 4, 0, 0, 0, loc)
	gotAfterNight := app.NextGeminiMemoryCleanupAt(afterNight)
	wantAfterNight := time.Date(2026, time.March, 10, 3, 0, 0, 0, loc)
	if !gotAfterNight.Equal(wantAfterNight) {
		t.Fatalf("after night cleanup time: got %v want %v", gotAfterNight, wantAfterNight)
	}
}

func TestGoogleSearchAdapter(t *testing.T) {
	adapter := app.GeminiGoogleSearchAdapter{}
	for _, prompt := range []string{
		"поищи кто сейчас президент?",
		"загугли курс доллара",
		"что пишут в интернете про релиз?",
		"найди в сети рецепт",
		"найди актуальный курс доллара в сети",
		"посмотри, пожалуйста, в сети курс доллара",
	} {
		if !adapter.ShouldSearch(prompt) {
			t.Fatalf("expected search for explicit search prompt %q", prompt)
		}
	}
	for _, prompt := range []string{
		"кто сейчас президент?",
		"как дела?",
		"объясни бинарный поиск",
		"почему интернет тормозит?",
		"я читал в интернете, что это миф",
		"почему в интернете столько рекламы?",
		"лол",
	} {
		if adapter.ShouldSearch(prompt) {
			t.Fatalf("did not expect search for non-search prompt %q", prompt)
		}
	}

	tools := adapter.BuildTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].GoogleSearch == nil {
		t.Fatalf("expected google_search tool to be set (Gemini 2.x format)")
	}
}

// waitForGeminiReply waits until the agent stores the assistant answer.
func waitForGeminiReply(t *testing.T, db *sql.DB, chatID int64) string {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var reply string
		err := db.QueryRow(
			"SELECT content FROM gemini_memories WHERE chat_id = ? AND role = ?",
			chatID,
			"bot",
		).Scan(&reply)
		if err == nil {
			return reply
		}
		time.Sleep(10 * time.Millisecond)
	}
	return ""
}

func geminiTestMessage(chatID int64) tgbotapi.Message {
	return tgbotapi.Message{
		MessageID: 1,
		Chat:      &tgbotapi.Chat{ID: chatID},
		From:      &tgbotapi.User{ID: 42, FirstName: "Денис"},
		Text:      "как дела?",
		Date:      int(time.Now().Unix()),
	}
}

func TestGeminiAgentRetriesOverloadedModel(t *testing.T) {
	db := setupGeminiDB(t)
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":503,"status":"UNAVAILABLE"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:          db,
		Client:      server.Client(),
		APIKey:      "test-key",
		APIBaseURL:  server.URL,
		MaxAttempts: 3,
		RetryDelay:  func(int) time.Duration { return 0 },
	})

	chatID := int64(100010)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	if reply := waitForGeminiReply(t, db, chatID); reply != "ответ" {
		t.Fatalf("expected the answer of the third attempt, got %q", reply)
	}
	if requests.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", requests.Load())
	}
}

func TestGeminiAgentFallsBackToAnotherModel(t *testing.T) {
	db := setupGeminiDB(t)
	var primaryRequests, fallbackRequests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "gemini-3.7-flash") {
			primaryRequests.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":503,"status":"UNAVAILABLE"}}`))
			return
		}
		fallbackRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"запасной ответ"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:             db,
		Client:         server.Client(),
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		Model:          "gemini-3.7-flash",
		FallbackModels: []string{"gemini-3.5-flash"},
		MaxAttempts:    2,
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	chatID := int64(100011)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	if reply := waitForGeminiReply(t, db, chatID); reply != "запасной ответ" {
		t.Fatalf("expected the fallback model answer, got %q", reply)
	}
	if primaryRequests.Load() != 2 {
		t.Fatalf("expected 2 attempts on the configured model, got %d", primaryRequests.Load())
	}
	if fallbackRequests.Load() != 1 {
		t.Fatalf("expected 1 request to the fallback model, got %d", fallbackRequests.Load())
	}
}

// TestGeminiAgentWalksTheWholeFallbackChain — фолбеков может быть несколько:
// когда и основная, и первая запасная модель лежат, очередь доходит до третьей
func TestGeminiAgentWalksTheWholeFallbackChain(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		mu     sync.Mutex
		models []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		mu.Lock()
		models = append(models, model)
		mu.Unlock()

		if model != "gemini-3.6-flash" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":503,"status":"UNAVAILABLE"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"третья модель"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:         db,
		Client:     server.Client(),
		APIKey:     "test-key",
		APIBaseURL: server.URL,
		Model:      "gemini-3.8-flash",
		// blanks, the primary model and a repeated fallback must not add extra hops
		FallbackModels: []string{" gemini-3.7-flash ", "", "gemini-3.8-flash", "gemini-3.7-flash", "gemini-3.6-flash"},
		MaxAttempts:    2,
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	chatID := int64(100013)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	if reply := waitForGeminiReply(t, db, chatID); reply != "третья модель" {
		t.Fatalf("expected the answer of the last fallback model, got %q", reply)
	}
	want := []string{
		"gemini-3.8-flash", "gemini-3.8-flash",
		"gemini-3.7-flash", "gemini-3.7-flash",
		"gemini-3.6-flash",
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(models, ",") != strings.Join(want, ",") {
		t.Fatalf("expected the models to be tried in order %v, got %v", want, models)
	}
}

func TestGeminiAgentDoesNotRetryClientErrors(t *testing.T) {
	db := setupGeminiDB(t)
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"status":"INVALID_ARGUMENT"}}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:             db,
		Client:         server.Client(),
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		Model:          "gemini-3.7-flash",
		FallbackModels: []string{"gemini-3.5-flash"},
		MaxAttempts:    3,
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	chatID := int64(100012)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	time.Sleep(200 * time.Millisecond)
	if requests.Load() != 1 {
		t.Fatalf("a 400 must not be retried, got %d requests", requests.Load())
	}
	if reply := waitForGeminiReply(t, db, chatID); reply != "" {
		t.Fatalf("no answer should be stored, got %q", reply)
	}
}

// TestGeminiAgentPutsReplyTargetIntoPrompt — память не говорит, на какое именно
// сообщение отвечает пользователь: нужная реплика может быть не последней или
// вовсе выпасть из лимита, поэтому цитата идёт в промпт отдельным блоком
func TestGeminiAgentPutsReplyTargetIntoPrompt(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		promptMu sync.Mutex
		prompt   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		promptMu.Lock()
		prompt = string(body)
		promptMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ага"}]}}]}`))
	}))
	defer server.Close()

	selfBot := mockSendBot(t, true)
	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:         db,
		Bot:        selfBot,
		Client:     server.Client(),
		APIKey:     "test-key",
		APIBaseURL: server.URL,
		Model:      "gemini-3.7-flash",
	})

	chatID := int64(100020)
	message := geminiTestMessage(chatID)
	message.Text = "Я да"
	message.ReplyToMessage = &tgbotapi.Message{
		MessageID: 7,
		Chat:      &tgbotapi.Chat{ID: chatID},
		From:      &tgbotapi.User{ID: selfBot.Self.ID, FirstName: "TestBot", IsBot: true},
		Text:      "Вот 3 случайных факта из 12",
	}

	if !agent.TryRespondImmediate(message) {
		t.Fatal("message should be processed")
	}
	if reply := waitForGeminiReply(t, db, chatID); reply != "ага" {
		t.Fatalf("expected an answer, got %q", reply)
	}

	promptMu.Lock()
	defer promptMu.Unlock()
	if !strings.Contains(prompt, "The latest message is a reply to:") {
		t.Fatalf("prompt has no reply block: %s", prompt)
	}
	if !strings.Contains(prompt, `bot: Вот 3 случайных факта из 12`) {
		t.Fatalf("prompt has no quoted bot message: %s", prompt)
	}
}

// TestGeminiAgentQuotesReplyToOtherUser — реплай на чужое сообщение тоже
// попадает в промпт, под именем автора
func TestGeminiAgentQuotesReplyToOtherUser(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		promptMu sync.Mutex
		prompt   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		promptMu.Lock()
		prompt = string(body)
		promptMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ага"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:         db,
		Client:     server.Client(),
		APIKey:     "test-key",
		APIBaseURL: server.URL,
		Model:      "gemini-3.7-flash",
	})

	chatID := int64(100021)
	message := geminiTestMessage(chatID)
	message.Text = "поддерживаю"
	message.ReplyToMessage = &tgbotapi.Message{
		MessageID: 7,
		Chat:      &tgbotapi.Chat{ID: chatID},
		From:      &tgbotapi.User{ID: 555, FirstName: "Enroscado"},
		Caption:   "погнали в бар",
	}

	if !agent.TryRespondImmediate(message) {
		t.Fatal("message should be processed")
	}
	if reply := waitForGeminiReply(t, db, chatID); reply != "ага" {
		t.Fatalf("expected an answer, got %q", reply)
	}

	promptMu.Lock()
	defer promptMu.Unlock()
	if !strings.Contains(prompt, `Enroscado: погнали в бар`) {
		t.Fatalf("prompt has no quoted user message: %s", prompt)
	}
}

// TestGeminiAgentDoesNotCallOtherBotsSelf — "bot" в промпте означает нас самих,
// поэтому чужой бот чата должен остаться под своим именем
func TestGeminiAgentDoesNotCallOtherBotsSelf(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		promptMu sync.Mutex
		prompt   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		promptMu.Lock()
		prompt = string(body)
		promptMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ага"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:         db,
		Bot:        mockSendBot(t, true),
		Client:     server.Client(),
		APIKey:     "test-key",
		APIBaseURL: server.URL,
		Model:      "gemini-3.7-flash",
	})

	chatID := int64(100022)
	message := geminiTestMessage(chatID)
	message.Text = "и тебе того же"
	message.ReplyToMessage = &tgbotapi.Message{
		MessageID: 7,
		Chat:      &tgbotapi.Chat{ID: chatID},
		From:      &tgbotapi.User{ID: 777, FirstName: "ЧужойБот", IsBot: true},
		Text:      "погода сегодня ясная",
	}

	if !agent.TryRespondImmediate(message) {
		t.Fatal("message should be processed")
	}
	if reply := waitForGeminiReply(t, db, chatID); reply != "ага" {
		t.Fatalf("expected an answer, got %q", reply)
	}

	promptMu.Lock()
	defer promptMu.Unlock()
	if !strings.Contains(prompt, `ЧужойБот: погода сегодня ясная`) {
		t.Fatalf("another bot must keep its name in the prompt: %s", prompt)
	}
}

// TestGeminiAgentDefaultModelPoolRotation verifies that starting from
// gemini-3.6-flash with no fallback models specified walks the pool down and
// rotates round to 4-hydrogen and 3.8: 3.6 -> 3.5-lite -> 4-hydrogen -> 3.8.
func TestGeminiAgentDefaultModelPoolRotation(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		mu     sync.Mutex
		models []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		mu.Lock()
		models = append(models, model)
		mu.Unlock()

		if model != "gemini-3.8-flash" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":503,"status":"UNAVAILABLE"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ от 3.8"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:             db,
		Client:         server.Client(),
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		Model:          "gemini-3.6-flash",
		FallbackModels: nil, // default rotation from pool
		MaxAttempts:    1,
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	chatID := int64(100030)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	if reply := waitForGeminiReply(t, db, chatID); reply != "ответ от 3.8" {
		t.Fatalf("expected the answer from gemini-3.8-flash, got %q", reply)
	}

	want := []string{
		"gemini-3.6-flash",
		"gemini-3.5-flash-lite",
		"gemini-4-hydrogen",
		"gemini-3.8-flash",
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(models, ",") != strings.Join(want, ",") {
		t.Fatalf("expected models in order %v, got %v", want, models)
	}
}

// TestGeminiAgentDefaultModelPoolRotation_FromFirstModel verifies rotation
// starting from the first model in pool (gemini-4-hydrogen).
func TestGeminiAgentDefaultModelPoolRotation_FromFirstModel(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		mu     sync.Mutex
		models []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		mu.Lock()
		models = append(models, model)
		mu.Unlock()

		if model != "gemini-3.5-flash-lite" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":503,"status":"UNAVAILABLE"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ от lite"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:             db,
		Client:         server.Client(),
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		Model:          "gemini-4-hydrogen",
		FallbackModels: nil,
		MaxAttempts:    1,
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	chatID := int64(100031)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	if reply := waitForGeminiReply(t, db, chatID); reply != "ответ от lite" {
		t.Fatalf("expected the answer from lite, got %q", reply)
	}

	want := []string{
		"gemini-4-hydrogen",
		"gemini-3.8-flash",
		"gemini-3.6-flash",
		"gemini-3.5-flash-lite",
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(models, ",") != strings.Join(want, ",") {
		t.Fatalf("expected models in order %v, got %v", want, models)
	}
}

// TestGeminiAgentDefaultModelPoolRotation_FromLastModel verifies rotation
// starting from the last model in pool (gemini-3.5-flash-lite).
func TestGeminiAgentDefaultModelPoolRotation_FromLastModel(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		mu     sync.Mutex
		models []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		mu.Lock()
		models = append(models, model)
		mu.Unlock()

		if model != "gemini-3.6-flash" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":503,"status":"UNAVAILABLE"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ от 3.6"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:             db,
		Client:         server.Client(),
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		Model:          "gemini-3.5-flash-lite",
		FallbackModels: nil,
		MaxAttempts:    1,
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	chatID := int64(100032)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	if reply := waitForGeminiReply(t, db, chatID); reply != "ответ от 3.6" {
		t.Fatalf("expected the answer from 3.6, got %q", reply)
	}

	want := []string{
		"gemini-3.5-flash-lite",
		"gemini-4-hydrogen",
		"gemini-3.8-flash",
		"gemini-3.6-flash",
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(models, ",") != strings.Join(want, ",") {
		t.Fatalf("expected models in order %v, got %v", want, models)
	}
}

// TestGeminiAgentModelOutsidePool_NoDefaultFallbacks verifies that a custom
// model outside the default pool does not pull in the default pool fallbacks.
func TestGeminiAgentModelOutsidePool_NoDefaultFallbacks(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		mu     sync.Mutex
		models []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		mu.Lock()
		models = append(models, model)
		mu.Unlock()

		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":503,"status":"UNAVAILABLE"}}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:             db,
		Client:         server.Client(),
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		Model:          "gemini-2.5-pro",
		FallbackModels: nil,
		MaxAttempts:    1,
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	chatID := int64(100033)
	msg := geminiTestMessage(chatID)
	msg.Text = "привет"
	if !agent.TryRespondImmediate(msg) {
		t.Fatal("message should be processed")
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(models) != 1 || models[0] != "gemini-2.5-pro" {
		t.Fatalf("expected only gemini-2.5-pro to be called, got %v", models)
	}
}

// TestGeminiAgentExplicitFallback_SameAsPrimary verifies that explicitly setting
// fallback to the primary model does not trigger the default pool.
func TestGeminiAgentExplicitFallback_SameAsPrimary(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		mu     sync.Mutex
		models []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		mu.Lock()
		models = append(models, model)
		mu.Unlock()

		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":503,"status":"UNAVAILABLE"}}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:             db,
		Client:         server.Client(),
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		Model:          "gemini-3.7-flash",
		FallbackModels: []string{"gemini-3.7-flash"},
		MaxAttempts:    1,
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	chatID := int64(100034)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(models) != 1 || models[0] != "gemini-3.7-flash" {
		t.Fatalf("expected only gemini-3.7-flash, got %v", models)
	}
}

// TestGeminiAgentModelUnavailable404_SkipsDirectlyToNextModel verifies that 404
// skips immediately to the next model without wasting retries on the missing model.
func TestGeminiAgentModelUnavailable404_SkipsDirectlyToNextModel(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		mu     sync.Mutex
		models []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		mu.Lock()
		models = append(models, model)
		mu.Unlock()

		if model == "gemini-3.7-flash" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"status":"NOT_FOUND","message":"models/gemini-3.7-flash is not found"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ от фолбека"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:             db,
		Client:         server.Client(),
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		Model:          "gemini-3.7-flash",
		FallbackModels: []string{"gemini-3.6-flash"},
		MaxAttempts:    3, // 404 must not use all 3 attempts!
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	chatID := int64(100035)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	if reply := waitForGeminiReply(t, db, chatID); reply != "ответ от фолбека" {
		t.Fatalf("expected fallback reply, got %q", reply)
	}

	mu.Lock()
	defer mu.Unlock()
	// gemini-3.7-flash should be called EXACTLY once (no retries on 404), then gemini-3.6-flash
	want := []string{"gemini-3.7-flash", "gemini-3.6-flash"}
	if strings.Join(models, ",") != strings.Join(want, ",") {
		t.Fatalf("expected %v, got %v", want, models)
	}
}

// TestGeminiAgentEmptyResponse_FallsBack verifies that an empty response (text is "")
// does not count as a successful answer and instead falls back to the next model.
func TestGeminiAgentEmptyResponse_FallsBack(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		mu     sync.Mutex
		models []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		mu.Lock()
		models = append(models, model)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if model == "gemini-3.7-flash" {
			// Returns empty parts text
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"   "}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"успешный ответ"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:             db,
		Client:         server.Client(),
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		Model:          "gemini-3.7-flash",
		FallbackModels: []string{"gemini-3.6-flash"},
		MaxAttempts:    2,
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	chatID := int64(100036)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	if reply := waitForGeminiReply(t, db, chatID); reply != "успешный ответ" {
		t.Fatalf("expected answer from fallback model, got %q", reply)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"gemini-3.7-flash", "gemini-3.6-flash"}
	if strings.Join(models, ",") != strings.Join(want, ",") {
		t.Fatalf("expected %v, got %v", want, models)
	}
}

// TestGeminiAgentApiKeyInHeaderNotURL verifies that API key is passed via
// x-goog-api-key header and not leaked in the request URL.
func TestGeminiAgentApiKeyInHeaderNotURL(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		headerKey string
		urlRaw    string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerKey = r.Header.Get("x-goog-api-key")
		urlRaw = r.URL.String()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"секрет сохранён"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:         db,
		Client:     server.Client(),
		APIKey:     "secret-gemini-key",
		APIBaseURL: server.URL,
		Model:      "gemini-3.7-flash",
	})

	chatID := int64(100037)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	if reply := waitForGeminiReply(t, db, chatID); reply != "секрет сохранён" {
		t.Fatalf("expected reply, got %q", reply)
	}

	if headerKey != "secret-gemini-key" {
		t.Fatalf("expected x-goog-api-key header 'secret-gemini-key', got %q", headerKey)
	}
	if strings.Contains(urlRaw, "secret-gemini-key") || strings.Contains(urlRaw, "key=") {
		t.Fatalf("URL must not contain API key: %s", urlRaw)
	}
}

// TestGeminiAgentGenericOutputCandidate verifies that candidates[0].output
// format is correctly extracted as a valid answer.
func TestGeminiAgentGenericOutputCandidate(t *testing.T) {
	db := setupGeminiDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"output":"valid candidate output"}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:         db,
		Client:     server.Client(),
		APIKey:     "test-key",
		APIBaseURL: server.URL,
		Model:      "gemini-3.7-flash",
	})

	chatID := int64(100038)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	if reply := waitForGeminiReply(t, db, chatID); reply != "valid candidate output" {
		t.Fatalf("expected valid candidate output, got %q", reply)
	}
}

// TestGeminiAgentEmptyCandidatesArray_FallsBack verifies that {"candidates":[]}
// is recognized as an empty response and triggers fallback to the next model.
func TestGeminiAgentEmptyCandidatesArray_FallsBack(t *testing.T) {
	db := setupGeminiDB(t)
	var (
		mu     sync.Mutex
		models []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		mu.Lock()
		models = append(models, model)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if model == "gemini-3.7-flash" {
			_, _ = w.Write([]byte(`{"candidates":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ после пустого массива"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:             db,
		Client:         server.Client(),
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		Model:          "gemini-3.7-flash",
		FallbackModels: []string{"gemini-3.6-flash"},
		MaxAttempts:    1,
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	chatID := int64(100039)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	if reply := waitForGeminiReply(t, db, chatID); reply != "ответ после пустого массива" {
		t.Fatalf("expected reply from fallback model, got %q", reply)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"gemini-3.7-flash", "gemini-3.6-flash"}
	if strings.Join(models, ",") != strings.Join(want, ",") {
		t.Fatalf("expected %v, got %v", want, models)
	}
}

// TestGeminiAgentDeadlineExceeded_ReportsToStatusBus verifies that when a context
// timeout occurs, the agent publishes TIMEOUT status on the status bus and halts.
func TestGeminiAgentDeadlineExceeded_ReportsToStatusBus(t *testing.T) {
	db := setupGeminiDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// simulate client timeout
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 10 * time.Millisecond}
	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:             db,
		Client:         client,
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		Model:          "gemini-3.7-flash",
		FallbackModels: []string{"gemini-3.6-flash"},
		MaxAttempts:    1,
		RetryDelay:     func(int) time.Duration { return 0 },
	})

	registry := agentbus.NewRegistry(time.Minute)
	agent.SetStatusBus(registry, "timeout_bot")
	_, _ = registry.Report("timeout_bot", agentbus.HealthOK, agentbus.QuotaOK, agentbus.ErrNone, "initial")

	chatID := int64(100040)
	if !agent.TryRespondImmediate(geminiTestMessage(chatID)) {
		t.Fatal("message should be processed")
	}

	time.Sleep(100 * time.Millisecond)

	status, ok := registry.Get("timeout_bot")
	if !ok {
		t.Fatal("status should be registered")
	}
	if status.ErrorClass != "TIMEOUT" {
		t.Fatalf("expected errorClass TIMEOUT, got %q", status.ErrorClass)
	}
	if status.Health != "degraded" {
		t.Fatalf("expected health degraded, got %q", status.Health)
	}
}

// TestGeminiAgentSearchSuccess_PublishesRecoveredStatus verifies that a successful
// search-enabled request (which returns before generateContentWithFallback) still
// clears a previously published failure on the status bus.
func TestGeminiAgentSearchSuccess_PublishesRecoveredStatus(t *testing.T) {
	db := setupGeminiDB(t)

	var sawTools atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "google_search") {
			sawTools.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ с поиском"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:          db,
		Client:      server.Client(),
		APIKey:      "test-key",
		APIBaseURL:  server.URL,
		Model:       "gemini-2.5-flash",
		MaxAttempts: 1,
		RetryDelay:  func(int) time.Duration { return 0 },
	})

	registry := agentbus.NewRegistry(time.Minute)
	agent.SetStatusBus(registry, "search_bot")
	_, _ = registry.Report("search_bot", agentbus.HealthDown, agentbus.QuotaExhausted, agentbus.ErrUpstreamDown, "")

	chatID := int64(100041)
	msg := geminiTestMessage(chatID)
	msg.Text = "поищи кто сейчас президент?"
	if !agent.TryRespondImmediate(msg) {
		t.Fatal("message should be processed")
	}
	if reply := waitForGeminiReply(t, db, chatID); reply != "ответ с поиском" {
		t.Fatalf("unexpected reply %q", reply)
	}
	if !sawTools.Load() {
		t.Fatal("expected a search-enabled request")
	}

	status, ok := registry.Get("search_bot")
	if !ok {
		t.Fatal("status should be registered")
	}
	if status.Health != agentbus.HealthOK || status.Quota != agentbus.QuotaOK || status.ErrorClass != agentbus.ErrNone {
		t.Fatalf("expected recovered status, got %+v", status)
	}
}

// TestGeminiAgentStatusHeartbeat_KeepsIdleStatusCurrent verifies that a bot
// with no LLM traffic stays fresh on the board and keeps its last known quota
// instead of decaying to stale/unknown after the TTL.
func TestGeminiAgentStatusHeartbeat_KeepsIdleStatusCurrent(t *testing.T) {
	db := setupGeminiDB(t)
	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{DB: db})

	registry := agentbus.NewRegistry(80 * time.Millisecond)
	agent.SetStatusBus(registry, "idle_bot")
	_, _ = registry.Report("idle_bot", agentbus.HealthDown, agentbus.QuotaExhausted, agentbus.ErrResourceExausted, "")

	stop := agent.StartStatusHeartbeat(10 * time.Millisecond)
	defer stop()

	time.Sleep(300 * time.Millisecond) // > 3x TTL with no LLM calls

	status, ok := registry.Get("idle_bot")
	if !ok {
		t.Fatal("status should be registered")
	}
	if status.Stale {
		t.Fatal("heartbeat should keep the status current")
	}
	if status.Quota != agentbus.QuotaExhausted || status.ErrorClass != agentbus.ErrResourceExausted {
		t.Fatalf("heartbeat must not change the reported state, got %+v", status)
	}
}

// TestGeminiAgentStatusHeartbeat_PublishesInitialAndStops verifies the first
// beat publishes "ok, quota unknown" and that stop() ends the refreshing.
func TestGeminiAgentStatusHeartbeat_PublishesInitialAndStops(t *testing.T) {
	db := setupGeminiDB(t)
	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{DB: db})

	registry := agentbus.NewRegistry(80 * time.Millisecond)
	agent.SetStatusBus(registry, "fresh_bot")

	stop := agent.StartStatusHeartbeat(10 * time.Millisecond)
	status, ok := registry.Get("fresh_bot")
	if !ok || status.Health != agentbus.HealthOK || status.Quota != agentbus.QuotaUnknown {
		t.Fatalf("expected initial ok/unknown status, got %+v (ok=%v)", status, ok)
	}

	stop()
	stop() // idempotent
	time.Sleep(300 * time.Millisecond)
	if status, _ := registry.Get("fresh_bot"); !status.Stale {
		t.Fatal("after stop() the status should go stale")
	}
}

// TestGeminiModelResolution_EdgeCases tests edge cases for model normalization,
// resolution, default fallbacks rotation, and fallback filtering.
func TestGeminiModelResolution_EdgeCases(t *testing.T) {
	// resolveGeminiModel defaults
	if got := resolveGeminiModel(""); got != "gemini-4-hydrogen" {
		t.Fatalf("empty model should default to gemini-4-hydrogen, got %q", got)
	}
	if got := resolveGeminiModel("   "); got != "gemini-4-hydrogen" {
		t.Fatalf("whitespace model should default to gemini-4-hydrogen, got %q", got)
	}
	if got := resolveGeminiModel("not-gemini-model"); got != "gemini-4-hydrogen" {
		t.Fatalf("invalid model should default to gemini-4-hydrogen, got %q", got)
	}
	if got := resolveGeminiModel("models/gemini-4-hydrogen-latest"); got != "gemini-4-hydrogen" {
		t.Fatalf("expected normalized gemini-4-hydrogen, got %q", got)
	}

	// defaultGeminiFallbacks rotation
	want36 := []string{"gemini-3.5-flash-lite", "gemini-4-hydrogen", "gemini-3.8-flash"}
	got36 := defaultGeminiFallbacks("gemini-3.6-flash")
	if strings.Join(got36, ",") != strings.Join(want36, ",") {
		t.Fatalf("default fallbacks for 3.6: got %v want %v", got36, want36)
	}

	// Deprecated/redirected models outside pool have no default fallbacks
	if got := defaultGeminiFallbacks("gemini-3.7-flash"); got != nil {
		t.Fatalf("removed model 3.7 should have nil fallbacks, got %v", got)
	}
	if got := defaultGeminiFallbacks("gemini-2.5-pro"); got != nil {
		t.Fatalf("model outside pool should have nil fallbacks, got %v", got)
	}

	// normalizeGeminiFallbackModels edge cases
	if got := normalizeGeminiFallbackModels("gemini-3.7-flash", []string{" "}); len(got) != 0 {
		t.Fatalf("whitespace fallback should be dropped, got %v", got)
	}
	if got := normalizeGeminiFallbackModels("gemini-3.7-flash", []string{"garbage"}); len(got) != 0 {
		t.Fatalf("garbage fallback should be dropped, got %v", got)
	}
	if got := normalizeGeminiFallbackModels("gemini-3.7-flash", []string{"gemini-3.7-flash"}); len(got) != 0 {
		t.Fatalf("fallback matching primary should be dropped, got %v", got)
	}
	mixed := normalizeGeminiFallbackModels("gemini-3.7-flash", []string{
		"models/gemini-3.6-flash-latest",
		"gemini-3.6-flash",
		"garbage",
		"gemini-3.5-flash",
	})
	wantMixed := []string{"gemini-3.6-flash", "gemini-3.5-flash"}
	if strings.Join(mixed, ",") != strings.Join(wantMixed, ",") {
		t.Fatalf("expected %v, got %v", wantMixed, mixed)
	}

	// resolveGeminiSearchModel edge cases
	if got := resolveGeminiSearchModel("", false); got != "gemini-2.5-flash-lite" {
		t.Fatalf("empty search model should default to gemini-2.5-flash-lite, got %q", got)
	}
	if got := resolveGeminiSearchModel("gemini-2.5-flash-lite", true); got != "" {
		t.Fatalf("disabled search should return empty string, got %q", got)
	}
	if got := resolveGeminiSearchModel("not-gemini", false); got != "" {
		t.Fatalf("invalid search model should disable search, got %q", got)
	}
	if got := resolveGeminiSearchModel("models/gemini-2.5-pro-latest", false); got != "gemini-2.5-pro" {
		t.Fatalf("expected normalized gemini-2.5-pro, got %q", got)
	}
}

// TestGeminiAgentNonSearchQuestionSkipsSearchModelAndTools verifies that a
// regular question without explicit search intent goes straight to the primary
// model (gemini-4-hydrogen) without google_search tools.
func TestGeminiAgentNonSearchQuestionSkipsSearchModelAndTools(t *testing.T) {
	db := setupGeminiDB(t)

	var requests atomic.Int32
	var sawTools atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "google_search") {
			sawTools.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ без поиска"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:          db,
		Client:      server.Client(),
		APIKey:      "test-key",
		APIBaseURL:  server.URL,
		Model:       "gemini-4-hydrogen",
		MaxAttempts: 1,
		RetryDelay:  func(int) time.Duration { return 0 },
	})

	chatID := int64(100042)
	msg := geminiTestMessage(chatID)
	msg.Text = "кто сейчас президент?"
	if !agent.TryRespondImmediate(msg) {
		t.Fatal("message should be processed")
	}
	if reply := waitForGeminiReply(t, db, chatID); reply != "ответ без поиска" {
		t.Fatalf("unexpected reply %q", reply)
	}
	if sawTools.Load() {
		t.Fatal("gemini-4-hydrogen must not send google_search tools")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected exactly 1 request without search retry, got %d", got)
	}
}

// TestGeminiAgentRoutesSearchToSearchModel verifies that an explicit search
// request uses the configured search model (defaulting to gemini-2.5-flash-lite)
// with google_search tools even when the primary model is gemini-4-hydrogen.
func TestGeminiAgentRoutesSearchToSearchModel(t *testing.T) {
	db := setupGeminiDB(t)

	var (
		mu       sync.Mutex
		models   []string
		sawTools bool
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		models = append(models, model)
		if strings.Contains(string(body), "google_search") {
			sawTools = true
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"нашёл в сети"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:          db,
		Client:      server.Client(),
		APIKey:      "test-key",
		APIBaseURL:  server.URL,
		Model:       "gemini-4-hydrogen",
		MaxAttempts: 1,
		RetryDelay:  func(int) time.Duration { return 0 },
	})

	chatID := int64(100043)
	msg := geminiTestMessage(chatID)
	msg.Text = "поищи курс доллара"
	if !agent.TryRespondImmediate(msg) {
		t.Fatal("message should be processed")
	}
	if reply := waitForGeminiReply(t, db, chatID); reply != "нашёл в сети" {
		t.Fatalf("unexpected reply %q", reply)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(models) != 1 || models[0] != "gemini-2.5-flash-lite" {
		t.Fatalf("expected single request to gemini-2.5-flash-lite, got %v", models)
	}
	if !sawTools {
		t.Fatal("expected google_search tool to be sent to search model")
	}
}

// TestGeminiAgentSearchFailureFallsBackToPrimaryWithoutTools verifies that when
// the search model fails, the agent falls back to the primary model without tools.
func TestGeminiAgentSearchFailureFallsBackToPrimaryWithoutTools(t *testing.T) {
	db := setupGeminiDB(t)

	var (
		mu             sync.Mutex
		models         []string
		primaryTools   bool
		fallbackSystem string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		models = append(models, model)
		if model == "gemini-4-hydrogen" {
			if strings.Contains(string(body), "google_search") {
				primaryTools = true
			}
			var decoded struct {
				SystemInstruction *struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"system_instruction"`
			}
			if err := json.Unmarshal(body, &decoded); err == nil && decoded.SystemInstruction != nil && len(decoded.SystemInstruction.Parts) > 0 {
				fallbackSystem = decoded.SystemInstruction.Parts[0].Text
			}
		}
		mu.Unlock()

		if model == "gemini-2.5-flash-lite" {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":429,"status":"RESOURCE_EXHAUSTED"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ без поиска от 4"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:          db,
		Client:      server.Client(),
		APIKey:      "test-key",
		APIBaseURL:  server.URL,
		Model:       "gemini-4-hydrogen",
		MaxAttempts: 1,
		RetryDelay:  func(int) time.Duration { return 0 },
	})

	chatID := int64(100044)
	msg := geminiTestMessage(chatID)
	msg.Text = "загугли последние новости"
	if !agent.TryRespondImmediate(msg) {
		t.Fatal("message should be processed")
	}
	if reply := waitForGeminiReply(t, db, chatID); reply != "ответ без поиска от 4" {
		t.Fatalf("unexpected reply %q", reply)
	}

	mu.Lock()
	defer mu.Unlock()
	if strings.Join(models, ",") != "gemini-2.5-flash-lite,gemini-4-hydrogen" {
		t.Fatalf("expected fallback from search model to primary, got %v", models)
	}
	if primaryTools {
		t.Fatal("fallback request to primary model must not include google_search tools")
	}
	if !strings.Contains(fallbackSystem, "поиск в интернете сейчас недоступен") {
		t.Fatalf("expected fallback system instruction to mention search unavailability, got %q", fallbackSystem)
	}
}

// TestGeminiAgentDisableSearchSkipsSearchModel verifies that DisableSearch=true
// routes even explicit search prompts directly to the primary model without tools.
func TestGeminiAgentDisableSearchSkipsSearchModel(t *testing.T) {
	db := setupGeminiDB(t)

	var (
		mu       sync.Mutex
		models   []string
		sawTools bool
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		models = append(models, model)
		if strings.Contains(string(body), "google_search") {
			sawTools = true
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"без поиска"}]}}]}`))
	}))
	defer server.Close()

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:            db,
		Client:        server.Client(),
		APIKey:        "test-key",
		APIBaseURL:    server.URL,
		Model:         "gemini-4-hydrogen",
		DisableSearch: true,
		MaxAttempts:   1,
		RetryDelay:    func(int) time.Duration { return 0 },
	})

	chatID := int64(100045)
	msg := geminiTestMessage(chatID)
	msg.Text = "поищи курс доллара"
	if !agent.TryRespondImmediate(msg) {
		t.Fatal("message should be processed")
	}
	if reply := waitForGeminiReply(t, db, chatID); reply != "без поиска" {
		t.Fatalf("unexpected reply %q", reply)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(models) != 1 || models[0] != "gemini-4-hydrogen" {
		t.Fatalf("expected direct request to gemini-4-hydrogen, got %v", models)
	}
	if sawTools {
		t.Fatal("disabled search must not send google_search tools")
	}
}

// TestGeminiAgentSearchClientTimeoutFallsBackWhenCallerCtxAlive verifies that
// an http.Client.Timeout on the search attempt falls back to the primary model
// when the caller's context is still active, while a canceled/expired caller
// context aborts immediately without falling back.
func TestGeminiAgentSearchClientTimeoutFallsBackWhenCallerCtxAlive(t *testing.T) {
	db := setupGeminiDB(t)

	var (
		mu     sync.Mutex
		models []string
	)
	client := &http.Client{
		Timeout: 15 * time.Millisecond,
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
			mu.Lock()
			models = append(models, model)
			mu.Unlock()

			if model == "gemini-2.5-flash-lite" {
				<-r.Context().Done()
				return nil, r.Context().Err()
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"ответ после таймаута поиска"}]}}]}`)),
			}, nil
		}),
	}

	agent := app.NewGeminiAgentWithConfig(app.GeminiAgentConfig{
		DB:          db,
		Client:      client,
		APIKey:      "test-key",
		Model:       "gemini-4-hydrogen",
		MaxAttempts: 1,
		RetryDelay:  func(int) time.Duration { return 0 },
	})

	reply, err := geminiAgentCallLLM(agent, context.Background(), "system", "user", "поищи новости", true)
	if err != nil {
		t.Fatalf("expected fallback reply after search timeout, got err: %v", err)
	}
	if reply != "ответ после таймаута поиска" {
		t.Fatalf("unexpected reply %q", reply)
	}
	mu.Lock()
	gotModels := append([]string(nil), models...)
	mu.Unlock()
	if strings.Join(gotModels, ",") != "gemini-2.5-flash-lite,gemini-4-hydrogen" {
		t.Fatalf("expected search timeout to fall back to primary, got %v", gotModels)
	}

	// Now verify an already-canceled caller context stops without fallback.
	mu.Lock()
	models = nil
	mu.Unlock()
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := geminiAgentCallLLM(agent, canceledCtx, "system", "user", "поищи новости", true); err == nil {
		t.Fatal("expected error for canceled caller context")
	}
	mu.Lock()
	canceledModels := append([]string(nil), models...)
	mu.Unlock()
	if len(canceledModels) != 1 || canceledModels[0] != "gemini-2.5-flash-lite" {
		t.Fatalf("expected canceled caller ctx to skip fallback, got %v", canceledModels)
	}
}

// TestNewGeminiAgentSearchModelEnv verifies GEMINI_SEARCH_MODEL env handling
// (unset -> default, empty -> disabled, invalid -> disabled, custom -> used)
// through the real app.NewGeminiAgent constructor.
func TestNewGeminiAgentSearchModelEnv(t *testing.T) {
	db := setupGeminiDB(t)

	for _, tc := range []struct {
		name      string
		setEnv    bool
		envValue  string
		wantModel string
	}{
		{name: "unset defaults to 2.5-flash-lite", setEnv: false, wantModel: "gemini-2.5-flash-lite"},
		{name: "empty disables search", setEnv: true, envValue: "   ", wantModel: "gemini-4-hydrogen"},
		{name: "invalid disables search", setEnv: true, envValue: "not-a-gemini", wantModel: "gemini-4-hydrogen"},
		{name: "custom search model", setEnv: true, envValue: "gemini-2.5-pro", wantModel: "gemini-2.5-pro"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnv {
				t.Setenv("GEMINI_SEARCH_MODEL", tc.envValue)
			} else {
				// t.Setenv records any pre-existing value for cleanup before Unsetenv.
				t.Setenv("GEMINI_SEARCH_MODEL", "")
				_ = os.Unsetenv("GEMINI_SEARCH_MODEL")
			}
			t.Setenv("GEMINI_MODEL", "gemini-4-hydrogen")
			t.Setenv("GEMINI_API_KEY", "test-key")

			var gotModel string
			origTransport := http.DefaultTransport
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotModel = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)),
				}, nil
			})
			t.Cleanup(func() { http.DefaultTransport = origTransport })

			agent := app.NewGeminiAgent(db, nil)
			if _, err := geminiAgentCallLLM(agent, context.Background(), "sys", "usr", "поищи", true); err != nil {
				t.Fatalf("callLLM error: %v", err)
			}
			if gotModel != tc.wantModel {
				t.Fatalf("expected model %q, got %q", tc.wantModel, gotModel)
			}
		})
	}
}
