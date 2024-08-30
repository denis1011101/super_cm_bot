package app

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleBotAddition обрабатывает добавление бота в чат
func HandleBotAddition(update tgbotapi.Update, bot *tgbotapi.BotAPI) {
	if update.MyChatMember.NewChatMember.User.UserName == bot.Self.UserName {
		if update.MyChatMember.Chat.IsGroup() || update.MyChatMember.Chat.IsSuperGroup() {
			log.Printf("Bot added to group: %s", update.MyChatMember.Chat.Title)
			// sendMessage(update.MyChatMember.Chat.ID, "Здарова! Я ваш новый папочка 😈 Жмякай на кнопку, если не ссылко: /pen", bot, 0)
		} else if update.MyChatMember.Chat.IsPrivate() {
			log.Printf("Bot added to private chat with: %s", update.MyChatMember.From.UserName)
			// sendMessage(update.MyChatMember.Chat.ID, "Этот бот работает только в группах.", bot, 0)
		}
	}

	if update.Message.GroupChatCreated {
		log.Printf("Создан новый групповой чат: %s", update.Message.Chat.Title)
		// sendMessage(update.Message.Chat.ID, "Привет! Я ваш новый бот. Жмякай на кнопку, если не ссылко: /pen", bot, 0)
	}
}

// HandleSpin обрабатывает команду "спин"
func HandleSpin(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Получение текущего размера пениса пользователя из базы данных
	var currentSize int
	var lastUpdate sql.NullTime
	err := db.QueryRow("SELECT pen_length, pen_last_update_at FROM pens WHERE tg_pen_id = ? AND tg_chat_id = ?", userID, chatID).Scan(&currentSize, &lastUpdate)
	if err != nil {
		if err == sql.ErrNoRows {
			// Регистрация пользователя, если он не найден в базе данных
			RegisterBot(update, bot, db, true)
			return
		}
		log.Printf("Error querying pen size: %v", err)
		return
	}

	// Проверка времени последнего обновления
	if lastUpdate.Valid {
		duration := time.Since(lastUpdate.Time)
		if duration.Seconds() < 24 {
			// sendMessage(chatID, "Могу только по губам поводить. Приходи позже...", bot, update.Message.MessageID)
			return
		}
	}

	// Выполнение спина
	pen := pen{Size: currentSize}
	result := SpinpenSize(pen)

	// Обновление размера пениса и времени последнего обновления в базе данных
	newSize := currentSize + result.Size
	_, err = db.Exec("UPDATE pens SET pen_length = ?, pen_last_update_at = ? WHERE tg_pen_id = ? AND tg_chat_id = ?", newSize, time.Now(), userID, chatID)
	if err != nil {
		log.Printf("Error updating pen size and last update time: %v", err)
		return
	}

	// Отправка ответного сообщения
	// var responseText string
	// switch result.ResultType {
	// case "ADD":
	// 	switch result.Size {
	// 	case 1:
	// 		responseText = fmt.Sprintf("+1 и все. Твой сайз: %d см", newSize)
	// 	case 2:
	// 		responseText = fmt.Sprintf("+2 это уже лучше чем +1 🤡 Твой сайз: %d см", newSize)
	// 	case 3:
	// 		responseText = fmt.Sprintf("+3 на повышение идешь?🍆 Твой сайз: %d см", newSize)
	// 	case 4:
	// 		responseText = fmt.Sprintf("+4 воу чел! Я смотрю ты подходишь к делу серьезно 😎 Твой сайз: %d см", newSize)
	// 	case 5:
	// 		responseText = fmt.Sprintf("Это RAMPAGE🔥 +5 АУФ волчара 🐺 Твой сайз: %d см", newSize)
	// 	}
	// case "DIFF":
	// 	switch result.Size {
	// 	case -1:
	// 		responseText = fmt.Sprintf("-1 ты чё пидр? Да я шучу. Твой сайз: %d см", newSize)
	// 	case -2:
	// 		responseText = fmt.Sprintf("-2 не велика потеря бро 🥸 Твой сайз: %d см", newSize)
	// 	case -3:
	// 		responseText = fmt.Sprintf("-3 это хуже чем +1 🤡 Твой сайз: %d см", newSize)
	// 	case -4:
	// 		responseText = fmt.Sprintf("-4 не переживай до свадьбы отрастет 🤥 Твой сайз: %d см", newSize)
	// 	case -5:
	// 		responseText = fmt.Sprintf("У тебя -5 петушара🐓 И я не шучу. Твой сайз: %d см", newSize)
	// 	}
	// case "RESET":
	// 	responseText = "Теперь ты просто пезда. Твой сайз: zero см"
	// case "ZERO":
	// 	responseText = "Чеееел... у тебя 0 см прибавилось. Твой сайз: %d см"
	// }

	// sendMessage(chatID, responseText, bot, update.Message.MessageID)
}

