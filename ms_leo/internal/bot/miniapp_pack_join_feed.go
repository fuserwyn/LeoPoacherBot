package bot

import (
	"fmt"
	"strings"

	"leo-bot/internal/domain"
)

const (
	userMessageTypePackJoin   = "pack_join"
	userMessageTypePackRejoin = "pack_rejoin"
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
