package handlers

import (
	"database/sql"
	"log"

	"github.com/denis1011101/super_cm_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandlepenCommand регистрирует всех пользователей кто пишет в чат
func HandlePenCommand(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Проверка наличия пользователя в базе данных
	exists, err := app.UserExists(db, userID, chatID)
	if err != nil {
		log.Printf("Error checking if user exists: %v", err)
		return
	}
	
	// Регистрация пользователя, если он не найден в базе данных и это не событие входа пользователя в чат
	// или если он не найден в базе данных и вступил в чат
	if !exists && (!isUserLeaveEvent(update) || isUserJoinEvent(update)) {
		log.Printf("User not found in database, registering: %v", userID)
		registerBot(update, bot, db, true)
	}

	// Получение текущего размера пениса пользователя
	pen, err := app.GetUserPen(db, userID, chatID)
	if err != nil {
		log.Printf("Error querying pen size: %v", err)
		return
	}

	log.Printf("Current pen size for tg_pen_id %d in chat_id %d: %d", userID, chatID, pen.Size)
}