// ChooseGiga выбирает "красавчика"
func ChooseGiga(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Проверка времени последнего обновления
	var lastUpdate sql.NullTime
	err := db.QueryRow("SELECT MAX(handsome_last_update_at) FROM pens WHERE tg_chat_id = ?", chatID).Scan(&lastUpdate)
	if err != nil {
		log.Printf("Error querying last update time: %v", err)
		return
	}

	if lastUpdate.Valid {
		duration := time.Since(lastUpdate.Time)
		if duration.Seconds() < 24 {
			// sendMessage(chatID, "Вы можете выбрать красавчика только раз в 24 часа.", bot, update.Message.MessageID)
			return
		}
	}

	// Получение списка участников группы через получение всех pen_name из базы данных
	penNames, err := GetPenNames(db)
	if err != nil {
		log.Printf("Error getting pen names: %v", err)
		return
	}

	for _, penName := range penNames {
		log.Printf("Pen Name: %v", penName)
	}

	// if len(penNames) <= 1 {
	// 	sendMessage(chatID, "Недостаток пенисов в чате!", bot, update.Message.MessageID)
	// 	return
	// }

    // Преобразование penNames в список объектов Member
    members, err := GetPenNames(db)
    if err != nil {
        log.Printf("Error getting pen names: %v", err)
        return
    }

	// Выбор случайного участника
	randomMember := SpinunhandsomeOrGiga(members)

    // Получение текущего размера пениса выбранного участника
    var currentSize int
    err = db.QueryRow("SELECT pen_length FROM pens WHERE tg_pen_id = ? AND tg_chat_id = ?", randomMember.ID, chatID).Scan(&currentSize)
    if err != nil {
        if err == sql.ErrNoRows {
            log.Printf("No pen size found for tg_pen_id: %d in chat_id: %d", randomMember.ID, chatID)
        } else {
            log.Printf("Error getting current pen size: %v", err)
        }
        return
    }
    log.Printf("Current pen size for tg_pen_id %d in chat_id %d: %d", randomMember.ID, chatID, currentSize)

	// Вычисление нового размера
	result := SpinAddpenSize(pen{Size: currentSize})
	newSize := currentSize + result.Size

	// Обновление значения у выигравшего участника и времени последнего обновления у всех участников
	_, err = db.Exec("UPDATE gigas SET giga_count = giga_count + 1 WHERE tg_pen_id = ? AND tg_chat_id = ?", newSize, randomMember.Name, chatID)
	if err != nil {
		log.Printf("Error updating giga count: %v", err)
		return
	}

	_, err = db.Exec("UPDATE gigas SET handsome_last_update_at = ? WHERE tg_chat_id = ?", time.Now(), chatID)
	if err != nil {
		log.Printf("Error updating last update time: %v", err)
		return
	}

	// Отправка сообщения с именем выбранного "красавчика"
	// sendMessage(chatID, fmt.Sprintf("Воу воу воу паприветсвуйте хасанчика @%s!🔥Твой член стал длиннее на %d см. Теперь он %d см.", randomMember.Name, result.Size, newSize), bot, update.Message.MessageID)
}


