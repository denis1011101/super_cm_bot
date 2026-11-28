package app

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Свои сообщения бот никогда не обрабатывает как входящие — Telegram их и не
// присылает. Но в контексте Gemini они нужны: иначе он не понимает разговор,
// половину которого сам же и наговорил (результаты /pen, /giga, топы).
var (
	outgoingMemoryMu sync.RWMutex
	outgoingMemoryDB *sql.DB
)

// EnableOutgoingMemory включает запись всех исходящих сообщений бота в память Gemini
func EnableOutgoingMemory(db *sql.DB) {
	outgoingMemoryMu.Lock()
	defer outgoingMemoryMu.Unlock()
	outgoingMemoryDB = db
}

func recordOutgoingMemory(chatID int64, messageID int, text string) {
	outgoingMemoryMu.RLock()
	db := outgoingMemoryDB
	outgoingMemoryMu.RUnlock()
	if db == nil {
		return
	}
	if err := saveGeminiBotMessage(db, chatID, messageID, text, time.Now()); err != nil {
		log.Printf("recordOutgoingMemory: save bot memory error: %v", err)
	}
}

// SendMessage отправляет сообщение в чат или как ответ на конкретное сообщение
func SendMessage(chatID int64, text string, bot *telegram.Bot, replyToMessageID int) {
	if messageID, err := sendMessage(chatID, text, bot, replyToMessageID); err == nil {
		recordOutgoingMemory(chatID, messageID, text)
	}
}

// SendMessageWithoutMemory отправляет сообщение, не сохраняя его в память Gemini.
// Нужно там, где текст пересказывает данные, которые пользователь вправе стереть:
// иначе /forgetme удалит факты, а их копия останется висеть в контексте.
func SendMessageWithoutMemory(chatID int64, text string, bot *telegram.Bot, replyToMessageID int) {
	if messageID, err := sendMessage(chatID, text, bot, replyToMessageID); err == nil && messageID > 0 {
		recordOutgoingMemory(chatID, messageID, "")
	}
}

// sendMessage returns the ID assigned by Telegram after a successful send.
func sendMessage(chatID int64, text string, bot *telegram.Bot, replyToMessageID int) (int, error) {
	params := &telegram.SendMessageParams{ChatID: chatID, Text: text}
	if replyToMessageID != 0 {
		params.ReplyParameters = &models.ReplyParameters{MessageID: replyToMessageID}
	}
	sent, err := bot.SendMessage(context.Background(), params)
	if err != nil {
		log.Println("Error sending message:", err)
		return 0, err
	}
	log.Printf("Message sent to chat ID %d: %s", chatID, text)
	return sent.ID, nil
}

// ArchiveInactiveUsers помечает пользователей как неактивных, если они не обновлялись 180 дней.
// Пока пользователь не крутил /pen (pen_last_update_at = NULL), срок считается от даты
// регистрации; запись без обеих дат архивируется как легаси.
func ArchiveInactiveUsers(db *sql.DB) error {
	// Вычисляем дату 180 дней назад
	cutoffDate := time.Now().AddDate(0, 0, -180)

	log.Printf("Starting archive process for users inactive since: %s", cutoffDate.Format("2006-01-02"))

	// SQL запрос для обновления неактивных пользователей
	query := `
        UPDATE pens
        SET is_active = FALSE
        WHERE (COALESCE(pen_last_update_at, registered_at) IS NULL
            OR COALESCE(pen_last_update_at, registered_at) < ?)
        AND is_active = TRUE
    `

	result, err := db.Exec(query, cutoffDate)
	if err != nil {
		log.Printf("Error archiving inactive users: %v", err)
		return err
	}

	// Получаем количество обновленных записей
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		return err
	}

	log.Printf("Successfully archived %d inactive users", rowsAffected)
	return nil
}

type penUserRow struct {
	userID  int64
	chatID  int64
	oldName sql.NullString
}

func loadPenUsersForRefresh(db *sql.DB) ([]penUserRow, error) {
	rows, err := db.Query("SELECT tg_pen_id, tg_chat_id, pen_name FROM pens WHERE tg_pen_id > 0")
	if err != nil {
		log.Printf("RefreshUsernames: failed to query pens: %v", err)
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("RefreshUsernames: failed to close rows: %v", err)
		}
	}()

	var users []penUserRow
	for rows.Next() {
		var u penUserRow
		if err := rows.Scan(&u.userID, &u.chatID, &u.oldName); err != nil {
			log.Printf("RefreshUsernames: row scan error: %v", err)
			continue
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		log.Printf("RefreshUsernames: rows iteration error: %v", err)
		return nil, err
	}

	return users, nil
}

