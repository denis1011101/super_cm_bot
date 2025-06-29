package app

import (
	"database/sql"
	"log"
	"math/rand"
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

// StartDailyCommandsRoutine запускает горутину для ежедневного вызова команд в случайное время
func StartDailyCommandsRoutine(
    db *sql.DB,
    bot *tgbotapi.BotAPI,
    chatID int64,
    gigaHandler func(tgbotapi.Update, *tgbotapi.BotAPI, *sql.DB),
    unhHandler func(tgbotapi.Update, *tgbotapi.BotAPI, *sql.DB),
) {
    // Запускаем отдельные горутины для каждой команды
    go func() {
        log.Printf("Daily /giga command routine started")
        for {
            // Генерируем случайное время в течение дня (0-23 часа, 0-59 минут)
            randomHour := rand.Intn(24)
            randomMinute := rand.Intn(60)
            
            // Текущее время
            now := time.Now()
            
            // Создаем время для сегодняшнего запуска
            nextRun := time.Date(now.Year(), now.Month(), now.Day(), randomHour, randomMinute, 0, 0, now.Location())
            
            // Если время уже прошло, переносим на завтра
            if nextRun.Before(now) {
                nextRun = nextRun.AddDate(0, 0, 1)
            }
            
            // Вычисляем время ожидания
            waitTime := time.Until(nextRun)
            
            log.Printf("Next daily /giga command will run at: %s (in %v)",
                nextRun.Format("2006-01-02 15:04:05"), waitTime)

            // Ждем до назначенного времени
            time.Sleep(waitTime)
            
            log.Printf("Executing daily /giga command...")
            
            // Создаем фейковое обновление для /giga
            gigaUpdate := createFakeUpdate(chatID, "/giga")
            gigaHandler(gigaUpdate, bot, db)
            
            log.Printf("Daily /giga command executed successfully")
        }
    }()

    go func() {
        log.Printf("Daily /unh command routine started")
        for {
            // Генерируем случайное время в течение дня (0-23 часа, 0-59 минут)
            randomHour := rand.Intn(24)
            randomMinute := rand.Intn(60)
            
            // Текущее время
            now := time.Now()
            
            // Создаем время для сегодняшнего запуска
            nextRun := time.Date(now.Year(), now.Month(), now.Day(), randomHour, randomMinute, 0, 0, now.Location())
            
            // Если время уже прошло, переносим на завтра
            if nextRun.Before(now) {
                nextRun = nextRun.AddDate(0, 0, 1)
            }
            
            // Вычисляем время ожидания
            waitTime := time.Until(nextRun)
            
            log.Printf("Next daily /unh command will run at: %s (in %v)", 
                nextRun.Format("2006-01-02 15:04:05"), waitTime)
            
            // Ждем до назначенного времени
            time.Sleep(waitTime)
            
            log.Printf("Executing daily /unh command...")
            
            // Создаем фейковое обновление для /unh
            unhUpdate := createFakeUpdate(chatID, "/unh")
            unhHandler(unhUpdate, bot, db)
            
            log.Printf("Daily /unh command executed successfully")
        }
    }()
}

// createFakeUpdate создает фейковое обновление для имитации команды
func createFakeUpdate(chatID int64, command string) tgbotapi.Update {
    return tgbotapi.Update{
        Message: &tgbotapi.Message{
            MessageID: 0,
            From: &tgbotapi.User{
                ID:        0,
                FirstName: "Daily",
                LastName:  "Bot",
                UserName:  "daily_bot",
            },
            Chat: &tgbotapi.Chat{
                ID: chatID,
            },
            Date: int(time.Now().Unix()),
            Text: command,
        },
    }
}
