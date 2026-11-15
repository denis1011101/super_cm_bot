package app

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const devFakeUserBaseID int64 = -990000000

// EnsureDevFakeUsers creates missing deterministic test users for local
// development. Existing users are reactivated, and repeated calls do not
// create duplicates. Lowering the configured count never deletes data.
func EnsureDevFakeUsers(db *sql.DB, chatID int64, count int) error {
	if count < 0 {
		return fmt.Errorf("fake user count must not be negative: %d", count)
	}
	if count == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin fake user seed: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i := 1; i <= count; i++ {
		userID := devFakeUserBaseID - int64(i)
		userName := fmt.Sprintf("test_user_%d", i)
		_, err = tx.Exec(`
			INSERT INTO pens (
				pen_name, tg_pen_id, tg_chat_id, pen_length,
				handsome_count, unhandsome_count, pen_last_update_at, registered_at, is_active
			)
			VALUES (?, ?, ?, 5, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, TRUE)
			ON CONFLICT(tg_pen_id) DO UPDATE SET
				pen_name = excluded.pen_name,
				tg_chat_id = excluded.tg_chat_id,
				is_active = TRUE
		`, userName, userID, chatID)
		if err != nil {
			return fmt.Errorf("seed fake user %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit fake user seed: %w", err)
	}
	return nil
}

// EnsureDevUser registers the developer's real Telegram account in the chat,
// the same way the first /pen would: 5 cm and no previous spin, so the first
// command is allowed immediately. An existing row keeps its data and is only
// reactivated.
func EnsureDevUser(db *sql.DB, chatID, userID int64, userName string) error {
	if userID <= 0 {
		return fmt.Errorf("dev user id must be positive: %d", userID)
	}
	_, err := db.Exec(`
		INSERT INTO pens (
			pen_name, tg_pen_id, tg_chat_id, pen_length,
			handsome_count, unhandsome_count, pen_last_update_at, registered_at, is_active
		)
		VALUES (?, ?, ?, 5, 0, 0, NULL, ?, TRUE)
		ON CONFLICT(tg_pen_id) DO UPDATE SET is_active = TRUE
	`, userName, userID, chatID, time.Now())
	if err != nil {
		return fmt.Errorf("ensure dev user %d: %w", userID, err)
	}
	return nil
}

// ResetDatabaseFiles deletes the SQLite database and its WAL/SHM side files
// from dbDir so the next InitDB starts from an empty schema. Missing files are
// not an error. It must run before the database is opened.
func ResetDatabaseFiles(dbDir string) error {
	for _, name := range []string{"pens.db", "pens.db-wal", "pens.db-shm"} {
		if err := os.Remove(filepath.Join(dbDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	return nil
}
