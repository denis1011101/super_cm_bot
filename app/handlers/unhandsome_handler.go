package handlers

import (
	"database/sql"
	"log"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	messagegenerators "github.com/denis1011101/super_cm_bot/app/handlers/message_generators"
	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func normalizeUnhandsomeDiffSize(diffSize int) int {
	return min(diffSize, -1)
}

var unhSelection = selectionCommand{name: "/unh", lastUpdate: app.GetUnhandsomeLastUpdateTime, choose: chooseUnhandsome}

func ChooseUnhandsome(update models.Update, bot *telegram.Bot, db *sql.DB) {
	unhSelection.runManual(update, bot, db, nil)
}

func chooseUnhandsome(db *sql.DB, chatID int64) (string, bool) {
	now := time.Now()
	isHoliday := (now.Month() == time.December && now.Day() >= 24) || (now.Month() == time.January && now.Day() <= 2)

	// Проводим ролл на пропуск выбора пидора дня
	if app.SpinSkipAction() {
		if err := app.UpdateUnhandsomeLastUpdate(db, chatID); err != nil {
			log.Printf("Error updating unhandsome last update: %v", err)
			return "", false
		}
		return messagegenerators.GetSkipUnhandsomeMessage(isHoliday), true
	}

	// Преобразование penNames в список объектов Member
	members, err := app.GetPenNames(db, chatID)
	if err != nil {
		log.Printf("Error getting pen names: %v", err)
		return "", false
	}

	if len(members) <= 1 {
		return "Недостаточно пенисов в чате 💅", false
	}

	// Выбор случайного участника
	randomMember := app.SelectRandomMember(members)

	// Получение текущего размера пениса выбранного участника
	pen, err := app.GetUserPen(db, randomMember.ID, chatID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No pen size found for tg_pen_id: %d in chat_id: %d", randomMember.ID, chatID)
		} else {
			log.Printf("Error getting current pen size: %v", err)
		}
		return "", false
	}
	log.Printf("Current pen size for tg_pen_id %d in chat_id %d: %d", randomMember.ID, chatID, pen.Size)

	// Вычисление нового размера
	result := app.SpinDiffPenSize(pen)
	if result.ResultType != "RESET" {
		result.Size = normalizeUnhandsomeDiffSize(result.Size) // гарантируем уменьшение минимум на 1
	}
	newSize := pen.Size + result.Size

	// Обновление значения у выигравшего участника и времени последнего обновления у всех участников
	if err := app.UpdateUnhandsome(db, newSize, randomMember.ID, chatID); err != nil {
		log.Printf("Error updating unhandsome in database: %v", err)
		return "", false
	}

	return messagegenerators.GetRandomUnhandsomeMessage(randomMember.Name, result.Size, newSize, isHoliday), true
}
