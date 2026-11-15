package tests

import (
	"testing"

	"github.com/denis1011101/super_cm_bot/app"
)

func TestEnsureDevFakeUsers_CreatesIdempotentlyAndNeverDeletes(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(-100500)

	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := db.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}

	if err := app.EnsureDevFakeUsers(db, chatID, 3); err != nil {
		t.Fatalf("EnsureDevFakeUsers: %v", err)
	}
	if got := count("SELECT COUNT(*) FROM pens WHERE tg_chat_id = ? AND tg_pen_id < 0 AND is_active = TRUE", chatID); got != 3 {
		t.Fatalf("expected 3 active fake users, got %d", got)
	}

	// Повторный вызов не создаёт дубликатов.
	if err := app.EnsureDevFakeUsers(db, chatID, 3); err != nil {
		t.Fatalf("second EnsureDevFakeUsers: %v", err)
	}
	if got := count("SELECT COUNT(*) FROM pens"); got != 3 {
		t.Fatalf("expected 3 rows after repeat, got %d", got)
	}

	// Увеличение числа добавляет только недостающих.
	if err := app.EnsureDevFakeUsers(db, chatID, 5); err != nil {
		t.Fatalf("grow EnsureDevFakeUsers: %v", err)
	}
	if got := count("SELECT COUNT(*) FROM pens"); got != 5 {
		t.Fatalf("expected 5 rows after growth, got %d", got)
	}

	// Уменьшение и ноль ничего не удаляют.
	if err := app.EnsureDevFakeUsers(db, chatID, 1); err != nil {
		t.Fatalf("shrink EnsureDevFakeUsers: %v", err)
	}
	if err := app.EnsureDevFakeUsers(db, chatID, 0); err != nil {
		t.Fatalf("zero EnsureDevFakeUsers: %v", err)
	}
	if got := count("SELECT COUNT(*) FROM pens"); got != 5 {
		t.Fatalf("expected data kept, got %d rows", got)
	}

	// Архивный фейковый пользователь реактивируется.
	if _, err := db.Exec("UPDATE pens SET is_active = FALSE"); err != nil {
		t.Fatal(err)
	}
	if err := app.EnsureDevFakeUsers(db, chatID, 2); err != nil {
		t.Fatalf("reactivate EnsureDevFakeUsers: %v", err)
	}
	if got := count("SELECT COUNT(*) FROM pens WHERE is_active = TRUE"); got != 2 {
		t.Fatalf("expected 2 reactivated users, got %d", got)
	}

	if err := app.EnsureDevFakeUsers(db, chatID, -1); err == nil {
		t.Fatal("negative count should fail")
	}
}

func TestEnsureDevUser_RegistersLikeFirstPenAndKeepsExistingData(t *testing.T) {
	db := setupGeminiDB(t)
	chatID := int64(-100500)

	if err := app.EnsureDevUser(db, chatID, 42, "denis"); err != nil {
		t.Fatalf("EnsureDevUser: %v", err)
	}
	pen, err := app.GetUserPen(db, 42, chatID)
	if err != nil {
		t.Fatalf("GetUserPen: %v", err)
	}
	if pen.Size != 5 || !pen.LastUpdateTime.IsZero() {
		t.Fatalf("expected 5 cm and no last update, got %+v", pen)
	}

	// Повторный запуск не трогает прогресс и не плодит строки.
	if _, err := db.Exec("UPDATE pens SET pen_length = 17, is_active = FALSE WHERE tg_pen_id = 42"); err != nil {
		t.Fatal(err)
	}
	if err := app.EnsureDevUser(db, chatID, 42, "other"); err != nil {
		t.Fatalf("second EnsureDevUser: %v", err)
	}
	var n, size int
	var name string
	var active bool
	if err := db.QueryRow("SELECT COUNT(*), MAX(pen_length), MAX(pen_name), MAX(is_active) FROM pens").Scan(&n, &size, &name, &active); err != nil {
		t.Fatal(err)
	}
	if n != 1 || size != 17 || name != "denis" || !active {
		t.Fatalf("got rows=%d size=%d name=%q active=%v", n, size, name, active)
	}

	if err := app.EnsureDevUser(db, chatID, 0, "x"); err == nil {
		t.Fatal("non-positive id should fail")
	}
}
