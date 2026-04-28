package bot

import (
	"fmt"
	"strings"

	"leo-bot/internal/domain"
)

const (
	userMessageTypePackJoin     = "pack_join"
	userMessageTypePackRejoin   = "pack_rejoin"
	userMessageTypeDailyWisdom  = "daily_wisdom"
	userMessageTypePackRemoved  = "pack_removed"
)

func packJoinMiniappWelcomeText(displayName string) string {
	d := strings.TrimSpace(displayName)
	if d == "" {
		d = "участник"
	}
	return fmt.Sprintf(
		"Привет, %s! Я Лео — добро пожаловать в стаю. Отмечай тренировки тегом #training_done и следи за дедлайном. Рад видеть тебя здесь! 🐆",
		d,
	)
}

func packRejoinMiniappWelcomeText(displayName string) string {
	d := strings.TrimSpace(displayName)
	if d == "" {
		d = "участник"
	}
	return fmt.Sprintf(
		"%s, с возвращением! Я Лео — продолжай отмечать тренировки через #training_done. Рад снова видеть тебя в стае! 🐆",
		d,
	)
}

// savePackJoinMiniappFeed — строка ленты мини-аппа: короткое приветствие от Лео с именем вступившего (user_id / username — новичок).
func (b *Bot) savePackJoinMiniappFeed(chatID, userID int64, username, msgType, leoText string) {
	if b == nil || b.db == nil || userID == 0 {
		return
	}
	if b.config.MonetizedChatID == 0 || chatID != b.config.MonetizedChatID {
		return
	}
	um := &domain.UserMessage{
		UserID:      userID,
		ChatID:      chatID,
		Username:    strings.TrimSpace(username),
		MessageText: leoText,
		MessageType: msgType,
	}
	if err := b.db.SaveUserMessage(um); err != nil {
		b.logger.Warnf("miniapp pack feed %s user=%d: %v", msgType, userID, err)
	}
}

// saveDailyWisdomPackFeed — «мудрость дня» в ленту мини-аппа (одна запись в чате стаи, автор Лео).
func (b *Bot) saveDailyWisdomPackFeed(wisdom string) {
	if b == nil || b.db == nil {
		return
	}
	chatID := b.config.MonetizedChatID
	if chatID == 0 {
		return
	}
	t := strings.TrimSpace(wisdom)
	if t == "" {
		return
	}
	um := &domain.UserMessage{
		UserID:      0,
		ChatID:      chatID,
		Username:    "Лео",
		MessageText: t,
		MessageType: userMessageTypeDailyWisdom,
	}
	if err := b.db.SaveUserMessage(um); err != nil {
		b.logger.Warnf("miniapp pack feed daily_wisdom: %v", err)
	}
}

func (b *Bot) savePackRemovedMiniappFeed(chatID, userID int64, username string) {
	if b == nil || b.db == nil || userID == 0 {
		return
	}
	if b.config.MonetizedChatID == 0 || chatID != b.config.MonetizedChatID {
		return
	}
	um := &domain.UserMessage{
		UserID:      userID,
		ChatID:      chatID,
		Username:    strings.TrimSpace(username),
		MessageText: PackRemovedFeedNotice(b.config.Prompts, username),
		MessageType: userMessageTypePackRemoved,
	}
	if err := b.db.SaveUserMessage(um); err != nil {
		b.logger.Warnf("miniapp pack feed pack_removed user=%d: %v", userID, err)
	}
}
