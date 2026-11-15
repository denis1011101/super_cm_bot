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
