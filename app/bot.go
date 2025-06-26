package app

import (
	"database/sql"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SendMessage отправляет сообщение в чат или как ответ на конкретное сообщение
func SendMessage(chatID int64, text string, bot *tgbotapi.BotAPI, replyToMessageID int) {
	msg := tgbotapi.NewMessage(chatID, text)
	if replyToMessageID != 0 {
		msg.ReplyToMessageID = replyToMessageID
	}
    if _, err := bot.Send(msg); err != nil {
        log.Println("Error sending message:", err)
    } else {
        log.Printf("Message sent to chat ID %d: %s", chatID, text)
    }
}

// ArchiveInactiveUsers помечает пользователей как неактивных, если они не обновлялись 180 дней
func ArchiveInactiveUsers(db *sql.DB) error {
    // Вычисляем дату 180 дней назад
    cutoffDate := time.Now().AddDate(0, 0, -180)
    
    log.Printf("Starting archive process for users inactive since: %s", cutoffDate.Format("2006-01-02"))
    
    // SQL запрос для обновления неактивных пользователей
    query := `
        UPDATE pens 
        SET is_active = FALSE 
        WHERE pen_last_update_at < ? 
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

// StartArchiveRoutine запускает горутину для еженедельного архивирования
func StartArchiveRoutine(db *sql.DB) {
    go func() {
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
            // Выполняем архивирование
            if err := ArchiveInactiveUsers(db); err != nil {
                log.Printf("Error in archive routine: %v", err)
            } else {
                log.Printf("Archive completed successfully")
            }
            
            // Ждем ровно неделю до следующего запуска
            time.Sleep(7 * 24 * time.Hour)
        }
    }()
}
