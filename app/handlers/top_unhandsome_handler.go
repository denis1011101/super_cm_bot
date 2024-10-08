package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/denis1011101/super_cm_bot/app"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TopUnhandsomeStruct struct {
	unhandsome_count  int    `db:"unhandsome_count"`
	pen_name          string `db:"pen_name"`
	UnhandsomeComment string
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
	TheMostUnhandsome := []string{"Самый крепкий анус на деревне 🐓", "Около пидорства 💩"}
	Straight := "Может даже он натурал 🤡"

	// Обработка результатов запроса
	for i := 0; rows.Next(); i++ {
		var record TopUnhandsomeStruct
		if err := rows.Scan(&record.unhandsome_count, &record.pen_name); err != nil {
			panic(err)
		}

		// Присвоение комментариев в зависимости от индекса
		if i < 2 {
			record.UnhandsomeComment = TheMostUnhandsome[i]
		} else {
			record.UnhandsomeComment = Straight
		}

		records = append(records, record)
	}

	// Формирование сообщения с рейтингом
	var sb strings.Builder
	sb.WriteString("Топ 10 пидоров:\n")
	for _, record := range records {
		sb.WriteString(fmt.Sprintf("@%s: %d раз. %s\n", record.pen_name, record.unhandsome_count, record.UnhandsomeComment))
	}

	message := sb.String()

	// Отправка сообщения
	app.SendMessage(chatID, message, bot, update.Message.MessageID)

}
