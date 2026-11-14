package handlers

import (
	"database/sql"
	"fmt"
	"log"
	mathrand "math/rand"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
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

	// Holiday multiplier: в период 24 Dec..31 Dec и 1..2 Jan в 2/3 случаев умножаем результат спина на случайный 1..5
	now := time.Now()
	isHoliday := (now.Month() == time.December && now.Day() >= 24) || (now.Month() == time.January && now.Day() <= 2)
	if isHoliday {
		pos := mathrand.Intn(3) // 0 = no multiplier (1/3), 1..2 = apply multiplier (2/3)
		if pos != 0 {
			mul := mathrand.Intn(5) + 1 // 1..5
			result.Size = result.Size * mul
			log.Printf("Holiday multiplier applied to spin: x%d (new result.Size=%d)", mul, result.Size)
		}
	}

	// Обновление размера  и времени последнего обновления в базе данных
	newSize := pen.Size + result.Size
	if err := app.UpdateUserPen(db, userID, chatID, newSize); err != nil {
		log.Printf("Error updating pen size: %v", err)
		return
	}
	log.Printf("Updated pen size: %d", newSize)

	//Отправка ответного сообщения
	sizeLabel := "Твой сайз"
	if isHoliday {
		sizeLabel = "Твой новогодний сайз"
	}

	var responseText string
	switch result.ResultType {
	case "ADD":
		switch result.Size {
		case 1:
			responseText = fmt.Sprintf("+1 и все. %s: %d см", sizeLabel, newSize)
		case 2:
			responseText = fmt.Sprintf("+2 это уже лучше чем +1 🤡 %s: %d см", sizeLabel, newSize)
		case 3:
			responseText = fmt.Sprintf("+3 на повышение идешь?🍆 %s: %d см", sizeLabel, newSize)
		case 4:
			responseText = fmt.Sprintf("+4 воу чел! Я смотрю ты подходишь к делу серьезно 😎 %s: %d см", sizeLabel, newSize)
		case 5:
			responseText = fmt.Sprintf("Это RAMPAGE🔥 +5 АУФ волчара 🐺 %s: %d см", sizeLabel, newSize)
		default:
			// Обработка больших прибавок (например из-за множителя)
			responseText = fmt.Sprintf("+%d это охуенно! %s: %d см", result.Size, sizeLabel, newSize)
		}
	case "DIFF":
		switch result.Size {
		case -1:
			responseText = fmt.Sprintf("-1 ты чё, пидр? Да я шучу. %s: %d см", sizeLabel, newSize)
		case -2:
			responseText = fmt.Sprintf("-2 не велика потеря, бро 🥸 %s: %d см", sizeLabel, newSize)
		case -3:
			responseText = fmt.Sprintf("-3 это хуже чем +1 🤡 %s: %d см", sizeLabel, newSize)
		case -4:
			responseText = fmt.Sprintf("-4 не переживай, до свадьбы отрастет 🤥 %s: %d см", sizeLabel, newSize)
		case -5:
			responseText = fmt.Sprintf("У тебя -5, петушара🐓 И я не шучу. %s: %d см", sizeLabel, newSize)
		default:
			// Обработка больших потерь
			responseText = fmt.Sprintf("%d — серьёзный удар, но ты всё ещё в игре. %s: %d см", result.Size, sizeLabel, newSize)
		}
	case "RESET":
		responseText = fmt.Sprintf("Теперь ты просто пезда. %s: %d см", sizeLabel, newSize)
	case "ZERO":
		responseText = fmt.Sprintf("Чеееел... у тебя 0 см прибавилось. %s: %d см", sizeLabel, newSize)
	}

	log.Printf("Response text: %s", responseText)
	app.SendMessage(chatID, responseText, bot, update.Message.MessageID)
}
