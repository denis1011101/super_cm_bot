package app

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const sqliteTimestampLayout = "2006-01-02 15:04:05Z07:00"

// InitDB инициализирует базу данных
func InitDB() (*sql.DB, error) {
	dbDir := "./data"

	// Проверка, существует ли директория базы данных
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		// Директория не существует, создаём её
		err = os.MkdirAll(dbDir, os.ModePerm)
		if err != nil {
			log.Printf("Error creating directory: %v", err)
			return nil, err
		}
	}

	dbPath := "./data/pens.db"

	// Проверка, существует ли директория базы данных
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		// Директория не существует, создаём её
		err = os.MkdirAll(dbDir, os.ModePerm)
		if err != nil {
			log.Printf("Error creating directory: %v", err)
			return nil, err
		}
	}

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

		// Создание индекса для pen_length
		createIndexQuery := `CREATE INDEX IF NOT EXISTS idx_pen_length ON pens(pen_length);`
		_, err = db.Exec(createIndexQuery)
		if err != nil {
			db.Close() // Закрываем базу данных при ошибке
			log.Printf("Error creating index: %v", err)
			return nil, err
		}

        // Создание индекса для tg_pen_id
        createIndexQuery = `CREATE INDEX IF NOT EXISTS idx_tg_pen_id ON pens(tg_pen_id);`
        _, err = db.Exec(createIndexQuery)
        if err != nil {
            db.Close() // Закрываем базу данных при ошибке
            log.Printf("Error creating index: %v", err)
            return nil, err
        }

		log.Println("Database and table and index created successfully")
		return db, nil
	}

	// Файл существует, просто открываем базу данных
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("Error opening database: %v", err)
		return nil, err
	}

	// Создание индекса для pen_length, если он еще не существует
	createIndexQuery := `CREATE INDEX IF NOT EXISTS idx_pen_length ON pens(pen_length);`
	_, err = db.Exec(createIndexQuery)
	if err != nil {
		log.Printf("Error creating index: %v", err)
		return nil, err
	}

	// Создание индекса для tg_pen_id
	createIndexQuery = `CREATE INDEX IF NOT EXISTS idx_tg_pen_id ON pens(tg_pen_id);`
	_, err = db.Exec(createIndexQuery)
	if err != nil {
		db.Close() // Закрываем базу данных при ошибке
		log.Printf("Error creating index: %v", err)
		return nil, err
	}

	log.Println("Index created successfully in existing database")

	// Установка режима журнала WAL
	_, err = db.Exec("PRAGMA journal_mode = WAL;")
	if err != nil {
		log.Printf("Error setting journal_mode: %v", err)
		return nil, err
	}

	log.Println("Database opened successfully")
	return db, nil
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
func GetPenNames(db *sql.DB, chatID int64) ([]Member, error) {
	rows, err := db.Query("SELECT tg_pen_id, pen_name FROM pens WHERE tg_chat_id = ?", chatID)
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

// GetUserPen получает значения pen_length и pen_last_update_at из базы данных
func GetUserPen(db *sql.DB, userID int64, chatID int64) (Pen, error) {
	var currentSize int
	var lastUpdate sql.NullTime
	err := db.QueryRow("SELECT pen_length, pen_last_update_at FROM pens WHERE tg_pen_id = ? AND tg_chat_id = ?", userID, chatID).Scan(&currentSize, &lastUpdate)
	if err != nil {
		log.Printf("Error querying user pen: %v", err)
		return Pen{}, err
	} else {
		log.Printf("User pen retrieved successfully")
	}
	return Pen{currentSize, lastUpdate.Time}, err
}

// UpdateUserPen обновляет значения pen_length и pen_last_update_at в базе данных
func UpdateUserPen(db *sql.DB, userID int64, chatID int64, newSize int) {
	_, err := db.Exec("UPDATE pens SET pen_length = ?, pen_last_update_at = ? WHERE tg_pen_id = ? AND tg_chat_id = ?", newSize, time.Now(), userID, chatID)
	if err != nil {
		log.Printf("Error updating pen size and last update time: %v", err)
	} else {
		log.Printf("Successfully updated pen size and last update time for userID: %d, chatID: %d, newSize: %d", userID, chatID, newSize)
	}
}

// GetGigaLastUpdateTime получает время последнего обновления для команды /giga
func GetGigaLastUpdateTime(db *sql.DB, chatID int64) (time.Time, error) {
	var lastUpdateText sql.NullString
	err := db.QueryRow("SELECT MAX(handsome_last_update_at) FROM pens WHERE tg_chat_id = ?", chatID).Scan(&lastUpdateText)
	if err != nil {
		log.Printf("Error querying last update time: %v", err)
	} else {
		log.Printf("Last update time retrieved successfully")
	}
	if lastUpdateText.Valid {
		lastUpdate, err := time.Parse(sqliteTimestampLayout, lastUpdateText.String)
		if err != nil {
			log.Printf("Error parsing last update time: %v", err)
			return time.Time{}, err
		}
		log.Printf("Last update time parsed successfully")
		return lastUpdate, nil
	}
	log.Printf("Last update time is empty")
	return time.Time{}, nil
}

// GetUnhandsomeLastUpdateTime получает время последнего обновления для команды /unhandsome
func GetUnhandsomeLastUpdateTime(db *sql.DB, chatID int64) (time.Time, error) {
	var lastUpdateText sql.NullString
	err := db.QueryRow("SELECT MAX(unhandsome_last_update_at) FROM pens WHERE tg_chat_id = ?", chatID).Scan(&lastUpdateText)
	if err != nil {
		log.Printf("Error querying last update time: %v", err)
	} else {
		log.Printf("Last update time retrieved successfully")
	}
	if lastUpdateText.Valid {
		lastUpdate, err := time.Parse(sqliteTimestampLayout, lastUpdateText.String)
		if err != nil {
			log.Printf("Error parsing last update time: %v", err)
			return time.Time{}, err
		}
		log.Printf("Last update time parsed successfully")
		return lastUpdate, nil
	}
	log.Printf("Last update time is empty")
	return time.Time{}, nil
}

// UpdateGiga обновляет значения handsome_count и handsome_last_update_at в базе данных
func UpdateGiga(db *sql.DB, newSize int, userID int64, chatID int64) {
	_, err := db.Exec("UPDATE pens SET pen_length = ?, handsome_count = handsome_count + 1, handsome_last_update_at = ? WHERE tg_pen_id = ? AND tg_chat_id = ?", newSize, time.Now(), userID, chatID)
	if err != nil {
		log.Printf("Error updating giga count and last_update_at : %v", err)
	} else {
		log.Printf("Successfully updated giga count and last_update_at for userID: %d, chatID: %d, newSize: %d", userID, chatID, newSize)
	}
}

// UpdateUnhandsome обновляет значения unhandsome_count и unhandsome_last_update_at в базе данных
func UpdateUnhandsome(db *sql.DB, newSize int, userID int64, chatID int64) {
	_, err := db.Exec("UPDATE pens SET pen_length = ?, unhandsome_count = unhandsome_count + 1, unhandsome_last_update_at = ? WHERE tg_pen_id = ? AND tg_chat_id = ?", newSize, time.Now(), userID, chatID)
	if err != nil {
		log.Printf("Error updating unhandsome count and last_update_at: %v", err)
	} else {
		log.Printf("Successfully updated unhandsome count and last_update_at for userID: %d, chatID: %d, newSize: %d", userID, chatID, newSize)
	}
}

// backupDatabase создает резервную копию базы данных SQLite
func backupDatabase() error {
	// Определение пути к файлу резервной копии в корневом каталоге
	source := "./data/pens.db"
	backupDir := "backups"

	// Генерация уникального имени файла резервной копии на основе текущей даты и времени
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(backupDir, "database_backup_"+timestamp+".db")

	// Создание директории для резервной копии, если она не существует
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			log.Fatalf("Cannot create backup directory: %s, error: %v", backupDir, err)
			return err
		}
	}

	// Проверка существования файла базы данных
	if _, err := os.Stat(source); os.IsNotExist(err) {
		log.Fatalf("Database file does not exist: %s", source)
		return fmt.Errorf("database file does not exist: %s", source)
	}

	// Открытие исходной базы данных
	srcDB, err := sql.Open("sqlite3", source)
	if err != nil {
		return fmt.Errorf("failed to open source database: %v", err)
	}
	defer srcDB.Close()

    // Выполнение резервного копирования с использованием команды VACUUM INTO
    _, err = srcDB.Exec(fmt.Sprintf("VACUUM INTO '%s';", backupFile))
    if err != nil {
        return fmt.Errorf("failed to backup database: %v", err)
    }

    // Вывод сообщения об успешном создании резервной копии
    log.Printf("Backup created successfully at %s", backupFile)
    return nil // Возвращаем nil, если все операции прошли успешно
}

