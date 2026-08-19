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
	noSavedFactsMessage      = "ИИ пока ничего о тебе не запомнил."
	savedFactsMessageHeading = "Вот что ИИ запомнил о тебе:"
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
		app.SendMessage(update.Message.Chat.ID, "Не получилось достать воспоминания ИИ. Попробуй позже.", bot, update.Message.MessageID)
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

	message := "ИИ и так ничего о тебе не помнил."
	if deleted > 0 {
		message = fmt.Sprintf("Готово, ИИ забыл о тебе фактов: %d.", deleted)
	}
	app.SendMessage(update.Message.Chat.ID, message, bot, update.Message.MessageID)
}

func buildMyFactsMessage(db *sql.DB, chatID int64, user *tgbotapi.User) (string, error) {
	if user == nil {
		return "", errors.New("user is nil")
	}

	facts, err := app.LoadGeminiUserFactsByNames(db, chatID, geminiUserAliases(user), myFactsLimit)
	if err != nil {
		return "", err
	}
	if len(facts) == 0 {
		return noSavedFactsMessage, nil
	}

	var message strings.Builder
	message.WriteString(savedFactsMessageHeading)
	messageRunes := utf8.RuneCountInString(savedFactsMessageHeading)
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
	return app.DeleteGeminiUserFactsByNames(db, chatID, geminiUserAliases(user))
}

func geminiUserAliases(user *tgbotapi.User) []string {
	aliases := []string{user.FirstName, user.UserName}
	if user.FirstName != "" && user.LastName != "" {
		aliases = append(aliases, user.FirstName+" "+user.LastName)
	}
	return aliases
}
