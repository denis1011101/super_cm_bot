package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func checkIsSpinNotLegal(lastUpdate time.Time) bool {
	if !lastUpdate.IsZero() {
		duration := time.Since(lastUpdate)

		if duration.Hours() < 4 {
			log.Println("Spin is not legal: less than 4 hours since last update")
			return true
		}
	}
	log.Println("Spin is legal")
	return false
}

func registerBot(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB, sendWelcomeMessage bool) {
	// Логика регистрации в боте
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	userName := update.Message.From.UserName

	// Подготовка запроса для вставки пользователя в базу данных
	stmt, err := db.Prepare(`
		INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length, handsome_count, unhandsome_count)
		VALUES (?, ?, ?, ?, 0, 0)
	`)
	if err != nil {
		log.Printf("Error preparing insert statement: %v", err)
		return
	}
	defer func() {
		if closeErr := stmt.Close(); closeErr != nil {
			log.Printf("Error closing statement: %v", closeErr)
		}
	}()

	// Выполнение подготовленного запроса с параметрами и добавление 5 см к размеру пениса
	_, err = stmt.Exec(userName, userID, chatID, 5)
	if err != nil {
		log.Printf("Error inserting user into database: %v", err)
		return
	}

	// Отправка ответного сообщения, если флаг установлен
	if sendWelcomeMessage {
		app.SendMessage(chatID, "Велком ту зе клаб, бади 😎🤝😎", bot, update.Message.MessageID)
	}

	fmt.Println("User registered in bot")
}

// isUserJoinEvent проверяет, является ли сообщение событием входа пользователя в чат
func isUserJoinEvent(update tgbotapi.Update) bool {
    return update.Message.NewChatMembers != nil
}

// isUserLeaveEvent проверяет, является ли сообщение событием выхода пользователя из чата
func isUserLeaveEvent(update tgbotapi.Update) bool {
    return update.Message.LeftChatMember != nil
}
