package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/denis1011101/super_cm_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	myFactsLimit             = 3
	maxMyFactsMessageRunes   = 4000
	noSavedFactsMessage      = "Пенис-ИИ пока ничего о тебе не запомнил."
	savedFactsMessageHeading = "Вот что пенис-ИИ запомнил о тебе:"
	// когда фактов больше, чем влезает в ответ, показываем случайные —
	// иначе непонятно, почему каждый раз разные
	sampledFactsMessageHeading = "Вот %d случайных факта из %d, что пенис-ИИ о тебе помнит:"
	singleFactMessageHeading   = "Вот один случайный факт из %d, что пенис-ИИ о тебе помнит:"
)

// ShowMyGeminiFacts sends the command author only the facts that Gemini saved
// about that author in the current chat.
func ShowMyGeminiFacts(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	if update.Message == nil || update.Message.Chat == nil || update.Message.From == nil {
		log.Printf("ShowMyGeminiFacts: update has no message, chat or author")
		return
	}

	message, err := buildMyFactsMessage(db, update.Message.Chat.ID, update.Message.From)
	if err != nil {
		log.Printf("ShowMyGeminiFacts: failed to load facts: %v", err)
		app.SendMessage(update.Message.Chat.ID, "Не получилось достать воспоминания пенис-ИИ. Попробуй позже.", bot, update.Message.MessageID)
		return
	}

	app.SendMessage(update.Message.Chat.ID, message, bot, update.Message.MessageID)
}

// ForgetMyGeminiFacts deletes facts about the command author in this chat.
func ForgetMyGeminiFacts(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	if update.Message == nil || update.Message.Chat == nil || update.Message.From == nil {
		log.Printf("ForgetMyGeminiFacts: update has no message, chat or author")
		return
	}

	deleted, err := deleteMyFacts(db, update.Message.Chat.ID, update.Message.From)
	if err != nil {
		log.Printf("ForgetMyGeminiFacts: failed to delete facts: %v", err)
		app.SendMessage(update.Message.Chat.ID, "Не получилось очистить факты. Попробуй позже.", bot, update.Message.MessageID)
		return
	}

	message := "Пенис-ИИ и так ничего о тебе не помнил."
	if deleted > 0 {
		message = fmt.Sprintf("Готово, пенис-ИИ забыл о тебе фактов: %d.", deleted)
	}
	app.SendMessage(update.Message.Chat.ID, message, bot, update.Message.MessageID)
}

func buildMyFactsMessage(db *sql.DB, chatID int64, user *tgbotapi.User) (string, error) {
	if user == nil {
		return "", errors.New("user is nil")
	}

	if claimed, err := app.ClaimOwnerlessGeminiUserFacts(db, chatID, user); err != nil {
		log.Printf("buildMyFactsMessage: claim ownerless facts: %v", err)
	} else if claimed > 0 {
		log.Printf("buildMyFactsMessage: claimed %d ownerless facts for user %d", claimed, user.ID)
	}

	facts, err := app.LoadGeminiUserFactsForUser(db, chatID, user.ID, myFactsLimit)
	if err != nil {
		return "", err
	}
	if len(facts) == 0 {
		return noSavedFactsMessage, nil
	}

	total, err := app.CountGeminiUserFactsForUser(db, chatID, user.ID)
	if err != nil {
		return "", err
	}

	heading := savedFactsMessageHeading
	switch {
	case total <= len(facts):
	case len(facts) == 1:
		heading = fmt.Sprintf(singleFactMessageHeading, total)
	default:
		heading = fmt.Sprintf(sampledFactsMessageHeading, len(facts), total)
	}

	var message strings.Builder
	message.WriteString(heading)
	messageRunes := utf8.RuneCountInString(heading)
	for _, fact := range facts {
		line := "\n• " + fact.Fact
		remaining := maxMyFactsMessageRunes - messageRunes
		if remaining <= 1 {
			break
		}

		lineRunes := []rune(line)
		if len(lineRunes) > remaining {
			message.WriteString(string(lineRunes[:remaining-1]))
			message.WriteRune('…')
			break
		}
		message.WriteString(line)
		messageRunes += len(lineRunes)
	}

	return message.String(), nil
}

func deleteMyFacts(db *sql.DB, chatID int64, user *tgbotapi.User) (int64, error) {
	if user == nil {
		return 0, errors.New("user is nil")
	}
	if _, err := app.ClaimOwnerlessGeminiUserFacts(db, chatID, user); err != nil {
		log.Printf("deleteMyFacts: claim ownerless facts: %v", err)
	}
	return app.DeleteGeminiUserFactsForUser(db, chatID, user.ID)
}
