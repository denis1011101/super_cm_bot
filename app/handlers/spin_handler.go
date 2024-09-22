package handlers

import (
	"database/sql"
	"log"
	"fmt"

	"github.com/denis1011101/super_cum_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleSpin(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
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
	shouldReturn := checkIsSpinNotLegal(pen.LastUpdateTime)
	if shouldReturn {
		app.SendMessage(chatID, "Могу только по губам поводить. Приходи позже...", bot, update.Message.MessageID)
		return
	}

	// Выполнение спина
	result := app.SpinPenSize(pen)
	log.Printf("Spin result: %+v", result)

	// Обновление размера  и времени последнего обновления в базе данных
	newSize := pen.Size + result.Size
	app.UpdateUserPen(db, userID, chatID, newSize)
	log.Printf("Updated pen size: %d", newSize)

	//Отправка ответного сообщения
	var responseText string
	switch result.ResultType {
	case "ADD":
		switch result.Size {
		case 1:
			responseText = fmt.Sprintf("+1 и все. Твой сайз: %d см", newSize)
		case 2:
			responseText = fmt.Sprintf("+2 это уже лучше чем +1 🤡 Твой сайз: %d см", newSize)
		case 3:
			responseText = fmt.Sprintf("+3 на повышение идешь?🍆 Твой сайз: %d см", newSize)
		case 4:
			responseText = fmt.Sprintf("+4 воу чел! Я смотрю ты подходишь к делу серьезно 😎 Твой сайз: %d см", newSize)
		case 5:
			responseText = fmt.Sprintf("Это RAMPAGE🔥 +5 АУФ волчара 🐺 Твой сайз: %d см", newSize)
		}
	case "DIFF":
		switch result.Size {
		case -1:
			responseText = fmt.Sprintf("-1 ты чё, пидр? Да я шучу. Твой сайз: %d см", newSize)
		case -2:
			responseText = fmt.Sprintf("-2 не велика потеря, бро 🥸 Твой сайз: %d см", newSize)
		case -3:
			responseText = fmt.Sprintf("-3 это хуже чем +1 🤡 Твой сайз: %d см", newSize)
		case -4:
			responseText = fmt.Sprintf("-4 не переживай, до свадьбы отрастет 🤥 Твой сайз: %d см", newSize)
		case -5:
			responseText = fmt.Sprintf("У тебя -5, петушара🐓 И я не шучу. Твой сайз: %d см", newSize)
		}
	case "RESET":
		responseText = fmt.Sprintf("Теперь ты просто пезда. Твой сайз: %d см", newSize)
	case "ZERO":
		responseText = fmt.Sprintf("Чеееел... у тебя 0 см прибавилось. Твой сайз: %d см", newSize)
	}

	log.Printf("Response text: %s", responseText)
	app.SendMessage(chatID, responseText, bot, update.Message.MessageID)
}
