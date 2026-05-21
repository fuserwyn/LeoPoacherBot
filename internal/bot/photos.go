package bot

import (
	"strings"

	"leo-bot/internal/domain"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) messageCaptionOrText(msg *tgbotapi.Message) string {
	if msg == nil {
		return ""
	}
	if t := strings.TrimSpace(msg.Text); t != "" {
		return t
	}
	return strings.TrimSpace(msg.Caption)
}

func (b *Bot) hasPhoto(msg *tgbotapi.Message) bool {
	return msg != nil && len(msg.Photo) > 0
}

// indexPhotoMessageAsync описывает фото и сохраняет в RAG с указанием автора.
func (b *Bot) indexPhotoMessageAsync(msg *tgbotapi.Message, messageType string) {
	if b.aiClient == nil || !b.hasPhoto(msg) || msg.From == nil {
		return
	}
	go func() {
		url, err := b.telegramFileURL(b.largestPhotoFileID(msg))
		if err != nil {
			b.logger.Warnf("index photo: %v", err)
			return
		}
		author := telegramUserLabel(msg.From)
		caption := b.messageCaptionOrText(msg)
		desc, err := b.aiClient.DescribeImage(url, caption, author, b.config.OpenRouterVisionModel)
		if err != nil {
			b.logger.Warnf("describe photo for memory: %v", err)
			if caption == "" {
				desc = "(не удалось распознать фото)"
			} else {
				desc = ""
			}
		}
		text := buildPhotoMemoryText(msg.From.ID, author, caption, desc)
		b.persistChatMessage(&domain.UserMessage{
			UserID:      msg.From.ID,
			ChatID:      msg.Chat.ID,
			Username:    author,
			MessageText: text,
			MessageType: messageType,
		})
	}()
}
