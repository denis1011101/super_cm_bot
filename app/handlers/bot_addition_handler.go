package handlers

import (
	"log"

	"github.com/denis1011101/super_cm_bot/app"
	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func HandleBotAddition(update models.Update, bot *telegram.Bot) {
	if memberUpdate := update.MyChatMember; memberUpdate != nil && bot != nil {
		if user := app.TelegramMemberUser(memberUpdate.NewChatMember); user != nil && user.ID == bot.ID() {
			switch memberUpdate.Chat.Type {
			case models.ChatTypeGroup, models.ChatTypeSupergroup:
				log.Printf("Bot added to group: %s", memberUpdate.Chat.Title)
				app.SendMessage(memberUpdate.Chat.ID, "Здарова! Я ваш новый папочка 😈 Жмякай на кнопку, если не ссылко: /pen", bot, 0)
			case models.ChatTypePrivate:
				log.Printf("Bot added to private chat with: %s", memberUpdate.From.Username)
				app.SendMessage(memberUpdate.Chat.ID, "Этот бот работает только в группах.", bot, 0)
			}
		}
	}

	if update.Message != nil && update.Message.GroupChatCreated {
		log.Printf("Создан новый групповой чат: %s", update.Message.Chat.Title)
		app.SendMessage(update.Message.Chat.ID, "Привет! Я ваш новый бот. Жмякай на кнопку, если не ссылко: /pen", bot, 0)
	}
}
