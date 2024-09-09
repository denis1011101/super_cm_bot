package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/denis1011101/super_cum_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Topunhandsome обрабатывает команду "топ пидор"
func TopUnhandsome(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Подготовка запроса для получения топа по пидорам
	stmt, err := db.Prepare(`
		SELECT pen_name, unhandsome_count 
		FROM pens 
		WHERE tg_chat_id = ? 
		ORDER BY unhandsome_count DESC 
		LIMIT 10
	`)
	if err != nil {
		log.Printf("Error preparing query statement: %v", err)
		return
	}
	defer stmt.Close()

	// Выполнение подготовленного запроса с параметрами
	rows, err := stmt.Query(chatID)
	if err != nil {
		log.Printf("Error querying top unhandsome: %v", err)
		return
	}
	defer rows.Close()

	// Формирование сообщения с рейтингом
	var sb strings.Builder
	sb.WriteString("Топ 10 пидоров:\n")
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
	app.SendMessage(chatID, sb.String(), bot, update.Message.MessageID)
}