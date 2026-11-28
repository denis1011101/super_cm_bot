package handlers

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	mathrand "math/rand"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	messagegenerators "github.com/denis1011101/super_cm_bot/app/handlers/message_generators"
	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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

func normalizeGigaAddSize(addSize int) int {
	addSize = min(addSize, 15)
	addSize = max(addSize, 1)
	return addSize
}

var gigaSelection = selectionCommand{name: "/giga", lastUpdate: app.GetGigaLastUpdateTime, choose: chooseGiga}

func ChooseGiga(update models.Update, bot *telegram.Bot, db *sql.DB) {
	gigaSelection.runManual(update, bot, db, nil)
}

func chooseGiga(db *sql.DB, chatID int64) (string, bool) {
	now := time.Now()
	isHoliday := (now.Month() == time.December && now.Day() >= 24) || (now.Month() == time.January && now.Day() <= 2)

	// Проводим ролл на пропуск выбора гигачада дня
	if app.SpinSkipAction() {
		if err := app.UpdateGigaLastUpdate(db, chatID); err != nil {
			log.Printf("Error updating giga last update: %v", err)
			return "", false
		}
		return messagegenerators.GetSkipGigaMessage(isHoliday), true
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

	addSize = normalizeGigaAddSize(addSize)

	// Holiday multiplier: в период 24 Dec..31 Dec и 1..2 Jan в 2/3 случаев умножаем addSize на случайный 1..5
	if isHoliday {
        pos := mathrand.Intn(3) // 0 = no multiplier (1/3), 1..2 = apply multiplier (2/3)
        if pos != 0 {
            mul := mathrand.Intn(5) + 1 // 1..5
            addSize = addSize * mul
            log.Printf("Holiday multiplier applied: x%d", mul)
        }
    }

    log.Printf("Calculated addSize: %d (formula #%d: %s)", addSize, formula, formulaName)
	
	newSize := pen.Size + addSize

	// Обновление значения члена и времени последнего обновления у выигравшего участника
	if err := app.UpdateGiga(db, newSize, randomMember.ID, chatID); err != nil {
		log.Printf("Error updating giga in database: %v", err)
		return "", false
	}

	return messagegenerators.GetRandomGigaMessage(randomMember.Name, addSize, newSize, isHoliday), true
}
