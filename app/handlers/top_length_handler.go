package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TopLength(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Выполнение SQL-запроса для получения топа по длине
	rows, err := db.Query("SELECT pen_name, pen_length FROM pens WHERE tg_chat_id = ? ORDER BY pen_length DESC LIMIT 10", chatID)
	if err != nil {
		log.Printf("Error querying top length: %v", err)
		return
	}
	defer rows.Close()

	// Формирование сообщения с рейтингом
	var sb strings.Builder
	sb.WriteString("Топ 10 по длине пениса:\n")
	for rows.Next() {
		var name string
		var length int
		if err := rows.Scan(&name, &length); err != nil {
			log.Printf("Error scanning row: %v", err)
			return
		}
		sb.WriteString(fmt.Sprintf("%s: %d см\n", name, length))
	}

	// Отправка сообщения
	// app.SendMessage(chatID, sb.String(), bot, update.Message.MessageID)
}