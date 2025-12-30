package tests

import (
    "os"
    "testing"
    "time"
    "strings"
    _ "unsafe"
    _ "github.com/denis1011101/super_cm_bot/app"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// link to unexported/exported symbols in app package
//go:linkname callLLM github.com/denis1011101/super_cm_bot/app.callLLM
func callLLM(systemPrompt, userPrompt string) (string, error)

//go:linkname parseTextFromGenericResponse github.com/denis1011101/super_cm_bot/app.parseTextFromGenericResponse
func parseTextFromGenericResponse(b []byte) string

//go:linkname getPersonas github.com/denis1011101/super_cm_bot/app.getPersonas
func getPersonas(userText string) (string, string)

//go:linkname TryGeminiRespond github.com/denis1011101/super_cm_bot/app.TryGeminiRespond
func TryGeminiRespond(update tgbotapi.Update, bot *tgbotapi.BotAPI, targetChatID int64) bool

// add linkname for immediate responder
//go:linkname TryGeminiRespondImmediate github.com/denis1011101/super_cm_bot/app.TryGeminiRespondImmediate
func TryGeminiRespondImmediate(m tgbotapi.Message, bot *tgbotapi.BotAPI) bool

//go:linkname respondWithGemini github.com/denis1011101/super_cm_bot/app.respondWithGemini
func respondWithGemini(m tgbotapi.Message, bot *tgbotapi.BotAPI) error

//go:linkname geminiLast github.com/denis1011101/super_cm_bot/app.geminiLast
var geminiLast map[int64]time.Time

func TestCallLLM_NoAPIKey(t *testing.T) {
    orig := os.Getenv("GEMINI_API_KEY")
    defer func() { _ = os.Setenv("GEMINI_API_KEY", orig) }()

    _ = os.Unsetenv("GEMINI_API_KEY")
    _, err := callLLM("sys", "hello")
    if err == nil {
        t.Fatalf("expected error when GEMINI_API_KEY is not set, got nil")
    }
    if err.Error() != "GEMINI_API_KEY is not set" {
        t.Fatalf("unexpected error: %v", err)
    }
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

func TestGetPersonas_NonEmptyAndSafety(t *testing.T) {
    sys, user := getPersonas("Привет")
    if sys == "" {
        t.Fatalf("system instruction should not be empty")
    }
    if user != "Привет" {
        t.Fatalf("user text should be preserved, got %q", user)
    }
    if !containsIgnoreCase(sys, "SAFETY") && !containsIgnoreCase(sys, "safety") {
        t.Fatalf("system instruction should include SAFETY rules, got: %q", sys)
    }
    if !containsIgnoreCase(sys, "Persona") {
        t.Fatalf("system instruction should include Persona description, got: %q", sys)
    }
}

func TestTryGeminiRespond_SetsRandomCooldown(t *testing.T) {
    // ensure map is clean
    geminiLast = make(map[int64]time.Time)

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

    ok := TryGeminiRespond(upd, nil, chatID)
    if !ok {
        t.Fatalf("TryGeminiRespond should have started processing")
    }

    next, ok := geminiLast[chatID]
    if !ok {
        t.Fatalf("geminiLast not set for chat")
    }

    dur := next.Sub(now)
    if dur < 30*time.Minute {
        t.Fatalf("cooldown too short: %v", dur)
    }
    // allow small margin for scheduling/timing
    if dur > 61*time.Minute {
        t.Fatalf("cooldown too long: %v", dur)
    }
}

func TestRespondWithGemini_IgnoresOldMessages(t *testing.T) {
    // message older than allowed threshold (current code ignores >5m)
    old := time.Now().Add(-10 * time.Minute)
    msg := tgbotapi.Message{
        Chat: &tgbotapi.Chat{ID: 123},
        From: &tgbotapi.User{IsBot: false},
        Text: "old message",
        Date: int(old.Unix()),
    }

    // bot can be nil because old message returns early
    if err := respondWithGemini(msg, nil); err != nil {
        t.Fatalf("respondWithGemini should return nil for old messages, got: %v", err)
    }
}

// helper: case-insensitive substring check
func containsIgnoreCase(s, sub string) bool {
    return len(s) >= len(sub) && (len(sub) == 0 || (stringIndexFold(s, sub) >= 0))
}

// small portable case-insensitive index (avoids importing strings twice in tests)
func stringIndexFold(s, sep string) int {
    // quick path
    lo := lower(s)
    lsep := lower(sep)
    return index(lo, lsep)
}

func lower(s string) string {
    b := []byte(s)
    for i := range b {
        c := b[i]
        if 'A' <= c && c <= 'Z' {
            b[i] = c + 32
        }
    }
    return string(b)
}

func index(s, sep string) int {
    n := len(sep)
    if n == 0 {
        return 0
    }
    for i := 0; i+n <= len(s); i++ {
        if s[i:i+n] == sep {
            return i
        }
    }
    return -1
}

// test for immediate responder
func TestTryGeminiRespondImmediate_BasicChecks(t *testing.T) {
    orig := os.Getenv("GEMINI_API_KEY")
    defer func() { _ = os.Setenv("GEMINI_API_KEY", orig) }()

    // ensure no API key so goroutine won't try to call external API
    _ = os.Unsetenv("GEMINI_API_KEY")

    now := time.Now()

    cases := []struct {
        name string
        msg  tgbotapi.Message
        want bool
    }{
        {
            "from_bot",
            tgbotapi.Message{From: &tgbotapi.User{IsBot: true}, Text: "hi", Date: int(now.Unix())},
            false,
        },
        {
            "empty_text",
            tgbotapi.Message{From: &tgbotapi.User{IsBot: false}, Text: "   ", Date: int(now.Unix())},
            false,
        },
        {
            "command",
            tgbotapi.Message{From: &tgbotapi.User{IsBot: false}, Text: "/start", Date: int(now.Unix())},
            false,
        },
        {
            "old_message",
            tgbotapi.Message{From: &tgbotapi.User{IsBot: false}, Text: "hey", Date: int(now.Add(-10 * time.Minute).Unix())},
            false,
        },
        {
            "valid",
            tgbotapi.Message{From: &tgbotapi.User{IsBot: false}, Text: "hello", Date: int(now.Unix())},
            true,
        },
    }

    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            got := TryGeminiRespondImmediate(c.msg, nil)
            if got != c.want {
                t.Fatalf("case %s: got %v want %v", c.name, got, c.want)
            }
        })
    }
}

