package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TopGigaStruct struct {
	ID      int    `db:"handsome_count"`
	Data    string `db:"pen_name"`
	Comment string
}

// TopGiga обрабатывает команду "топ гигачад"
func TopGiga(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Подготовка запроса для получения топа по гигачадам
	stmt, err := db.Prepare(`
		SELECT handsome_count, pen_name 
		FROM pens 
		WHERE tg_chat_id = ? 
		ORDER BY handsome_count DESC 
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
		log.Printf("Error querying top gigachat: %v", err)
		return
	}
	defer rows.Close()

	var records []TopGigaStruct
	uniqueComments := []string{"Альфа самец 💪😎", "Четкий пацан 🐺"}
	commonComment := "Похож на пидора 🤡"

	// Обработка результатов запроса
	for i := 0; rows.Next(); i++ {
		var record TopGigaStruct
		if err := rows.Scan(&record.ID, &record.Data); err != nil {
			log.Printf("Error scanning row: %v", err)
			return
		}

		// Присвоение комментариев в зависимости от индекса
		if i < 2 {
			record.Comment = uniqueComments[i]
		} else {
			record.Comment = commonComment
		}

		records = append(records, record)
	}

	// Формирование сообщения с рейтингом
	var sb strings.Builder
	sb.WriteString("Топ 10 гигачадов:\n")
	for _, record := range records {
		sb.WriteString(fmt.Sprintf("@%s: %d раз. %s\n", record.Data, record.ID, record.Comment))
	}

	message := sb.String()

	// Отправка сообщения
	msg := tgbotapi.NewMessage(chatID, message)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Ошибка при отправке сообщения: %v", err)
	}
}
