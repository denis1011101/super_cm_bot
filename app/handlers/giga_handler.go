package handlers

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	mathrand "math/rand"

	"github.com/denis1011101/super_cm_bot/app"
	messagegenerators "github.com/denis1011101/super_cm_bot/app/handlers/message_generators"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// mass среднестатистического европейского в граммах 130..140 грамм
func averagePenMass() (int, error) {
    nBig, err := rand.Int(rand.Reader, big.NewInt(11)) // 0..10
    if err != nil {
        return 0, err
    }
    return int(nBig.Int64()) + 130, nil // 130..140
}

// Кинетическая энергия
func kineticEnergy(velocity int) (int, error) {
    mass, err := averagePenMass()
    if err != nil {
        return 0, err
    }
    return int(0.5 * float64(mass) * float64(velocity*velocity)), nil
}

// Потенциальная энергия
func potentialEnergy(height int) (int, error) {
    mass, err := averagePenMass()
    if err != nil {
        return 0, err
    }
    const g = 9.81
    return int(float64(mass) * g * float64(height)), nil
}

// voltage среднестатистического европейского во время процесса 1..100 милливольт
func averageVoltage() (int, error) {
    nBig, err := rand.Int(rand.Reader, big.NewInt(100)) // 0..99
    if err != nil {
        return 0, err
    }
    return int(nBig.Int64()) + 1, nil // 1..100
}

// Закон Ома
func ohmLaw(resistance int) (int, error) {
    voltage, err := averageVoltage()
    if err != nil {
        return 0, err
    }
    if resistance == 0 {
        return 0, fmt.Errorf("resistance can't be zero")
    }
    return voltage / resistance, nil
}

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

	// Получение времени последнего обновления
	lastUpdate, err := app.GetGigaLastUpdateTime(db, chatID)
	if err != nil {
		return
	}

	// Проверка времени последнего обновления
	shouldReturn := checkIsSpinNotLegal(lastUpdate)
	if shouldReturn { // TODO: Добавить вывод Сегодня альфа самец @%s и никто его не заменит!
		app.SendMessage(chatID, "Могу только по губам поводить. Приходи позже...", bot, update.Message.MessageID)
		return
	}

	// Проводим ролл на пропуск выбора гигачада дня
	if app.SpinSkipAction() {
		if err := app.UpdateGigaLastUpdate(db, chatID); err != nil {
			log.Printf("Error updating giga last update: %v", err)
		}
		message := messagegenerators.GetSkipGigaMessage()
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
	result := app.SpinAddPenSize(pen)

	// Выбираем случайную формулу: 0 - kinetic, 1 - potential, 2 - ohm
    formula := mathrand.Intn(3)
    var addSize int
	var formulaName string
	switch formula {
		case 0:
			// kineticEnergy: velocity от 0 до 5, среднее addSize ≈ 9
			velocity := result.Size % 6
			raw, err := kineticEnergy(velocity)
			if err != nil {
				log.Printf("Ошибка расчёта kineticEnergy: %v", err)
				addSize = 0
			} else {
				addSize = raw / 70 // нормализация к диапазону, чтобы среднее было ближе к 10
			}
			formulaName = "kineticEnergy"
		case 1:
			// potentialEnergy: height от 0 до 5, среднее addSize ≈ 10
			height := result.Size % 6
			raw, err := potentialEnergy(height)
			if err != nil {
				log.Printf("Ошибка расчёта potentialEnergy: %v", err)
				addSize = 0
			} else {
				addSize = raw / 330 // нормализация к диапазону, чтобы среднее было ближе к 10
			}
			formulaName = "potentialEnergy"
		case 2:
			// ohmLaw: resistance от 0 до 5 (0 заменяем на 1), среднее addSize ≈ 8
			resistance := result.Size % 6
			if resistance == 0 {
				resistance = 1 // чтобы избежать деления на 0
			}
			raw, err := ohmLaw(resistance)
			if err != nil {
				log.Printf("Ошибка расчёта ohmLaw: %v", err)
				addSize = 0
			} else {
				addSize = int(raw / 2) // нормализация к диапазону, чтобы среднее было ближе к 10
			}
			formulaName = "ohmLaw"
	}

	addSize = min(addSize, 15)
	addSize = max(addSize, 0)

	log.Printf("Calculated addSize: %d (formula #%d: %s)", addSize, formula, formulaName)
	
	newSize := pen.Size + addSize

	// Обновление значения члена и времени последнего обновления у выигравшего участника
	app.UpdateGiga(db, newSize, randomMember.ID, chatID)

	// Генерируем сообщени для чата
	message := messagegenerators.GetRandomGigaMessage(randomMember.Name, addSize, newSize)

	// Отправка сообщения с именем выбранного "красавчика"
	app.SendMessage(chatID, message, bot, update.Message.MessageID)
}
