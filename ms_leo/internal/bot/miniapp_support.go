package bot

import (
	"fmt"
	"strings"

	"leo-bot/internal/domain"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MiniappSupportChatHistory — отдельный чат пользователя с поддержкой.
func (b *Bot) MiniappSupportChatHistory(userID int64, sinceID int64) ([]*domain.MiniappSupportChatMessage, error) {
	if b == nil || b.db == nil || userID == 0 {
		return []*domain.MiniappSupportChatMessage{}, nil
	}
	if b.config == nil || b.config.MonetizedChatID == 0 {
		return []*domain.MiniappSupportChatMessage{}, nil
	}
	return b.db.ListMiniappSupportChat(userID, b.config.MonetizedChatID, 200, sinceID)
}

// MiniappSupportSendFromUser — пользователь пишет в поддержку, не в Лео.
func (b *Bot) MiniappSupportSendFromUser(userID int64, text string) error {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 || userID == 0 {
		return nil
	}
	t := strings.TrimSpace(text)
	if t == "" {
		return nil
	}
	if _, err := b.db.InsertMiniappSupportChatMessage(userID, b.config.MonetizedChatID, "user", t); err != nil {
		return err
	}
	b.notifyAdminsAboutSupportMessage(userID, t)
	return nil
}

// AdminSupportChatHistory — история отдельного support-чата юзера для админа.
func (b *Bot) AdminSupportChatHistory(userID int64) ([]*domain.MiniappSupportChatMessage, error) {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 || userID == 0 {
		return []*domain.MiniappSupportChatMessage{}, nil
	}
	return b.db.ListMiniappSupportChat(userID, b.config.MonetizedChatID, 30, 0)
}

// AdminSupportReply — ответ админа пользователю в отдельный support-чат.
func (b *Bot) AdminSupportReply(userID int64, text string) error {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 || userID == 0 {
		return nil
	}
	t := strings.TrimSpace(text)
	if t == "" {
		return nil
	}
	if _, err := b.db.InsertMiniappSupportChatMessage(userID, b.config.MonetizedChatID, "support", t); err != nil {
		return err
	}
	if b.api != nil {
		if _, err := b.api.Send(tgbotapi.NewMessage(userID, t)); err != nil {
			b.logger.Warnf("admin support dm user=%d: %v", userID, err)
		}
	}
	return nil
}

func (b *Bot) notifyAdminsAboutSupportMessage(userID int64, text string) {
	if b == nil || b.api == nil || b.config == nil {
		return
	}
	adminIDs := make([]int64, 0, len(b.config.AdminIDs)+1)
	seen := map[int64]struct{}{}
	if b.config.OwnerID != 0 {
		seen[b.config.OwnerID] = struct{}{}
		adminIDs = append(adminIDs, b.config.OwnerID)
	}
	for _, id := range b.config.AdminIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		adminIDs = append(adminIDs, id)
	}
	if len(adminIDs) == 0 {
		return
	}

	title := fmt.Sprintf("Новый запрос в поддержку от %s", b.supportDisplayName(userID))
	body := clipAdminSupportText(text, 500)
	for _, adminID := range adminIDs {
		msg := tgbotapi.NewMessage(adminID, title+"\n\n"+body+"\n\nНажми «⚙️ Админ-панель» внизу → Поддержка.")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✍️ Ответить", fmt.Sprintf("admin_support_reply_%d", userID)),
			),
		)
		if _, err := b.api.Send(msg); err != nil {
			b.logger.Warnf("support notify admin=%d user=%d: %v", adminID, userID, err)
		}
	}
}

func (b *Bot) supportDisplayName(userID int64) string {
	if b == nil || b.db == nil || b.config == nil || b.config.MonetizedChatID == 0 || userID == 0 {
		return fmt.Sprintf("user%d · %d", userID, userID)
	}
	if p, err := b.db.GetMiniappUserProfile(userID, b.config.MonetizedChatID); err == nil && p != nil {
		if dn := strings.TrimSpace(p.DisplayName); dn != "" {
			return fmt.Sprintf("%s · %d", dn, userID)
		}
	}
	if ml, err := b.db.GetMessageLogAnyState(userID, b.config.MonetizedChatID); err == nil && ml != nil {
		if un := strings.TrimSpace(ml.Username); un != "" {
			return fmt.Sprintf("%s · %d", un, userID)
		}
	}
	return fmt.Sprintf("user%d · %d", userID, userID)
}
