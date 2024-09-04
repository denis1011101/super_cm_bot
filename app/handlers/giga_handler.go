package handlers

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/denis1011101/super_cum_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ChooseGiga(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Проверка времени последнего обновления
	lastUpdate, err := app.GetGigaLastUpdateTime(db, chatID)
	if err != nil {
		return
	}

	// Проверка времени последнего обновления
	shouldReturn := checkIsSpinNotLegal(lastUpdate)
	if shouldReturn {
		app.SendMessage(chatID, "Могу только по губам поводить. Приходи позже...", bot, update.Message.MessageID)
		return
	}

	// Преобразование penNames в список объектов Member
	members, err := app.GetPenNames(db, chatID)
	if err != nil {
		log.Printf("Error getting pen names: %v", err)
		return
	}

	// if len(members) <= 1 {
	// 	app.SendMessage(chatID, "Недостаток пенисов в чате!", bot, update.Message.MessageID)
	// 	return
	// }

	for _, penName := range members {
		log.Printf("Pen Name: %v", penName)
	}

	// Выбор случайного участника
	randomMember := app.SpinunhandsomeOrGiga(members)

	// Получение текущего размера пениса выбранного участника
	pen, err := app.GetUserPen(db, randomMember.ID, chatID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No pen size found for tg_pen_id: %d in chat_id: %d", randomMember.ID, chatID)
		} else {
			log.Printf("Error getting current pen size: %v", err)
		}
		return
	}
	log.Printf("Current pen size for tg_pen_id %d in chat_id %d: %d", randomMember.ID, chatID, pen.Size)

	// Вычисление нового размера
	result := app.SpinAddPenSize(pen)
	newSize := pen.Size + result.Size

	// Обновление значения члена и времени последнего обновления у выигравшего участника
	app.UpdateGiga(db, newSize, randomMember.ID, chatID)

	// Отправка сообщения с именем выбранного "красавчика"
	app.SendMessage(chatID, fmt.Sprintf("Воу воу воу паприветсвуйте хасанчика @%s!🔥Твой член стал длиннее на %d см. Теперь он %d см.", randomMember.Name, result.Size, newSize), bot, update.Message.MessageID)
}
