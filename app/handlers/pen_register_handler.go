package handlers

import (
	"database/sql"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandlepenCommand регистрирует всех пользователей кто пишет в чат
func HandlePenCommand(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	registerBot(update, bot, db, false)
}
