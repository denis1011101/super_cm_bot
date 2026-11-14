package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/denis1011101/super_cm_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type topConfig struct {
	query     string
	title     string
	errorLog  string
	formatRow func(index int, name string, value int) string
}

func handleTop(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB, cfg topConfig) {
	if update.Message == nil || update.Message.Chat == nil {
		return
	}
	chatID := update.Message.Chat.ID

	rows, err := db.Query(cfg.query, chatID)
	if err != nil {
		log.Printf("%s: %v", cfg.errorLog, err)
		return
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("Error closing rows: %v", closeErr)
		}
	}()

	var sb strings.Builder
	sb.WriteString(cfg.title)

	for i := 0; rows.Next(); i++ {
		var (
			value   int
			penName string
		)
		if err := rows.Scan(&value, &penName); err != nil {
			log.Printf("Error scanning top row: %v", err)
			return
		}
		sb.WriteString(cfg.formatRow(i, penName, value))
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating top rows: %v", err)
		return
	}

	app.SendMessage(chatID, sb.String(), bot, update.Message.MessageID)
}

var (
	giantPenComment = []string{
		"Настоящий гигачад с елдой ",
		"Полупокер но с большим хреном ",
		"Лучше быть третьим чем выступать в цирке ",
		"Куколд с ",
	}
	giantPenSm = []string{" см 😱", " см 💪", " см 🐺", "см 🤡"}

	theMostGiga    = []string{"Альфа самец 💪😎", "Четкий пацан 🐺"}
	aspiringToGiga = "Похож на пидора 🤡"

	theMostUnhandsome = []string{"Самый крепкий анус на деревне 🐓", "Около пидорства 💩"}
	straight          = "Может даже он натурал 🤡"
)

// TopLength обрабатывает команду "топ длина"
func TopLength(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	handleTop(update, bot, db, topConfig{
		query: `
			SELECT pen_length, pen_name
			FROM pens 
			WHERE tg_chat_id = ? 
			ORDER BY pen_length DESC 
			LIMIT 10
		`,
		title:    "Топ 10 по длине пениса:\n",
		errorLog: "Error querying top length",
		formatRow: func(i int, name string, length int) string {
			comment := "У него писунька "
			sm := " см 🤡"
			if i < 4 {
				comment = giantPenComment[i]
				sm = giantPenSm[i]
			}
			return fmt.Sprintf("@%s: %s %d %s\n", name, comment, length, sm)
		},
	})
}

// TopGiga обрабатывает команду "топ гигачад"
func TopGiga(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	handleTop(update, bot, db, topConfig{
		query: `
			SELECT handsome_count, pen_name 
			FROM pens 
			WHERE tg_chat_id = ? 
			ORDER BY handsome_count DESC 
			LIMIT 10
		`,
		title:    "Топ 10 гигачадов:\n",
		errorLog: "Error querying top gigachat",
		formatRow: func(i int, name string, count int) string {
			comment := aspiringToGiga
			if i < 2 {
				comment = theMostGiga[i]
			}
			return fmt.Sprintf("@%s: %d раз. %s\n", name, count, comment)
		},
	})
}

// TopUnhandsome обрабатывает команду "топ пидор"
func TopUnhandsome(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	handleTop(update, bot, db, topConfig{
		query: `
			SELECT unhandsome_count, pen_name 
			FROM pens 
			WHERE tg_chat_id = ? 
			ORDER BY unhandsome_count DESC 
			LIMIT 10
		`,
		title:    "Топ 10 пидоров:\n",
		errorLog: "Error querying top unhandsome",
		formatRow: func(i int, name string, count int) string {
			comment := straight
			if i < 2 {
				comment = theMostUnhandsome[i]
			}
			return fmt.Sprintf("@%s: %d раз. %s\n", name, count, comment)
		},
	})
}
