package tests

import (
	"database/sql"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	"github.com/denis1011101/super_cm_bot/tests/testutils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

func setupBotDB(t *testing.T) *sql.DB {
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

func insertPen(t *testing.T, db *sql.DB, userID, chatID int64, lastUpdateAt interface{}) {
	t.Helper()
	var err error
	if lastUpdateAt == nil {
		_, err = db.Exec(
			`INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length, handsome_count, unhandsome_count) VALUES (?, ?, ?, 5, 0, 0)`,
			"user", userID, chatID,
		)
	} else {
		_, err = db.Exec(
			`INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length, handsome_count, unhandsome_count, pen_last_update_at) VALUES (?, ?, ?, 5, 0, 0, ?)`,
			"user", userID, chatID, lastUpdateAt,
		)
	}
	if err != nil {
		t.Fatalf("insertPen: %v", err)
	}
}

func isActiveFor(t *testing.T, db *sql.DB, userID, chatID int64) bool {
	t.Helper()
	var active bool
	err := db.QueryRow("SELECT is_active FROM pens WHERE tg_pen_id = ? AND tg_chat_id = ?", userID, chatID).Scan(&active)
	if err != nil {
		t.Fatalf("isActiveFor(%d): %v", userID, err)
	}
	return active
}

// TestArchiveInactiveUsers_OldUserGetsArchived — пользователь с last_update > 180 дней назад
// должен стать неактивным.
func TestArchiveInactiveUsers_OldUserGetsArchived(t *testing.T) {
	db := setupBotDB(t)
	old := time.Now().AddDate(0, 0, -181).Format("2006-01-02 15:04:05Z07:00")
	insertPen(t, db, 1, 100, old)

	if err := app.ArchiveInactiveUsers(db); err != nil {
		t.Fatalf("ArchiveInactiveUsers: %v", err)
	}
	if isActiveFor(t, db, 1, 100) {
		t.Fatal("expected user to be archived (is_active=FALSE), but is_active=TRUE")
	}
}

// TestArchiveInactiveUsers_RecentUserStaysActive — пользователь, активный вчера, не должен архивироваться.
func TestArchiveInactiveUsers_RecentUserStaysActive(t *testing.T) {
	db := setupBotDB(t)
	recent := time.Now().AddDate(0, 0, -1).Format("2006-01-02 15:04:05Z07:00")
	insertPen(t, db, 2, 100, recent)

	if err := app.ArchiveInactiveUsers(db); err != nil {
		t.Fatalf("ArchiveInactiveUsers: %v", err)
	}
	if !isActiveFor(t, db, 2, 100) {
		t.Fatal("expected recent user to stay active, but is_active=FALSE")
	}
}

// TestArchiveInactiveUsers_NullLastUpdateArchived — пользователь с NULL pen_last_update_at
// ДОЛЖЕН архивироваться. NULL остаётся только у легаси-записей, созданных до того, как
// registerBot начал ставить pen_last_update_at = CURRENT_TIMESTAMP: спина у них не было
// никогда, но из-за SQLite-семантики NULL < date = NULL архиватор их раньше пропускал,
// и они бесконечно участвовали в giga/unh.
func TestArchiveInactiveUsers_NullLastUpdateArchived(t *testing.T) {
	db := setupBotDB(t)
	insertPen(t, db, 3, 100, nil) // NULL pen_last_update_at

	if err := app.ArchiveInactiveUsers(db); err != nil {
		t.Fatalf("ArchiveInactiveUsers: %v", err)
	}
	if isActiveFor(t, db, 3, 100) {
		t.Fatal("user with NULL pen_last_update_at should be archived")
	}
}

// TestArchiveInactiveUsers_AlreadyInactiveUnchanged — уже неактивный пользователь
// не должен изменить статус после повторного запуска.
func TestArchiveInactiveUsers_AlreadyInactiveUnchanged(t *testing.T) {
	db := setupBotDB(t)
	old := time.Now().AddDate(0, 0, -200).Format("2006-01-02 15:04:05Z07:00")
	insertPen(t, db, 4, 100, old)
	// вручную ставим FALSE
	if _, err := db.Exec("UPDATE pens SET is_active = FALSE WHERE tg_pen_id = 4 AND tg_chat_id = 100"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := app.ArchiveInactiveUsers(db); err != nil {
		t.Fatalf("ArchiveInactiveUsers: %v", err)
	}
	if isActiveFor(t, db, 4, 100) {
		t.Fatal("already-inactive user should remain inactive")
	}
}

// TestArchiveInactiveUsers_179DaysNotArchived — пользователь, активный 179 дней назад, не архивируется.
func TestArchiveInactiveUsers_179DaysNotArchived(t *testing.T) {
	db := setupBotDB(t)
	recent := time.Now().AddDate(0, 0, -179).Format("2006-01-02 15:04:05Z07:00")
	insertPen(t, db, 5, 100, recent)

	if err := app.ArchiveInactiveUsers(db); err != nil {
		t.Fatalf("ArchiveInactiveUsers: %v", err)
	}
	if !isActiveFor(t, db, 5, 100) {
		t.Fatal("user active 179 days ago should not be archived")
	}
}

// mockSendBot возвращает бота, чей sendMessage успешен, если sendOK
func mockSendBot(t *testing.T, sendOK bool) *tgbotapi.BotAPI {
	t.Helper()
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := `{"ok":true,"result":{"id":123456,"is_bot":true,"first_name":"TestBot","username":"test_bot"}}`
			status := http.StatusOK
			if strings.Contains(r.URL.Path, "/sendMessage") {
				body = `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":-987654321,"type":"group"},"text":"ok"}}`
				if !sendOK {
					status = http.StatusBadRequest
					body = `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`
				}
			}
			return &http.Response{
				StatusCode: status,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    r,
			}, nil
		}),
	}
	bot, err := tgbotapi.NewBotAPIWithClient("fake-token", "https://telegram.test/bot%s/%s", client)
	if err != nil {
		t.Fatalf("Failed to create mock bot: %v", err)
	}
	return bot
}

