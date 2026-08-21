package app

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("Error closing database: %v", closeErr)
			}
			log.Printf("Error creating table: %v", err)
			return nil, err
		}

		// Создание индекса для pen_length
		createIndexQuery := `CREATE INDEX IF NOT EXISTS idx_pen_length ON pens(pen_length);`
		_, err = db.Exec(createIndexQuery)
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("Error closing database: %v", closeErr)
			}
			log.Printf("Error creating index: %v", err)
			return nil, err
		}

		// Создание индекса для tg_pen_id
		createIndexQuery = `CREATE INDEX IF NOT EXISTS idx_tg_pen_id ON pens(tg_pen_id);`
		_, err = db.Exec(createIndexQuery)
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("Error closing database: %v", closeErr)
			}
			log.Printf("Error creating index: %v", err)
			return nil, err
		}

		// Запускаем миграции
		err = RunMigrations(db)
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("Error closing database: %v", closeErr)
			}
			log.Printf("Error running migrations: %v", err)
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
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Error closing database: %v", closeErr)
		}
		log.Printf("Error creating index: %v", err)
		return nil, err
	}

	// Создание индекса для tg_pen_id
	createIndexQuery = `CREATE INDEX IF NOT EXISTS idx_tg_pen_id ON pens(tg_pen_id);`
	_, err = db.Exec(createIndexQuery)
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Error closing database: %v", closeErr)
		}
		log.Printf("Error creating index: %v", err)
		return nil, err
	}

	log.Println("Index created successfully in existing database")

	// Запускаем миграции для существующей базы данных
	err = RunMigrations(db)
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Error closing database: %v", closeErr)
		}
		log.Printf("Error running migrations: %v", err)
		return nil, err
	}

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

// GetPenNames получает все значения pen_name из таблицы pens для активных пользователей
func GetPenNames(db *sql.DB, chatID int64) ([]Member, error) {
	rows, err := db.Query("SELECT tg_pen_id, pen_name FROM pens WHERE tg_chat_id = ? AND is_active = TRUE", chatID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("Error closing rows: %v", closeErr)
		}
	}()

	var members []Member
	for rows.Next() {
		var member Member
		err := rows.Scan(&member.ID, &member.Name)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	log.Printf("Active members list: %v", members)
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

// UpdateUserPen обновляет значения pen_length, pen_last_update_at и отмечает пользователя как активного
func UpdateUserPen(db *sql.DB, userID int64, chatID int64, newSize int) {
	_, err := db.Exec("UPDATE pens SET pen_length = ?, pen_last_update_at = ?, is_active = TRUE WHERE tg_pen_id = ? AND tg_chat_id = ?", newSize, time.Now(), userID, chatID)
	if err != nil {
		log.Printf("Error updating pen size, last update time and active status: %v", err)
	} else {
		log.Printf("Successfully updated pen size, last update time and active status for userID: %d, chatID: %d, newSize: %d", userID, chatID, newSize)
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
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return
	}

	_, err = tx.Exec("UPDATE pens SET pen_length = ?, handsome_count = handsome_count + 1 WHERE tg_pen_id = ? AND tg_chat_id = ?", newSize, userID, chatID)
	if err != nil {
		log.Printf("Error updating giga count: %v", err)
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Printf("Error on transaction rollback: %v", rbErr)
		}
	} else {
		log.Printf("Successfully updated giga count for userID: %d, chatID: %d, newSize: %d", userID, chatID, newSize)
	}

	// Обновляем last_update
	err = UpdateGigaLastUpdate(tx, chatID)
	if err != nil {
		return
	}

	// Подтверждаем транзакцию
	err = tx.Commit()
	if err != nil {
		log.Printf("Error committing transaction: %v", err)
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Printf("Error on transaction rollback: %v", rbErr)
		}
	}
}

func UpdateGigaLastUpdate(db SQLExecutor, chatID int64) error {
	var err error
	dbStatement := "UPDATE pens SET handsome_last_update_at = ? WHERE tg_chat_id = ?"
	_, err = db.Exec(dbStatement, time.Now(), chatID)

	if err != nil {
		log.Printf("Error updating handsome last_update_at: %v", err)
		return err
	} else {
		log.Printf("Successfully updated handsome last_update_at for chatID: %d,", chatID)
		return nil
	}
}

