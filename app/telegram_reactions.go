package app

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
)

func reactionLabel(reaction models.ReactionType) string {
	switch {
	case reaction.ReactionTypeEmoji != nil:
		return reaction.ReactionTypeEmoji.Emoji
	case reaction.ReactionTypeCustomEmoji != nil:
		return "кастомный эмодзи " + reaction.ReactionTypeCustomEmoji.CustomEmojiID
	case reaction.Type == models.ReactionTypeTypePaid:
		return "⭐ (платная реакция)"
	default:
		return ""
	}
}

func reactionDifference(previous, current []models.ReactionType) []string {
	var difference []string
	for _, reaction := range current {
		label := reactionLabel(reaction)
		if label != "" && !slices.ContainsFunc(previous, func(old models.ReactionType) bool {
			return reactionLabel(old) == label
		}) {
			difference = append(difference, label)
		}
	}
	return difference
}

// RecordTelegramReaction puts reactions to the bot's own messages in Gemini's short-term memory.
func RecordTelegramReaction(db *sql.DB, chatID int64, update models.Update) error {
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
		return saveTelegramReaction(db, chatID, change.MessageID, update.ID, tgUserID(change.User), actor, text, change.Date)
	}
	if counts := update.MessageReactionCount; counts != nil && counts.Chat.ID == chatID {
		var labels []string
		for _, reaction := range counts.Reactions {
			if label := reactionLabel(reaction.Type); label != "" && reaction.TotalCount > 0 {
				labels = append(labels, fmt.Sprintf("%s × %d", label, reaction.TotalCount))
			}
		}
		description := "реакций больше нет"
		if len(labels) > 0 {
			description = strings.Join(labels, ", ")
		}
		text := fmt.Sprintf("[Анонимные реакции на сообщение бота #%d] %s", counts.MessageID, description)
		return saveTelegramReaction(db, chatID, counts.MessageID, update.ID, 0, "Telegram", text, counts.Date)
	}
	return nil
}

func saveTelegramReaction(db *sql.DB, chatID int64, messageID int, updateID, userID int64, role, text string, date int) error {
	_, err := db.Exec(`
		INSERT INTO gemini_memories (chat_id, user_id, role, content, created_at, telegram_message_id, telegram_update_id)
		SELECT chat_id, ?, ?, ?, ?, telegram_message_id, ?
		FROM gemini_memories
		WHERE chat_id = ? AND telegram_message_id = ? AND role = 'bot' AND user_id = 0 AND telegram_update_id IS NULL
		ON CONFLICT(telegram_update_id) DO NOTHING`,
		userID, role, text, time.Unix(int64(date), 0).UTC().Format(sqliteTimestampLayout), updateID, chatID, messageID)
	return err
}
