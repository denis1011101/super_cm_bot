package handlers

import (
	"database/sql"
	"log"

	"github.com/denis1011101/super_cm_bot/app"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleReaction(update *tgbotapi.MessageReactionUpdated, bot *tgbotapi.BotAPI, db *sql.DB) {
	if len(update.Reactions) > 0 {
		alreadyReply, err := app.ReactionReplyExists(db, update.User.ID, update.Chat.ID, update.MessageID)
		if err != nil {
			log.Printf("Error checking if reaction reply exists: %v", err)
			return
		}

		if alreadyReply {
			return
		}

		sentEmoji := update.Reactions[0].Emoji
		replyMessage := emojiToReply[sentEmoji]
		if replyMessage == "" {
			return
		}
		app.SendMessage(update.Chat.ID, replyMessage, bot, update.MessageID)
		app.MarkMessageAsReply(db, update.User.ID, update.Chat.ID, update.MessageID, ReplyTypeMessage)
	}
}

var emojiToReply = map[string]string{
	"🤡": "Сосал?🤡",
	"🤔": "А че задумался, сладкий?",
	"💩": "Я тоже копро люблю❤‍🔥",
	"🗿": "🗿",
	"😢": "Хули грустный - хуй сосал невкусный?",
}

const (
	ReplyTypeEmoji   string = "emoji"
	ReplyTypeMessage string = "message"
)
