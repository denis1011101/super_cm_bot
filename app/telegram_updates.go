package app

import (
	"context"
	"encoding/json"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramUpdate includes reaction updates absent from telegram-bot-api v5.5.1.
type TelegramUpdate struct {
	tgbotapi.Update
	MessageReaction      *TelegramMessageReaction      `json:"message_reaction"`
	MessageReactionCount *TelegramMessageReactionCount `json:"message_reaction_count"`
}

// PollTelegramUpdates streams the update types handled by the bot.
func PollTelegramUpdates(ctx context.Context, bot *tgbotapi.BotAPI) <-chan TelegramUpdate {
	updates := make(chan TelegramUpdate, bot.Buffer)
	config := tgbotapi.UpdateConfig{
		Timeout:        60,
		AllowedUpdates: []string{"message", "my_chat_member", "message_reaction", "message_reaction_count"},
	}
	go func() {
		defer close(updates)
		for ctx.Err() == nil {
			response, err := bot.Request(config)
			if ctx.Err() != nil {
				return
			}
			var batch []TelegramUpdate
			if err == nil {
				err = json.Unmarshal(response.Result, &batch)
			}
			if err != nil {
				log.Printf("Telegram getUpdates failed: %v", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
				continue
			}
			for _, update := range batch {
				if update.UpdateID < config.Offset {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case updates <- update:
					config.Offset = update.UpdateID + 1
				}
			}
		}
	}()
	return updates
}