// ChooseUnhandsome выбирает "антикрасавчика"
func ChooseUnhandsome(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Проверка времени последнего обновления
	var lastUpdate sql.NullTime
	err := db.QueryRow("SELECT MAX(unhandsome_last_update_at) FROM pens WHERE tg_chat_id = ?", chatID).Scan(&lastUpdate)
	if err != nil {
		log.Printf("Error querying last update time: %v", err)
		return
	}

	if lastUpdate.Valid {
		duration := time.Since(lastUpdate.Time)
		if duration.Seconds() < 24*60*60 {
			// sendMessage(chatID, "Вы можете выбрать красавчика только раз в 24 часа.", bot, update.Message.MessageID)
			return
		}
	}

	// Получение списка участников группы через получение всех pen_name из базы данных
	penNames, err := GetPenNames(db)
	if err != nil {
		log.Printf("Error getting pen names: %v", err)
		return
	}

	for _, penName := range penNames {
		log.Printf("Pen Name: %v", penName)
	}

	// if len(penNames) <= 1 {
	// 	sendMessage(chatID, "Недостаток пенисов в чате!", bot, update.Message.MessageID)
	// 	return
	// }

    // Преобразование penNames в список объектов Member
    members, err := GetPenNames(db)
    if err != nil {
        log.Printf("Error getting pen names: %v", err)
        return
    }

	// Выбор случайного участника
	randomMember := SpinunhandsomeOrGiga(members)

    // Получение текущего размера пениса выбранного участника
    var currentSize int
    err = db.QueryRow("SELECT pen_length FROM pens WHERE tg_pen_id = ? AND tg_chat_id = ?", randomMember.ID, chatID).Scan(&currentSize)
    if err != nil {
        if err == sql.ErrNoRows {
            log.Printf("No pen size found for tg_pen_id: %d in chat_id: %d", randomMember.ID, chatID)
        } else {
            log.Printf("Error getting current pen size: %v", err)
        }
        return
    }
    log.Printf("Current pen size for tg_pen_id %d in chat_id %d: %d", randomMember.ID, chatID, currentSize)

	// Вычисление нового размера
	result := SpinAddpenSize(pen{Size: currentSize})
	newSize := currentSize - result.Size

	// Обновление значения у выигравшего участника и времени последнего обновления у всех участников
	_, err = db.Exec("UPDATE gigas SET unhandsome_count = unhandsome_count + 1 WHERE tg_user_name = ? AND tg_chat_id = ?", newSize, randomMember.Name, chatID)
	if err != nil {
		log.Printf("Error updating unhandsome count: %v", err)
		return
	}

	_, err = db.Exec("UPDATE gigas SET unhandsome_last_update_at = ? WHERE tg_chat_id = ?", time.Now(), chatID)
	if err != nil {
		log.Printf("Error updating last update time: %v", err)
		return
	}

	// Отправка сообщения с именем выбранного "антикрасавчика"
	// sendMessage(chatID, fmt.Sprintf("Пусть пидором будет @%s! Твой a стал короче на %d см. Теперь он %d см.", randomMember.Name, result.Size, newSize), bot, update.Message.MessageID)
}

// TopLength обрабатывает команду "топ длин"
func TopLength(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Выполнение SQL-запроса для получения топа по длине
	rows, err := db.Query("SELECT pen_name, pen_length FROM pens WHERE tg_chat_id = ? ORDER BY pen_length DESC LIMIT 10", chatID)
	if err != nil {
		log.Printf("Error querying top length: %v", err)
		return
	}
	defer rows.Close()

	// Формирование сообщения с рейтингом
	var sb strings.Builder
	sb.WriteString("Топ 10 по длине пениса:\n")
	for rows.Next() {
		var name string
		var length int
		if err := rows.Scan(&name, &length); err != nil {
			log.Printf("Error scanning row: %v", err)
			return
		}
		sb.WriteString(fmt.Sprintf("%s: %d см\n", name, length))
	}

	// Отправка сообщения
	// sendMessage(chatID, sb.String(), bot, update.Message.MessageID)
}

// TopGiga обрабатывает команду "топ гигачат"
func TopGiga(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Выполнение SQL-запроса для получения топа по гигачатам
	rows, err := db.Query("SELECT pen_name, handsome_count FROM pens WHERE tg_chat_id = ? ORDER BY handsome_count DESC LIMIT 10", chatID)
	if err != nil {
		log.Printf("Error querying top gigachat: %v", err)
		return
	}
	defer rows.Close()

	// Формирование сообщения с рейтингом
	var sb strings.Builder
	sb.WriteString("Топ 10 гигачатов:\n")
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			log.Printf("Error scanning row: %v", err)
			return
		}
		sb.WriteString(fmt.Sprintf("%s: %d раз\n", name, count))
	}

	// Отправка сообщения
	// sendMessage(chatID, sb.String(), bot, update.Message.MessageID)
}

// Topunhandsome обрабатывает команду "топ пидор"
func TopUnhandsome(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	chatID := update.Message.Chat.ID

	// Выполнение SQL-запроса для получения топа по пидорам
	rows, err := db.Query("SELECT pen_name, unhandsome_count FROM pens WHERE tg_chat_id = ? ORDER BY unhandsome_count DESC LIMIT 10", chatID)
	if err != nil {
		log.Printf("Error querying top unhandsome: %v", err)
		return
	}
	defer rows.Close()

	// Формирование сообщения с рейтингом
	var sb strings.Builder
	sb.WriteString("Топ 10 пидоров:\n")
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			log.Printf("Error scanning row: %v", err)
			return
		}
		sb.WriteString(fmt.Sprintf("%s: %d раз\n", name, count))
	}

	// Отправка сообщения
	// sendMessage(chatID, sb.String(), bot, update.Message.MessageID)
}

// HandlepenCommand регистрирует всех пользователей кто пишет в чат
func HandlepenCommand(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	RegisterBot(update, bot, db, false)
}

// TODO: остальную статистику добавим руками
