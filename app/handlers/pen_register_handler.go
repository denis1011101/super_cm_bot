package handlers

import (
	"database/sql"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/denis1011101/super_cum_bot/app"
)

// HandlepenCommand регистрирует всех пользователей кто пишет в чат
func HandlePenCommand(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Получение текущего размера пениса пользователя из базы данных
	pen, err := app.GetUserPen(db, userID, chatID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Регистрация пользователя, если он не найден в базе данных
			log.Printf("User not found in database, registering: %v", err)
			registerBot(update, bot, db, true)
			return
		}
		log.Printf("Error querying pen size: %v, pen: %+v", err, pen)
		return
	}
}
