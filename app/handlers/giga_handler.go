package handlers

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/denis1011101/super_cum_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ChooseGiga(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

    // Проверка наличия пользователя в базе данных
    exists, err := app.UserExists(db, userID, chatID)
    if err != nil {
        log.Printf("Error checking if user exists: %v", err)
        return
    }

    if !exists {
        // Регистрация пользователя, если он не найден в базе данных
        log.Printf("User not found in database, registering: %v", userID)
        registerBot(update, bot, db, true)
    }

    // Получение текущего размера пениса пользователя
    pen, err := app.GetUserPen(db, userID, chatID)
    if err != nil {
        log.Printf("Error querying pen size: %v", err)
        return
    }

    log.Printf("Current pen size for tg_pen_id %d in chat_id %d: %d", userID, chatID, pen.Size)

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

	if len(members) <= 1 {
		app.SendMessage(chatID, "Недостаточно пенисов в чате 💅", bot, update.Message.MessageID)
		return
	}

	for _, penName := range members {
		log.Printf("Pen Name: %v", penName)
	}

	// Выбор случайного участника
	randomMember := app.SpinunhandsomeOrGiga(members)

	// Вычисление нового размера
	result := app.SpinAddPenSize(pen)
	newSize := pen.Size + result.Size

	// Обновление значения члена и времени последнего обновления у выигравшего участника
	app.UpdateGiga(db, newSize, randomMember.ID, chatID)

	// Отправка сообщения с именем выбранного "красавчика"
	app.SendMessage(chatID, fmt.Sprintf("Воу воу воу паприветсвуйте хасанчика @%s!🔥Твой член стал длиннее на %d см. Теперь он %d см.", randomMember.Name, result.Size, newSize), bot, update.Message.MessageID)
}
