package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TopGiga обрабатывает команду "топ гигачад"
func TopGiga(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Выполнение SQL-запроса для получения топа по гигачатам
	rows, err := db.Query("SELECT pen_name, handsome_count FROM pens WHERE tg_chat_id = ? ORDER BY handsome_count DESC LIMIT 10", chatID)
	if err != nil {
		log.Printf("Error querying top gigachat: %v", err)
		return
	}
	defer rows.Close()

	// Формирование сообщения с рейтингом
	var sb strings.Builder
	sb.WriteString("Топ 10 гигачатов:\n")
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			log.Printf("Error scanning row: %v", err)
			return
		}
		sb.WriteString(fmt.Sprintf("%s: %d раз\n", name, count))
	}

	// Отправка сообщения
	// app.SendMessage(chatID, sb.String(), bot, update.Message.MessageID)
}