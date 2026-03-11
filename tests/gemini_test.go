package tests

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"
	_ "unsafe"

	"github.com/denis1011101/super_cm_bot/app"
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

	if err := app.SaveGeminiMemory(db, chatID, "user", "first", now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("save first memory: %v", err)
	}
	if err := app.SaveGeminiMemory(db, chatID, "assistant", "second", now.Add(-time.Hour)); err != nil {
		t.Fatalf("save second memory: %v", err)
	}
	if err := app.SaveGeminiMemory(db, chatID+1, "user", "other chat", now); err != nil {
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

	if err := app.SaveGeminiMemory(db, chatID, "user", "expired", now.Add(-48*time.Hour)); err != nil {
		t.Fatalf("save expired memory: %v", err)
	}
	if err := app.SaveGeminiMemory(db, chatID, "assistant", "keep-1", now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("save keep-1 memory: %v", err)
	}
	if err := app.SaveGeminiMemory(db, chatID, "user", "keep-2", now.Add(-time.Hour)); err != nil {
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

	if err := app.SaveGeminiMemory(db, 1, "user", "hello", time.Now()); err != nil {
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

	if err := app.SaveGeminiUserFact(db, chatID, "Denis", "сдал экзамен по Go", now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("save fact 1: %v", err)
	}
	if err := app.SaveGeminiUserFact(db, chatID, "Дима", "фанат заднего привода", now.Add(-time.Hour)); err != nil {
		t.Fatalf("save fact 2: %v", err)
	}
	if err := app.SaveGeminiUserFact(db, chatID, "Denis", "сдал экзамен по Go", now); err != nil {
		t.Fatalf("save duplicate fact: %v", err)
	}
	if err := app.SaveGeminiUserFact(db, chatID+1, "Other", "чужой факт", now); err != nil {
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
