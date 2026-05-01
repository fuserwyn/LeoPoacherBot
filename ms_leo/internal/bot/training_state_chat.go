package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// packTrainingStateChatID — chat_id строки training_state для этого апдейта.
// В личке с ботом Telegram даёт chat_id = user_id; для стаи храним состояние только на паре
// (user_id, MONETIZED_CHAT_ID), чтобы не было второй «фантомной» строки (user_id, user_id).
// В группах/супергруппах — реальный chat_id сообщения.
func (b *Bot) packTrainingStateChatID(msg *tgbotapi.Message) int64 {
	if b == nil || msg == nil || msg.Chat == nil {
		return 0
	}
	uid := int64(0)
	if msg.From != nil {
		uid = msg.From.ID
	}
	return b.packTrainingStateChatIDFromTelegram(uid, msg.Chat)
}

func (b *Bot) packTrainingStateChatIDFromTelegram(userID int64, chat *tgbotapi.Chat) int64 {
	if b == nil || chat == nil {
		return 0
	}
	if b.config != nil && b.config.MonetizedChatID != 0 && userID != 0 &&
		(chat.IsPrivate() || chat.ID == userID) {
		return b.config.MonetizedChatID
	}
	return chat.ID
}