// UpdateUnhandsome обновляет значения unhandsome_count и unhandsome_last_update_at в базе данных
func UpdateUnhandsome(db *sql.DB, newSize int, userID int64, chatID int64) {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return
	}
	_, err = tx.Exec("UPDATE pens SET pen_length = ?, unhandsome_count = unhandsome_count + 1 WHERE tg_pen_id = ? AND tg_chat_id = ?", newSize, userID, chatID)
	if err != nil {
		log.Printf("Error updating unhandsome count and last_update_at: %v", err)
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Printf("Error on transaction rollback: %v", rbErr)
		}
	} else {
		log.Printf("Successfully updated unhandsome count and last_update_at for userID: %d, chatID: %d, newSize: %d", userID, chatID, newSize)
	}

	// Обновляем last_update
	err = UpdateUnhandsomeLastUpdate(tx, chatID)
	if err != nil {
		return
	}

	// Подтверждаем транзакцию
	err = tx.Commit()
	if err != nil {
		log.Printf("Error committing transaction: %v", err)
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Printf("Error on transaction rollback: %v", rbErr)
		}
	}
}

func UpdateUnhandsomeLastUpdate(db SQLExecutor, chatID int64) error {
	var err error
	dbStatement := "UPDATE pens SET unhandsome_last_update_at = ? WHERE tg_chat_id = ?"
	_, err = db.Exec(dbStatement, time.Now(), chatID)

	if err != nil {
		log.Printf("Error updating unhandsome last_update_at: %v", err)
		return err
	} else {
		log.Printf("Successfully updated unhandsome last_update_at for chatID: %d,", chatID)
		return nil
	}
}