// TestSendMessageRecordsOutgoingMemory — ответ команды уходит в память Gemini,
// иначе бот не понимает реплаи на собственные сообщения.
func TestSendMessageRecordsOutgoingMemory(t *testing.T) {
	db := setupBotDB(t)
	chatID := int64(-987654321)

	app.EnableOutgoingMemory(db)
	t.Cleanup(func() { app.EnableOutgoingMemory(nil) })

	app.SendMessage(chatID, "Вот 3 случайных факта из 12", mockSendBot(t, true), 0)

	context, err := app.LoadGeminiMemoryContext(db, chatID, 10, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("LoadGeminiMemoryContext: %v", err)
	}
	if context != "bot: Вот 3 случайных факта из 12" {
		t.Fatalf("Expected outgoing message in memory, got %q", context)
	}
}

// TestSendMessageWithoutOutgoingMemory — без включённой записи ничего не пишем
func TestSendMessageWithoutOutgoingMemory(t *testing.T) {
	db := setupBotDB(t)
	chatID := int64(-987654321)

	app.SendMessage(chatID, "тишина", mockSendBot(t, true), 0)

	context, err := app.LoadGeminiMemoryContext(db, chatID, 10, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("LoadGeminiMemoryContext: %v", err)
	}
	if context != "" {
		t.Fatalf("Expected empty memory, got %q", context)
	}
}

// TestRecordCommandMemory — команда пишется от имени автора, чтобы ответ бота
// в контексте не висел без адресата
func TestRecordCommandMemory(t *testing.T) {
	db := setupBotDB(t)
	chatID := int64(-987654321)

	app.RecordCommandMemory(db, chatID, &tgbotapi.User{ID: 42, FirstName: "Enroscado"}, "/myfacts")

	context, err := app.LoadGeminiMemoryContext(db, chatID, 10, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("LoadGeminiMemoryContext: %v", err)
	}
	if context != "Enroscado: /myfacts" {
		t.Fatalf("Expected command in memory, got %q", context)
	}
}

// TestSendMessageDoesNotRecordFailedSend — недоставленное сообщение не должно
// оседать в памяти: бот бы помнил то, чего чат не видел
func TestSendMessageDoesNotRecordFailedSend(t *testing.T) {
	db := setupBotDB(t)
	chatID := int64(-987654321)

	app.EnableOutgoingMemory(db)
	t.Cleanup(func() { app.EnableOutgoingMemory(nil) })

	app.SendMessage(chatID, "это не дойдёт", mockSendBot(t, false), 0)

	context, err := app.LoadGeminiMemoryContext(db, chatID, 10, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("LoadGeminiMemoryContext: %v", err)
	}
	if context != "" {
		t.Fatalf("Expected empty memory after a failed send, got %q", context)
	}
}

// TestSendMessageWithoutMemoryDoesNotRecord — вывод /myfacts идёт мимо памяти,
// иначе /forgetme стирает факты, а их копия остаётся в контексте
func TestSendMessageWithoutMemoryDoesNotRecord(t *testing.T) {
	db := setupBotDB(t)
	chatID := int64(-987654321)

	app.EnableOutgoingMemory(db)
	t.Cleanup(func() { app.EnableOutgoingMemory(nil) })

	app.SendMessageWithoutMemory(chatID, "Вот 3 случайных факта из 12", mockSendBot(t, true), 0)

	context, err := app.LoadGeminiMemoryContext(db, chatID, 10, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("LoadGeminiMemoryContext: %v", err)
	}
	if context != "" {
		t.Fatalf("Expected empty memory, got %q", context)
	}
}
