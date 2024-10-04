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