// StartBackupRoutine запускает процесс резервного копирования
func StartBackupRoutine(db *sql.DB, mutex *sync.Mutex) {
	go func() {
		// Настройка таймера для выполнения раз в час
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

	// Удаление старых резервных копий, если общий размер превышает 10 МБ
	if err := removeOldBackups(backupDir); err != nil {
		return fmt.Errorf("failed to remove old backups: %v", err)
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
	defer func() {
		if closeErr := srcDB.Close(); closeErr != nil {
			log.Printf("Error closing source database: %v", closeErr)
		}
	}()

	// Выполнение резервного копирования с использованием команды VACUUM INTO
	_, err = srcDB.Exec(fmt.Sprintf("VACUUM INTO '%s';", backupFile))
	if err != nil {
		return fmt.Errorf("failed to backup database: %v", err)
	}

	// Вывод сообщения об успешном создании резервной копии
	log.Printf("Backup created successfully at %s", backupFile)
	return nil // Возвращаем nil, если все операции прошли успешно
}

// removeOldBackups удаляет старые резервные копии, если общий размер всех резервных копий превышает 10 МБ
func removeOldBackups(backupDir string) error {
	const maxSize = 10 * 1024 * 1024 // 10 МБ

	// Получение списка файлов в директории резервных копий
	files, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %v", err)
	}

	// Вычисление общего размера всех файлов
	var totalSize int64
	for _, file := range files {
		if info, err := file.Info(); err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
	}

	// Если общий размер меньше или равен maxSize, ничего не делаем
	if totalSize <= maxSize {
		return nil
	}

	// Сортировка файлов по времени модификации (от старых к новым)
	sort.Slice(files, func(i, j int) bool {
		infoI, _ := files[i].Info()
		infoJ, _ := files[j].Info()
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	// Удаление старых файлов до тех пор, пока общий размер не станет меньше maxSize
	for _, file := range files {
		info, err := file.Info()
		if err != nil || info.IsDir() {
			continue
		}
		filePath := filepath.Join(backupDir, file.Name())
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove file: %v", err)
		} else {
			log.Printf("Removed old backup file: %s", filePath)
		}
		totalSize -= info.Size()
		if totalSize <= maxSize {
			break
		}
	}

	return nil
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

// SQLExecutor is an interface that wraps the Exec, Query, and QueryRow methods of sql.DB
type SQLExecutor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
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

func SaveGeminiMemory(db *sql.DB, chatID int64, role, content string, createdAt time.Time) error {
	if db == nil {
		return errors.New("db is nil")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	_, err := db.Exec(
		"INSERT INTO gemini_memories (chat_id, role, content, created_at) VALUES (?, ?, ?, ?)",
		chatID, role, content, createdAt.UTC().Format(sqliteTimestampLayout),
	)
	return err
}

func normalizeMemoryRole(value, fallback string) string {
	replacer := strings.NewReplacer("\r", " ", "\n", " ", ":", " ")
	cleaned := strings.TrimSpace(replacer.Replace(value))
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	if cleaned == "" {
		return fallback
	}
	return cleaned
}

func normalizeGeminiFactText(value string) string {
	replacer := strings.NewReplacer("\r", " ", "\n", " ")
	cleaned := strings.TrimSpace(replacer.Replace(value))
	return strings.Join(strings.Fields(cleaned), " ")
}

type GeminiUserFact struct {
	UserName string
	Fact     string
}

const maxGeminiUserFactsPerOwner = 30

// SaveGeminiUserFact stores one fact about a chat member. userID is the
// immutable Telegram ID of that member, or 0 when the name Gemini used could
// not be resolved to exactly one member: such facts still feed the shared chat
// context, but they never belong to anybody personally.
func SaveGeminiUserFact(db *sql.DB, chatID, userID int64, userName, fact string, createdAt time.Time) error {
	if db == nil {
		return errors.New("db is nil")
	}
	userName = normalizeMemoryRole(userName, "")
	fact = normalizeGeminiFactText(fact)
	if userName == "" || fact == "" {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// the same fact may already be stored without an owner: keep the row and
	// attach the owner we know now
	_, err = tx.Exec(
		`INSERT INTO gemini_user_facts (chat_id, user_id, user_name, fact, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (chat_id, user_name, fact) DO UPDATE SET user_id = excluded.user_id
		WHERE gemini_user_facts.user_id = 0 AND excluded.user_id <> 0`,
		chatID, userID, userName, fact, createdAt.UTC().Format(sqliteTimestampLayout),
	)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := pruneGeminiUserFacts(tx, chatID, userID, userName, maxGeminiUserFactsPerOwner); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

func pruneGeminiUserFacts(tx *sql.Tx, chatID, userID int64, userName string, limit int) error {
	if limit <= 0 {
		return nil
	}

	ids, err := geminiUserFactIDsToPrune(tx, chatID, userID, userName, limit)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.Exec("DELETE FROM gemini_user_facts WHERE id = ?", id); err != nil {
			return err
		}
	}
	return nil
}

// geminiUserFactIDsToPrune returns the facts of one owner beyond the keep
// newest ones. Owned facts are grouped by Telegram ID, ownerless ones by name.
func geminiUserFactIDsToPrune(tx *sql.Tx, chatID, userID int64, userName string, keep int) ([]int64, error) {
	rows, err := tx.Query(
		`SELECT id, user_name
		FROM gemini_user_facts
		WHERE chat_id = ? AND user_id = ?
		ORDER BY created_at DESC, id DESC`,
		chatID, userID,
	)
	if err != nil {
		return nil, err
	}

	wantedName := NormalizePersonName(userName)
	ids := make([]int64, 0)
	matched := 0
	for rows.Next() {
		var (
			id      int64
			rowName string
		)
		if err := rows.Scan(&id, &rowName); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if userID == 0 && NormalizePersonName(rowName) != wantedName {
			continue
		}
		matched++
		if matched > keep {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return ids, nil
}

// DeleteGeminiUserFactsForUser deletes the facts owned by one Telegram user in
// one chat and returns the number of deleted rows.
func DeleteGeminiUserFactsForUser(db *sql.DB, chatID, userID int64) (int64, error) {
	if db == nil {
		return 0, errors.New("db is nil")
	}
	if userID == 0 {
		return 0, nil
	}

	result, err := db.Exec(
		"DELETE FROM gemini_user_facts WHERE chat_id = ? AND user_id = ?",
		chatID, userID,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ChatMember is one person seen in a chat, with the names they can be called by.
type ChatMember struct {
	ID        int64
	FirstName string
	LastName  string
	UserName  string
}

// NameKeys returns the normalized names this member answers to.
func (m ChatMember) NameKeys() []string {
	return PersonNameKeys(m.FirstName, m.LastName, m.UserName)
}

// RememberChatMember refreshes the chat roster from an incoming message, so
// facts told about somebody in the third person can later be matched to a
// telegram ID by name.
func RememberChatMember(db *sql.DB, chatID int64, user *tgbotapi.User) {
	if db == nil || user == nil || user.ID == 0 || user.IsBot {
		return
	}

	_, err := db.Exec(
		`INSERT INTO chat_members (chat_id, user_id, first_name, last_name, user_name, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (chat_id, user_id) DO UPDATE SET
			first_name = excluded.first_name,
			last_name = excluded.last_name,
			user_name = excluded.user_name,
			updated_at = excluded.updated_at`,
		chatID, user.ID, user.FirstName, user.LastName, user.UserName,
		time.Now().UTC().Format(sqliteTimestampLayout),
	)
	if err != nil {
		log.Printf("RememberChatMember: %v", err)
	}
}

// LoadChatMembers returns everybody known in one chat.
func LoadChatMembers(db *sql.DB, chatID int64) ([]ChatMember, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}

	rows, err := db.Query(
		"SELECT user_id, first_name, last_name, user_name FROM chat_members WHERE chat_id = ?",
		chatID,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("Error closing chat members rows: %v", closeErr)
		}
	}()

	members := make([]ChatMember, 0)
	for rows.Next() {
		var member ChatMember
		if err := rows.Scan(&member.ID, &member.FirstName, &member.LastName, &member.UserName); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

// ResolveGeminiFactUserID maps the name from a [SAVE: Name — fact] tag to an
// immutable Telegram ID. The author of the message is matched first, then the
// chat roster: that is how facts told about somebody else get an owner too.
// Names are compared by their normalized form, so "Дима", "Dimon" and
// "Dmitriy" all reach the same person. The result is 0 when the name does not
// resolve to exactly one member, so a name two people share never grants
// access to the facts of either.
func ResolveGeminiFactUserID(db *sql.DB, chatID int64, factName string, author *tgbotapi.User) int64 {
	factKey := NormalizePersonName(factName)
	if factKey == "" {
		return 0
	}

	if author != nil && author.ID != 0 && containsString(PersonNameKeys(author.FirstName, author.LastName, author.UserName), factKey) {
		return author.ID
	}

	if db == nil {
		return 0
	}
	members, err := LoadChatMembers(db, chatID)
	if err != nil {
		log.Printf("ResolveGeminiFactUserID: load chat members error: %v", err)
		return 0
	}

	return singleMemberByNameKey(members, factKey, 0)
}

// singleMemberByNameKey returns the only member known by the given name, or 0
// when nobody or more than one person answers to it. selfID is the member the
// name is being resolved for: a namesake makes the name ambiguous, the person
// themselves does not.
func singleMemberByNameKey(members []ChatMember, nameKey string, selfID int64) int64 {
	var resolved int64
	for _, member := range members {
		if member.ID == 0 || !containsString(member.NameKeys(), nameKey) {
			continue
		}
		if resolved != 0 && resolved != member.ID {
			return 0
		}
		resolved = member.ID
	}
	if selfID != 0 && resolved != 0 && resolved != selfID {
		return 0
	}
	return resolved
}

// ClaimOwnerlessGeminiUserFacts binds the facts saved before owners were
// recorded to the user they are about, and returns how many were claimed. A
// fact is claimed only when its name belongs to this user alone: while another
// member of the chat answers to the same name, it stays ownerless.
func ClaimOwnerlessGeminiUserFacts(db *sql.DB, chatID int64, user *tgbotapi.User) (int64, error) {
	if db == nil {
		return 0, errors.New("db is nil")
	}
	if user == nil || user.ID == 0 {
		return 0, nil
	}

	ownKeys := PersonNameKeys(user.FirstName, user.LastName, user.UserName)
	if len(ownKeys) == 0 {
		return 0, nil
	}

	members, err := LoadChatMembers(db, chatID)
	if err != nil {
		return 0, err
	}

	rows, err := db.Query(
		"SELECT DISTINCT user_name FROM gemini_user_facts WHERE chat_id = ? AND user_id = 0",
		chatID,
	)
	if err != nil {
		return 0, err
	}
	ownerless := make([]string, 0)
	for rows.Next() {
		var userName string
		if err := rows.Scan(&userName); err != nil {
			_ = rows.Close()
			return 0, err
		}
		ownerless = append(ownerless, userName)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	var claimed int64
	for _, userName := range ownerless {
		nameKey := NormalizePersonName(userName)
		if !containsString(ownKeys, nameKey) {
			continue
		}
		if singleMemberByNameKey(members, nameKey, user.ID) == 0 {
			continue
		}

		result, err := db.Exec(
			"UPDATE gemini_user_facts SET user_id = ? WHERE chat_id = ? AND user_id = 0 AND user_name = ?",
			user.ID, chatID, userName,
		)
		if err != nil {
			return claimed, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return claimed, err
		}
		claimed += affected
	}
	return claimed, nil
}

func LoadRandomGeminiUserFacts(db *sql.DB, chatID int64, limit int) ([]GeminiUserFact, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}
	if limit <= 0 {
		return nil, nil
	}

	rows, err := db.Query(
		`SELECT user_name, fact
		FROM gemini_user_facts
		WHERE chat_id = ?
		ORDER BY RANDOM()
		LIMIT ?`,
		chatID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("Error closing gemini user facts rows: %v", closeErr)
		}
	}()

	facts := make([]GeminiUserFact, 0, limit)
	for rows.Next() {
		var fact GeminiUserFact
		if err := rows.Scan(&fact.UserName, &fact.Fact); err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return facts, nil
}

// CountGeminiUserFactsForUser returns how many facts one Telegram user owns in
// one chat.
func CountGeminiUserFactsForUser(db *sql.DB, chatID, userID int64) (int, error) {
	if db == nil {
		return 0, errors.New("db is nil")
	}
	if userID == 0 {
		return 0, nil
	}

	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM gemini_user_facts WHERE chat_id = ? AND user_id = ?",
		chatID, userID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// LoadGeminiUserFactsForUser returns up to limit random facts owned by one
// Telegram user in one chat.
func LoadGeminiUserFactsForUser(db *sql.DB, chatID, userID int64, limit int) ([]GeminiUserFact, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}
	if limit <= 0 || userID == 0 {
		return nil, nil
	}

	rows, err := db.Query(
		`SELECT user_name, fact
		FROM gemini_user_facts
		WHERE chat_id = ? AND user_id = ?
		ORDER BY RANDOM()`,
		chatID, userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("Error closing gemini user facts rows: %v", closeErr)
		}
	}()

	facts := make([]GeminiUserFact, 0, limit)
	seenFacts := make(map[string]struct{}, limit)
	for rows.Next() {
		var fact GeminiUserFact
		if err := rows.Scan(&fact.UserName, &fact.Fact); err != nil {
			return nil, err
		}
		factKey := strings.ToLower(fact.Fact)
		if _, exists := seenFacts[factKey]; exists {
			continue
		}
		seenFacts[factKey] = struct{}{}
		facts = append(facts, fact)
		if len(facts) == limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return facts, nil
}

func LoadGeminiMemoryContext(db *sql.DB, chatID int64, limit int, since time.Time) (string, error) {
	if db == nil {
		return "", errors.New("db is nil")
	}
	if limit <= 0 {
		return "", nil
	}

	rows, err := db.Query(
		`SELECT role, content
		FROM gemini_memories
		WHERE chat_id = ? AND created_at >= ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?`,
		chatID, since.UTC().Format(sqliteTimestampLayout), limit,
	)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("Error closing gemini memory rows: %v", closeErr)
		}
	}()

	type memoryRow struct {
		role    string
		content string
	}

	memories := make([]memoryRow, 0, limit)
	for rows.Next() {
		var row memoryRow
		if err := rows.Scan(&row.role, &row.content); err != nil {
			return "", err
		}
		memories = append(memories, row)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(memories) == 0 {
		return "", nil
	}

	var b strings.Builder
	for i := len(memories) - 1; i >= 0; i-- {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(memories[i].role)
		b.WriteString(": ")
		b.WriteString(memories[i].content)
	}
	return b.String(), nil
}

func DeleteAllGeminiMemories(db *sql.DB) error {
	if db == nil {
		return errors.New("db is nil")
	}
	_, err := db.Exec("DELETE FROM gemini_memories")
	return err
}

func DeleteOldGeminiMemories(db *sql.DB, olderThan time.Time) error {
	if db == nil {
		return errors.New("db is nil")
	}
	_, err := db.Exec("DELETE FROM gemini_memories WHERE created_at < ?", olderThan.UTC().Format(sqliteTimestampLayout))
	return err
}

func NextGeminiMemoryCleanupAt(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func StartGeminiMemoryCleanupRoutine(db *sql.DB, nowFn func() time.Time) {
	if nowFn == nil {
		nowFn = time.Now
	}

	go func() {
		for {
			now := nowFn()
			next := NextGeminiMemoryCleanupAt(now)
			timer := time.NewTimer(time.Until(next))
			<-timer.C
			timer.Stop()

			cutoff := nowFn().Add(-geminiMemoryWindow)
			if err := DeleteOldGeminiMemories(db, cutoff); err != nil {
				log.Printf("Gemini memories cleanup failed: %v", err)
			} else {
				log.Printf("Gemini memories cleanup completed at %s", next.Format(time.RFC3339))
			}
		}
	}()
}
