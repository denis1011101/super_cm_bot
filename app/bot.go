package app

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SendMessage отправляет сообщение в чат или как ответ на конкретное сообщение
func SendMessage(chatID int64, text string, bot *tgbotapi.BotAPI, replyToMessageID int) {
	msg := tgbotapi.NewMessage(chatID, text)
	if replyToMessageID != 0 {
		msg.ReplyToMessageID = replyToMessageID
	}
    if _, err := bot.Send(msg); err != nil {
        log.Println("Error sending message:", err)
    } else {
        log.Printf("Message sent to chat ID %d: %s", chatID, text)
    }
}

func SendReaction(chatId int64, emoji string, bot *tgbotapi.BotAPI, replyToMessageId int) {
	reaction := tgbotapi.NewReaction(chatId, replyToMessageId, emoji)
	if _, err := bot.Send(reaction); err != nil {
        log.Printf("Error sending reaction to chat %d: %v", chatId, err)
    }
}

// Инициализирует клиент апи для бота
func ConfigureBot(botToken string) (*tgbotapi.BotAPI, tgbotapi.UpdatesChannel) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	u.AllowedUpdates = append(u.AllowedUpdates, "message", "message_reaction")

	updates := bot.GetUpdatesChan(u)
	return bot, updates
}
