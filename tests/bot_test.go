package tests

import (
	"database/sql"
	"testing"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	"github.com/denis1011101/super_cm_bot/tests/testutils"
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

// TestArchiveInactiveUsers_NullLastUpdateNotArchived — пользователь с NULL pen_last_update_at
// НЕ должен архивироваться, потому что в SQLite NULL < date = NULL (не TRUE).
// Регрессионный тест на поведение ArchiveInactiveUsers: регистрация через /giga или /unh
// раньше оставляла pen_last_update_at = NULL, из-за чего такие юзеры никогда не архивировались
// и продолжали побеждать в giga/unh после долгого отсутствия. Фикс: registerBot теперь
// ставит pen_last_update_at = CURRENT_TIMESTAMP при создании записи.
func TestArchiveInactiveUsers_NullLastUpdateNotArchived(t *testing.T) {
	db := setupBotDB(t)
	insertPen(t, db, 3, 100, nil) // NULL pen_last_update_at

	if err := app.ArchiveInactiveUsers(db); err != nil {
		t.Fatalf("ArchiveInactiveUsers: %v", err)
	}
	if !isActiveFor(t, db, 3, 100) {
		t.Fatal("user with NULL pen_last_update_at should not be archived")
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
