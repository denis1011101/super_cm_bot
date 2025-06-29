package handlers

import (
	"database/sql"
	"log"

	"github.com/denis1011101/super_cm_bot/app"
	messagegenerators "github.com/denis1011101/super_cm_bot/app/handlers/message_generators"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ChooseUnhandsome(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
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

	// Получение времени последнего обновления
	lastUpdate, err := app.GetUnhandsomeLastUpdateTime(db, chatID)
	if err != nil {
		return
	}

	// Проверка времени последнего обновления
	shouldReturn := checkIsSpinNotLegal(lastUpdate)
	if shouldReturn { // TODO: Добавить вывод На сегодня пидоров хватит. Если чё пидор сегодня @%s
		app.SendMessage(chatID, "Могу только по губам поводить. Приходи позже...", bot, update.Message.MessageID)
		return
	}

	// Проводим ролл на пропуск выбора пидора дня
	if app.SpinSkipAction() {
		if err := app.UpdateUnhandsomeLastUpdate(db, chatID); err != nil {
			log.Printf("Error updating unhandsome last update: %v", err)
		}
		message := messagegenerators.GetSkipUnhandsomeMessage()
		app.SendMessage(chatID, message, bot, update.Message.MessageID)
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
		return
	}
	log.Printf("Current pen size for tg_pen_id %d in chat_id %d: %d", randomMember.ID, chatID, pen.Size)

	// Вычисление нового размера
	result := app.SpinDiffPenSize(pen)
	newSize := pen.Size + result.Size

	// Обновление значения у выигравшего участника и времени последнего обновления у всех участников
	app.UpdateUnhandsome(db, newSize, randomMember.ID, chatID)

	// Генерируем сообщение для чата
	message := messagegenerators.GetRandomUnhandsomeMessage(randomMember.Name, result.Size, newSize)

	// Отправка сообщения с именем выбранного "антикрасавчика"
	app.SendMessage(chatID, message, bot, update.Message.MessageID)
}
