package handlers

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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