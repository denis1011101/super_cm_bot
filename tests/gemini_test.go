package tests

import (
	"database/sql"
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

//go:linkname defaultGeminiFallbacks github.com/denis1011101/super_cm_bot/app.defaultGeminiFallbacks
func defaultGeminiFallbacks(primary string) []string

//go:linkname normalizeGeminiFallbackModels github.com/denis1011101/super_cm_bot/app.normalizeGeminiFallbackModels
func normalizeGeminiFallbackModels(primary string, models []string) []string

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
	if !adapter.ShouldSearch("кто сейчас президент?") {
		t.Fatalf("expected search for explicit search-like prompt")
	}
	if adapter.ShouldSearch("лол") {
		t.Fatalf("did not expect search for casual short prompt")
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

// TestGeminiAgentDefaultModelPoolRotation verifies that starting from default
// gemini-3.7-flash with no fallback models specified walks the pool down and
// rotates round to 3.8: 3.7 -> 3.6 -> 3.5 -> 3.5-lite -> 3.8.
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
		Model:          "gemini-3.7-flash",
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
		"gemini-3.7-flash",
		"gemini-3.6-flash",
		"gemini-3.5-flash",
		"gemini-3.5-flash-lite",
		"gemini-3.8-flash",
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(models, ",") != strings.Join(want, ",") {
		t.Fatalf("expected models in order %v, got %v", want, models)
	}
}

// TestGeminiAgentDefaultModelPoolRotation_FromFirstModel verifies rotation
// starting from the first model in pool (gemini-3.8-flash).
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
		Model:          "gemini-3.8-flash",
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
		"gemini-3.8-flash",
		"gemini-3.7-flash",
		"gemini-3.6-flash",
		"gemini-3.5-flash",
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

		if model != "gemini-3.5-flash" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":503,"status":"UNAVAILABLE"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ответ от 3.5"}]}}]}`))
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

	if reply := waitForGeminiReply(t, db, chatID); reply != "ответ от 3.5" {
		t.Fatalf("expected the answer from 3.5, got %q", reply)
	}

	want := []string{
		"gemini-3.5-flash-lite",
		"gemini-3.8-flash",
		"gemini-3.7-flash",
		"gemini-3.6-flash",
		"gemini-3.5-flash",
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

// TestGeminiModelResolution_EdgeCases tests edge cases for model normalization,
// resolution, default fallbacks rotation, and fallback filtering.
func TestGeminiModelResolution_EdgeCases(t *testing.T) {
	// resolveGeminiModel defaults
	if got := resolveGeminiModel(""); got != "gemini-3.7-flash" {
		t.Fatalf("empty model should default to gemini-3.7-flash, got %q", got)
	}
	if got := resolveGeminiModel("   "); got != "gemini-3.7-flash" {
		t.Fatalf("whitespace model should default to gemini-3.7-flash, got %q", got)
	}
	if got := resolveGeminiModel("not-gemini-model"); got != "gemini-3.7-flash" {
		t.Fatalf("invalid model should default to gemini-3.7-flash, got %q", got)
	}
	if got := resolveGeminiModel("models/gemini-3.8-flash-latest"); got != "gemini-3.8-flash" {
		t.Fatalf("expected normalized gemini-3.8-flash, got %q", got)
	}

	// defaultGeminiFallbacks rotation
	want37 := []string{"gemini-3.6-flash", "gemini-3.5-flash", "gemini-3.5-flash-lite", "gemini-3.8-flash"}
	got37 := defaultGeminiFallbacks("gemini-3.7-flash")
	if strings.Join(got37, ",") != strings.Join(want37, ",") {
		t.Fatalf("default fallbacks for 3.7: got %v want %v", got37, want37)
	}

	// Model outside pool has no default fallbacks
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
}
