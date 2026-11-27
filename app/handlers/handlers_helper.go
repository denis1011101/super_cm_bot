package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/denis1011101/super_cm_bot/app"
	"github.com/denis1011101/super_cm_bot/app/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var runtimeConfig config.Config

// Guards the cooldown check, selection, and scheduler state transition.
var selectionMu sync.Mutex

type selectionCommand struct {
	name       string
	lastUpdate func(*sql.DB, int64) (time.Time, error)
	choose     func(*sql.DB, int64) (message string, completed bool)
}

func (c *selectionCommand) runManual(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB, scheduler *SelectionScheduler) {
	selectionMu.Lock()
	defer selectionMu.Unlock()

	completed, _ := c.run(update.Message, bot, db, false)
	if completed && scheduler != nil && update.Message.Chat.ID == scheduler.chatID && c == scheduler.next {
		scheduler.advance(time.Now())
		scheduler.wake()
	}
}

// run requires selectionMu to be held.
func (c *selectionCommand) run(message *tgbotapi.Message, bot *tgbotapi.BotAPI, db *sql.DB, automatic bool) (completed bool, retryAt time.Time) {
	update := tgbotapi.Update{Message: message}
	if !automatic && !ensureSelectionCaller(update, bot, db) {
		return false, time.Time{}
	}

	chatID := message.Chat.ID
	lastUpdate, err := c.lastUpdate(db, chatID)
	if err != nil {
		return false, time.Time{}
	}
	if checkIsSpinNotLegal(lastUpdate) {
		if automatic {
			return false, lastUpdate.Add(runtimeConfig.SpinCooldown())
		}
		app.SendMessage(chatID, "Могу только по губам поводить. Приходи позже...", bot, message.MessageID)
		return false, time.Time{}
	}

	if automatic && message.From != nil && !ensureSelectionCaller(update, bot, db) {
		return false, time.Time{}
	}
	reply, completed := c.choose(db, chatID)
	if reply != "" && (completed || !automatic) {
		app.SendMessage(chatID, reply, bot, message.MessageID)
	}
	return completed, time.Time{}
}

func ensureSelectionCaller(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) bool {
	userID := update.Message.From.ID
	exists, err := app.UserExists(db, userID, update.Message.Chat.ID)
	if err != nil {
		log.Printf("Error checking if user exists: %v", err)
		return false
	}
	if !exists {
		log.Printf("User not found in database, registering: %v", userID)
		registerBot(update, bot, db, true)
	}
	return true
}

// SetRuntimeConfig applies the validated configuration during application startup.
func SetRuntimeConfig(cfg config.Config) {
	runtimeConfig = cfg
}

func checkIsSpinNotLegal(lastUpdate time.Time) bool {
	if !lastUpdate.IsZero() {
		duration := time.Since(lastUpdate)
		cooldown := runtimeConfig.SpinCooldown()

		if duration < cooldown {
			log.Printf("Spin is not legal: less than %s since last update", cooldown)
			return true
		}
	}
	log.Println("Spin is legal")
	return false
}

func registerBot(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB, sendWelcomeMessage bool) {
	// Логика регистрации в боте
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	userName := update.Message.From.UserName

	// Подготовка запроса для вставки пользователя в базу данных
	stmt, err := db.Prepare(`
		INSERT INTO pens (pen_name, tg_pen_id, tg_chat_id, pen_length, handsome_count, unhandsome_count, pen_last_update_at, registered_at)
		VALUES (?, ?, ?, ?, 0, 0, NULL, ?)
	`)
	if err != nil {
		log.Printf("Error preparing insert statement: %v", err)
		return
	}
	defer func() {
		if closeErr := stmt.Close(); closeErr != nil {
			log.Printf("Error closing statement: %v", closeErr)
		}
	}()

	// Выполнение подготовленного запроса с параметрами и добавление 5 см к размеру пениса
	_, err = stmt.Exec(userName, userID, chatID, 5, time.Now())
	if err != nil {
		log.Printf("Error inserting user into database: %v", err)
		return
	}

	// Отправка ответного сообщения, если флаг установлен
	if sendWelcomeMessage {
		app.SendMessage(chatID, "Велком ту зе клаб, бади 😎🤝😎", bot, update.Message.MessageID)
	}

	fmt.Println("User registered in bot")
}

// isUserJoinEvent проверяет, является ли сообщение событием входа пользователя в чат
func isUserJoinEvent(update tgbotapi.Update) bool {
	return update.Message.NewChatMembers != nil
}

// isUserLeaveEvent проверяет, является ли сообщение событием выхода пользователя из чата
func isUserLeaveEvent(update tgbotapi.Update) bool {
	return update.Message.LeftChatMember != nil
}
