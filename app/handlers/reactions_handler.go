package handlers

import (
	"database/sql"

	"github.com/denis1011101/super_cm_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleReaction(update *tgbotapi.MessageReactionUpdated, bot *tgbotapi.BotAPI, db *sql.DB) {
	if (len(update.Reactions) > 0) {
		sendedEmoji := update.Reactions[0].Emoji
		replyMessage := emojiToReply[sendedEmoji]
		if replyMessage == "" { return }
		app.SendMessage(update.Chat.ID, replyMessage, bot, update.MessageID)
	}
}

var emojiToReply = map[string]string{
	"🤡": "Сосал?🤡",
	"🤔": "А че задумался, сладкий?",
	"💩": "Я тоже копро люблю❤‍🔥",
	"🗿": "🗿",
	"😢": "Хули грустный - хуй сосал невкусный?",
}
