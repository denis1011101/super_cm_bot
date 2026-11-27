package app

import (
	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// TelegramPollingOptions keeps updates in order so Gemini sees earlier reactions.
func TelegramPollingOptions() []telegram.Option {
	return []telegram.Option{
		telegram.WithAllowedUpdates(telegram.AllowedUpdates{
			models.AllowedUpdateMessage,
			models.AllowedUpdateMyChatMember,
			models.AllowedUpdateMessageReaction,
			models.AllowedUpdateMessageReactionCount,
		}),
		telegram.WithWorkers(1),
		telegram.WithNotAsyncHandlers(),
	}
}

// TelegramMemberUser returns the user stored in the active membership variant.
func TelegramMemberUser(member models.ChatMember) *models.User {
	switch {
	case member.Owner != nil:
		return member.Owner.User
	case member.Administrator != nil:
		return &member.Administrator.User
	case member.Member != nil:
		return member.Member.User
	case member.Restricted != nil:
		return member.Restricted.User
	case member.Left != nil:
		return member.Left.User
	case member.Banned != nil:
		return member.Banned.User
	default:
		return nil
	}
}
