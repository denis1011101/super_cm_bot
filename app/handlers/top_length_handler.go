package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/denis1011101/super_cm_bot/app"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TopLengthStruct struct {
	pen_length int    `db:"pen_length"`
	pen_name   string `db:"pen_name"`
	PenComment string
	PenSm      string
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
	GiantPenComment := []string{"Настоящий гигачад с елдой ", "Полупокер но с большим хреном ", "Лучше быть третьим чем выступать в цирке ", "Rуколд с "}
	MicroPenComment := "У него писунька "
	GiantPenSm := []string{" см 😱", " см 💪", " см 🐺", "см 🤡"}
	MicroPenSm := " см 🤡"

	// Обработка результатов запроса
	for i := 0; rows.Next(); i++ {
		var record TopLengthStruct
		if err := rows.Scan(&record.pen_length, &record.pen_name); err != nil {
			panic(err)
		}

		// Присвоение комментариев в зависимости от индекса
		if i < 4 {
			record.PenComment = GiantPenComment[i]
			record.PenSm = GiantPenSm[i]
		} else {
			record.PenComment = MicroPenComment
			record.PenSm = MicroPenSm
		}

		records = append(records, record)
	}

	// Формирование сообщения с рейтингом
	var sb strings.Builder
	sb.WriteString("Топ 10 по длине пениса:\n")
	for _, record := range records {
		sb.WriteString(fmt.Sprintf("@%s: %s %d %s\n", record.pen_name, record.PenComment, record.pen_length, record.PenSm))
	}

	message := sb.String()

	// Отправка сообщения
	app.SendMessage(chatID, message, bot, update.Message.MessageID)

}