func TestTryGeminiRespondImmediate_DoesNotAffectGeminiLast(t *testing.T) {
    // prepare geminiLast with a value
    geminiLast = make(map[int64]time.Time)
    chatID := int64(4242)
    prev := time.Now().Add(1 * time.Hour)
    geminiLast[chatID] = prev

    msg := tgbotapi.Message{
        Chat: &tgbotapi.Chat{ID: chatID},
        From: &tgbotapi.User{IsBot: false},
        Text: "hello",
        Date: int(time.Now().Unix()),
    }

    _ = TryGeminiRespondImmediate(msg, nil)

    if got, ok := geminiLast[chatID]; !ok {
        t.Fatalf("geminiLast entry removed unexpectedly")
    } else if !got.Equal(prev) {
        t.Fatalf("geminiLast changed by immediate responder: got %v want %v", got, prev)
    }
}

// Новогодний тест: в праздничный период ожидаем появление HOLIDAY-инструкций (в ~2/3 случаев),
// вариантов с BEGINNING и END, и указание эмодзи.
func TestGetPersonas_HolidayBehavior(t *testing.T) {
    now := time.Now()
    if !((now.Month() == time.December && now.Day() >= 24) || (now.Month() == time.January && now.Day() <= 2)) {
        t.Skip("not holiday period; skipping holiday-specific test")
    }

    runs := 120
    foundHoliday := 0
    foundBegin := 0
    foundEnd := 0
    emojis := []string{"🎄", "🎉", "🥂", "🎆", "✨"}

    for i := 0; i < runs; i++ {
        sys, _ := getPersonas("hello")
        if strings.Contains(sys, "HOLIDAY") {
            foundHoliday++
            if strings.Contains(sys, "BEGINNING") {
                foundBegin++
            }
            if strings.Contains(sys, "END") {
                foundEnd++
            }
            hasEmoji := false
            for _, e := range emojis {
                if strings.Contains(sys, e) {
                    hasEmoji = true
                    break
                }
            }
            if !hasEmoji {
                t.Fatalf("holiday instruction without emojis: %q", sys)
            }
        }
    }

    if foundHoliday == 0 {
        t.Fatalf("no HOLIDAY instruction observed in %d runs", runs)
    }
    if foundBegin == 0 || foundEnd == 0 {
        t.Fatalf("expected both BEGINNING and END variants, got begin=%d end=%d", foundBegin, foundEnd)
    }
}
