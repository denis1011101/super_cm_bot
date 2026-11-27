package app

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramMessageReaction struct {
	Chat        tgbotapi.Chat  `json:"chat"`
	MessageID   int            `json:"message_id"`
	User        *tgbotapi.User `json:"user"`
	ActorChat   *tgbotapi.Chat `json:"actor_chat"`
	Date        int64          `json:"date"`
	OldReaction []reactionType `json:"old_reaction"`
	NewReaction []reactionType `json:"new_reaction"`
}

type TelegramMessageReactionCount struct {
	Chat      tgbotapi.Chat `json:"chat"`
	MessageID int           `json:"message_id"`
	Date      int64         `json:"date"`
	Reactions []struct {
		Type       reactionType `json:"type"`
		TotalCount int          `json:"total_count"`
	} `json:"reactions"`
}

type reactionType struct {
	Type          string `json:"type"`
	Emoji         string `json:"emoji"`
	CustomEmojiID string `json:"custom_emoji_id"`
}

func (r reactionType) label() string {
	switch r.Type {
	case "emoji":
		return r.Emoji
	case "custom_emoji":
		return "кастомный эмодзи " + r.CustomEmojiID
	case "paid":
		return "⭐ (платная реакция)"
	default:
		return ""
	}
}

func reactionDifference(previous, current []reactionType) []string {
	var difference []string
	for _, reaction := range current {
		if label := reaction.label(); label != "" && !slices.Contains(previous, reaction) {
			difference = append(difference, label)
		}
	}
	return difference
}

// RecordTelegramReaction puts reactions to the bot's own messages in Gemini's short-term memory.
func RecordTelegramReaction(db *sql.DB, chatID int64, update TelegramUpdate) error {
	if change := update.MessageReaction; change != nil && change.Chat.ID == chatID {
		if change.User != nil && change.User.IsBot {
			return nil
		}
		var changes []string
		if added := reactionDifference(change.OldReaction, change.NewReaction); len(added) > 0 {
			changes = append(changes, "добавил "+strings.Join(added, ", "))
		}
		if removed := reactionDifference(change.NewReaction, change.OldReaction); len(removed) > 0 {
			changes = append(changes, "убрал "+strings.Join(removed, ", "))
		}
		if len(changes) == 0 {
			return nil
		}
		actor := "Анонимный участник"
		if change.User != nil {
			actor = memorySpeakerName(change.User)
		} else if change.ActorChat != nil {
			actor = normalizeMemoryRole(change.ActorChat.Title, actor)
		}
		text := fmt.Sprintf("[Реакция на сообщение бота #%d] %s", change.MessageID, strings.Join(changes, " и "))
		return saveTelegramReaction(db, chatID, change.MessageID, update.UpdateID, tgUserID(change.User), actor, text, change.Date)
	}
	if counts := update.MessageReactionCount; counts != nil && counts.Chat.ID == chatID {
		var labels []string
		for _, reaction := range counts.Reactions {
			if label := reaction.Type.label(); label != "" && reaction.TotalCount > 0 {
				labels = append(labels, fmt.Sprintf("%s × %d", label, reaction.TotalCount))
			}
		}
		description := "реакций больше нет"
		if len(labels) > 0 {
			description = strings.Join(labels, ", ")
		}
		text := fmt.Sprintf("[Анонимные реакции на сообщение бота #%d] %s", counts.MessageID, description)
		return saveTelegramReaction(db, chatID, counts.MessageID, update.UpdateID, 0, "Telegram", text, counts.Date)
	}
	return nil
}

func saveTelegramReaction(db *sql.DB, chatID int64, messageID, updateID int, userID int64, role, text string, date int64) error {
	_, err := db.Exec(`
		INSERT INTO gemini_memories (chat_id, user_id, role, content, created_at, telegram_message_id, telegram_update_id)
		SELECT chat_id, ?, ?, ?, ?, telegram_message_id, ?
		FROM gemini_memories
		WHERE chat_id = ? AND telegram_message_id = ? AND role = 'bot' AND user_id = 0 AND telegram_update_id IS NULL
		ON CONFLICT(telegram_update_id) DO NOTHING`,
		userID, role, text, time.Unix(date, 0).UTC().Format(sqliteTimestampLayout), updateID, chatID, messageID)
	return err
}
