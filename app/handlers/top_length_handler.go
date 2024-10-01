package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TopLengthStruct struct {
	ID       int    `db:"pen_length"`
	Data     string `db:"pen_name"`
	Comment  string
	Comment1 string
}

// TopLength обрабатывает команду "топ длина"
func TopLength(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Подготовка запроса для получения топа по длине
	stmt, err := db.Prepare(`
		SELECT pen_length, pen_name
		FROM pens 
		WHERE tg_chat_id = ? 
		ORDER BY pen_length DESC 
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
		log.Printf("Error querying top length: %v", err)
		return
	}
	defer rows.Close()

	var records []TopLengthStruct
	uniqueComments := []string{"Настоящий гигачад с елдой ", "Полупокер но с большим хреном ", "Лучше быть третьим чем выступать в цирке "}
	commonComment := "У него писунька "
	uniqueComments1 := []string{" см 😱", " см 💪", " см 🐺"}
	commonComment1 := " см 🤡"

	// Обработка результатов запроса
	for i := 0; rows.Next(); i++ {
		var record TopLengthStruct
		if err := rows.Scan(&record.ID, &record.Data); err != nil {
			panic(err)
		}

		// Присвоение комментариев в зависимости от индекса
		if i < 3 {
			record.Comment = uniqueComments[i]
			record.Comment1 = uniqueComments1[i]
		} else {
			record.Comment = commonComment
			record.Comment1 = commonComment1
		}

		records = append(records, record)
	}

	// Формирование сообщения с рейтингом
	var sb strings.Builder
	sb.WriteString("Топ 10 по длине пениса:\n")
	for _, record := range records {
		sb.WriteString(fmt.Sprintf("@%s: %s %d %s\n", record.Data, record.Comment, record.ID, record.Comment1))
	}

	message := sb.String()

	// Отправка сообщения
	msg := tgbotapi.NewMessage(chatID, message)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Ошибка при отправке сообщения: %v", err)
	}
}