// StartBackupRoutine запускает процесс резервного копирования
func StartBackupRoutine(db *sql.DB, mutex *sync.Mutex) {
	go func() {
		// Настройка таймера для выполнения раз в день
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			// Блокируем базу данных
			mutex.Lock()

			// Выполнение резервного копирования
			if err := backupDatabase(); err != nil {
				log.Printf("Ошибка при резервном копировании базы данных: %v", err)
			} else {
				log.Println("Резервное копирование завершено успешно")
			}

			// Разблокируем базу данных
			mutex.Unlock()

			// Задержка выполнения на 1 час
			log.Println("Ожидание 1 час перед следующим резервным копированием...")
		}
	}()
}

// CheckPenLength проверяет значения pen_length и пишет в лог, если больше половины значений равны 5
func CheckPenLength(db *sql.DB) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			<-ticker.C
			var totalCount, count5 int
			err := db.QueryRow(`
                SELECT 
                    COUNT(*) AS total_count,
                    SUM(CASE WHEN pen_length = 5 THEN 1 ELSE 0 END) AS count5
                FROM pens
            `).Scan(&totalCount, &count5)
			if err != nil {
				log.Printf("Failed to query pen_length: %v", err)
				continue
			}

			if totalCount > 0 && count5 > totalCount/2 {
				log.Println("База обнулилась: больше половины значений pen_length равны 5")
			}
		}
	}()
}

// Check database integrity and log the result
func CheckIntegrity(db *sql.DB) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			<-ticker.C
			_, err := db.Exec("PRAGMA integrity_check;")
			if err != nil {
				log.Printf("Integrity check FAILED!!!!: %v", err)
			} else {
				log.Println("Integrity check passed")
			}
		}
	}()
}

// Проверка наличия пользователя в базе данных
func UserExists(db *sql.DB, userID int64, chatID int64) (bool, error) {
    var exists bool
    query := `SELECT EXISTS(SELECT 1 FROM pens WHERE tg_pen_id = ? AND tg_chat_id = ?)`
    err := db.QueryRow(query, userID, chatID).Scan(&exists)
    if err != nil {
        return false, err
    }
    return exists, nil
}