// RefreshUsernames пробегает всех пользователей в таблице pens и обновляет pen_name,
// если Telegram API вернул отличный username. В случае ошибки оставляет прежний.
func RefreshUsernames(db *sql.DB, bot *telegram.Bot) error {
	// Загружаем список и закрываем rows до вызовов Telegram API и UPDATE,
	// чтобы не удерживать единственное соединение в пуле (MaxOpenConns=1).
	users, err := loadPenUsersForRefresh(db)
	if err != nil {
		return err
	}

	for _, u := range users {
		// Запрашиваем информацию о пользователе в чате
		member, err := bot.GetChatMember(context.Background(), &telegram.GetChatMemberParams{ChatID: u.chatID, UserID: u.userID})
		if err != nil {
			log.Printf("RefreshUsernames: GetChatMember failed for user %d in chat %d: %v", u.userID, u.chatID, err)
			// не обновляем, оставляем старое имя
			time.Sleep(100 * time.Millisecond) // небольшой респит, чтобы не спамить API
			continue
		}

		user := TelegramMemberUser(*member)
		if user == nil || user.Username == "" {
			// если у пользователя нет username — пропускаем
			time.Sleep(100 * time.Millisecond)
			continue
		}
		newName := user.Username

		if !u.oldName.Valid || newName != u.oldName.String {
			_, err := db.Exec("UPDATE pens SET pen_name = ? WHERE tg_pen_id = ? AND tg_chat_id = ?", newName, u.userID, u.chatID)
			if err != nil {
				log.Printf("RefreshUsernames: failed to update pen_name for user %d chat %d: %v", u.userID, u.chatID, err)
			} else {
				log.Printf("RefreshUsernames: updated pen_name for user %d in chat %d: %s -> %s", u.userID, u.chatID, u.oldName.String, newName)
			}
		}

		// Небольшая пауза между запросами к API
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// StartPenNameUpdateRoutine запускает горутину для еженедельного актуализирования pen_name
func StartPenNameUpdateRoutine(db *sql.DB, bot *telegram.Bot) {
    go func() {
        log.Printf("Pen name update routine started")

        // Первый запуск: дождаться ближайшего понедельника 06:00
        now := time.Now()

        // Вычисляем дни до следующего понедельника (Monday == 1)
        daysUntilMonday := (1 - int(now.Weekday()) + 7) % 7
        if daysUntilMonday == 0 && now.Hour() >= 6 {
            // Если сегодня понедельник и время уже прошло 6:00, ждём до следующего понедельника
            daysUntilMonday = 7
        }

        nextMonday := now.AddDate(0, 0, daysUntilMonday)
        nextMonday = time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 6, 0, 0, 0, nextMonday.Location())

        // Ждём до первого запуска
        timeUntilNext := time.Until(nextMonday)
        log.Printf("Pen name update routine will start at: %s (in %v)", nextMonday.Format("2006-01-02 15:04:05"), timeUntilNext)
        time.Sleep(timeUntilNext)

        // Запускаем обновление каждую неделю в понедельник 06:00
        for {
            log.Printf("Starting pen name update job...")

            if err := RefreshUsernames(db, bot); err != nil {
                log.Printf("Error updating pen names: %v", err)
            } else {
                log.Printf("Pen names updated successfully")
            }

            log.Printf("Next pen name update will run in 7 days")
            time.Sleep(7 * 24 * time.Hour)
        }
    }()
}

// StartArchiveRoutine запускает горутину для еженедельного архивирования
func StartArchiveRoutine(db *sql.DB) {
    go func() {
        log.Printf("Archive routine started")
        
        // Первый запуск: дождаться ближайшего вторника 6:00 утра
        now := time.Now()

        // Вычисляем дни до следующего вторника
        daysUntilTuesday := (2 - int(now.Weekday()) + 7) % 7
        if daysUntilTuesday == 0 && now.Hour() >= 6 {
            // Если сегодня вторник и время уже прошло 6:00, ждем до следующего вторника
            daysUntilTuesday = 7
        }

        nextTuesday := now.AddDate(0, 0, daysUntilTuesday)
        nextTuesday = time.Date(nextTuesday.Year(), nextTuesday.Month(), nextTuesday.Day(), 6, 0, 0, 0, nextTuesday.Location())

        // Ждем до следующего вторника
        timeUntilNext := time.Until(nextTuesday)
        log.Printf("Archive routine will start at: %s (in %v)", nextTuesday.Format("2006-01-02 15:04:05"), timeUntilNext)

        // Ждем до первого запуска
        time.Sleep(timeUntilNext)

        // Запускаем архивирование каждый вторник
        for {
            log.Printf("Starting archive job...")
            
            // Выполняем архивирование
            if err := ArchiveInactiveUsers(db); err != nil {
                log.Printf("Error in archive routine: %v", err)
            } else {
                log.Printf("Archive completed successfully")
            }

            log.Printf("Next archive job will run in 7 days")
            
            // Ждем ровно неделю до следующего запуска
            time.Sleep(7 * 24 * time.Hour)
        }
    }()
}
