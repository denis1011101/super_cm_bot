package app

import (
	"database/sql"
	"fmt"
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
				handsome_count, unhandsome_count, pen_last_update_at, is_active
			)
			VALUES (?, ?, ?, 5, 0, 0, CURRENT_TIMESTAMP, TRUE)
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
