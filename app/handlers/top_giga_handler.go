package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/denis1011101/super_cm_bot/app"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TopGigaStruct struct {
	handsome_count int    `db:"handsome_count"`
	pen_name       string `db:"pen_name"`
	TopGigaComment string
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
	defer func() {
		if closeErr := stmt.Close(); closeErr != nil {
			log.Printf("Error closing statement: %v", closeErr)
		}
	}()

	// Выполнение подготовленного запроса с параметрами
	rows, err := stmt.Query(chatID)
	if err != nil {
		log.Printf("Error querying top gigachat: %v", err)
		return
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("Error closing rows: %v", closeErr)
		}
	}()

	var records []TopGigaStruct
	TheMostGiga := []string{"Альфа самец 💪😎", "Четкий пацан 🐺"}
	AspiringToGiga := "Похож на пидора 🤡"

	// Обработка результатов запроса
	for i := 0; rows.Next(); i++ {
		var record TopGigaStruct
		if err := rows.Scan(&record.handsome_count, &record.pen_name); err != nil {
			log.Printf("Error scanning row: %v", err)
			return
		}

		// Присвоение комментариев в зависимости от индекса
		if i < 2 {
			record.TopGigaComment = TheMostGiga[i]
		} else {
			record.TopGigaComment = AspiringToGiga
		}

		records = append(records, record)
	}

	// Формирование сообщения с рейтингом
	var sb strings.Builder
	sb.WriteString("Топ 10 гигачадов:\n")
	for _, record := range records {
		sb.WriteString(fmt.Sprintf("@%s: %d раз. %s\n", record.pen_name, record.handsome_count, record.TopGigaComment))
	}

	message := sb.String()

	// Отправка сообщения
	app.SendMessage(chatID, message, bot, update.Message.MessageID)

}
