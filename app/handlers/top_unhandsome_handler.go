package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TopUnhandsomeStruct struct {
	ID      int    `db:"unhandsome_count"`
	Data    string `db:"pen_name"`
	Comment string
}

// Topunhandsome обрабатывает команду "топ пидор"
func TopUnhandsome(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Подготовка запроса для получения топа по пидорам
	stmt, err := db.Prepare(`
		SELECT unhandsome_count, pen_name 
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

	var records []TopUnhandsomeStruct
	uniqueComments := []string{"Самый крепкий анус на деревне 🐓", "Около пидорства 💩"}
	commonComment := "Может даже он натурал 🤡"

	// Обработка результатов запроса
	for i := 0; rows.Next(); i++ {
		var record TopUnhandsomeStruct
		if err := rows.Scan(&record.ID, &record.Data); err != nil {
			panic(err)
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
	sb.WriteString("Топ 10 пидоров:\n")
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
