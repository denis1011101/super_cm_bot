package handlers

import (
	"database/sql"

	"github.com/denis1011101/super_cm_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleReaction(update tgbotapi.Update, bot *tgbotapi.BotAPI, db *sql.DB) {
	if (len(update.MessageReaction.Reactions) > 0) {
		sendedEmoji := update.MessageReaction.Reactions[0].Emoji
		replyMessage := emojiToReply[sendedEmoji]
		if replyMessage == "" { return }
		app.SendMessage(update.MessageReaction.Chat.ID, replyMessage, bot, update.MessageReaction.MessageID)
	}
}

var emojiToReply = map[string]string{
	"🤡": "Сосал?🤡",
	"🤔": "А че задумался, сладкий?",
	"💩": "Я тоже копро люблю❤‍🔥",
	"🗿": "🗿",
	"😢": "Хули грустный - хуй сосал невкусный?",
}