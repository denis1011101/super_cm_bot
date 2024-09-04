package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/denis1011101/super_cum_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func checkIsSpinNotLegal(lastUpdate time.Time) bool {
	if !lastUpdate.IsZero() {
		duration := time.Since(lastUpdate)
		lastUpdateIsToday := compareTimesByDate(time.Now(), lastUpdate)

		if duration.Hours() < 4 && lastUpdateIsToday {
			log.Println("Spin is not legal: less than 4 hours since last update and it's today")
			return true
		}
	}
	log.Println("Spin is legal")
	return false
}

func compareTimesByDate(a, b time.Time) bool {
	return a.Year() == b.Year() &&
		a.Month() == b.Month() &&
		a.Day() == b.Day()
}

func registerBot(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB, sendWelcomeMessage bool) {
	// Логика регистрации в боте
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	userName := update.Message.From.UserName

	// Вставка пользователя в базу данных
	insertQuery := `
    INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length, handsome_count, unhandsome_count)
    VALUES (?, ?, ?, ?, 0, 0)
    `
	_, err := db.Exec(insertQuery, userName, userID, chatID, 5)
	if err != nil {
		log.Printf("Error inserting user into database: %v", err)
		return
	}

	// Обновление размера пениса
	err = app.UpdatePenSize(db, chatID, 5)
	if err != nil {
		log.Printf("Error updating pen size: %v", err)
		return
	}

	// Отправка ответного сообщения, если флаг установлен
	if sendWelcomeMessage {
		app.SendMessage(chatID, "Велком ту зе клаб, бади 😎🤝😎", bot, update.Message.MessageID)
	}

	fmt.Println("User registered in bot")
}
