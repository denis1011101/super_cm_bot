package app

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB инициализирует базу данных
func InitDB() (*sql.DB, error) {
    dbPath := "./data/pens.db"

    // Проверка, существует ли файл базы данных
    if _, err := os.Stat(dbPath); os.IsNotExist(err) {
        // Файл не существует, создаём базу данных и таблицу
        db, err := sql.Open("sqlite3", dbPath)
        if err != nil {
            log.Printf("Error opening database: %v", err)
            return nil, err
        }

		createTableQuery := `
		CREATE TABLE IF NOT EXISTS pens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pen_name TEXT,
			tg_pen_id INTEGER UNIQUE,
			tg_chat_id INTEGER,
			pen_length INTEGER,
			pen_last_update_at TIMESTAMP,
			handsome_count INTEGER,
			handsome_last_update_at TIMESTAMP,
			unhandsome_count INTEGER,
			unhandsome_last_update_at TIMESTAMP
		);`
        _, err = db.Exec(createTableQuery)
        if err != nil {
            db.Close() // Закрываем базу данных при ошибке
            log.Printf("Error creating table: %v", err)
            return nil, err
        }

        log.Println("Database and table created successfully")
        return db, nil
    }

	    // Файл существует, просто открываем базу данных
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			log.Printf("Error opening database: %v", err)
			return nil, err
		}

		log.Println("Database opened successfully")
		return db, nil
	}

// UpdatepenSize обновляет размер пениса в базе данных
func UpdatepenSize(db *sql.DB, tgChatID int64, newSize int) error {
	_, err := db.Exec(`UPDATE pens SET pen_length = ? WHERE tg_chat_id = ?`, newSize, tgChatID)
	if err != nil {
		log.Printf("Error updating pen size: %v", err)
		return err
	}
	log.Printf("Pen size updated successfully for tg_chat_id: %d, new size: %d", tgChatID, newSize)
	return nil
}

// GetUserIDByUsername получает ID пользователя по его username
func GetUserIDByUsername(db *sql.DB, username string) (int, error) {
    var userID int
    err := db.QueryRow("SELECT tg_pen_id FROM pens WHERE pen_name = ?", username).Scan(&userID)
    if err != nil {
        return 0, err
    }
    log.Printf("User ID retrieved successfully for username: %s, user ID: %d", username, userID)
    return userID, nil
}

// GetPenNames получает все значения pen_name из таблицы pens
func GetPenNames(db *sql.DB) ([]Member, error) {
    rows, err := db.Query("SELECT tg_pen_id, pen_name FROM pens")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var members []Member
    for rows.Next() {
        var member Member
        err := rows.Scan(&member.ID, &member.Name)
        if err != nil {
            return nil, err
        }
        members = append(members, member)
    }
    return members, nil
}
