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

func (b *Bot) photoFileID(msg *tgbotapi.Message) string {
	if msg == nil {
		return ""
	}
	if len(msg.Photo) > 0 {
		best := msg.Photo[0]
		for _, p := range msg.Photo[1:] {
			if p.FileSize > best.FileSize {
				best = p
			}
		}
		return best.FileID
	}
	if msg.Document != nil {
		mime := strings.ToLower(msg.Document.MimeType)
		if strings.HasPrefix(mime, "image/") && msg.Document.FileID != "" {
			return msg.Document.FileID
		}
	}
	return ""
}

func (b *Bot) hasVisualAttachment(msg *tgbotapi.Message) bool {
	return b.photoFileID(msg) != ""
}

func (b *Bot) hasPhoto(msg *tgbotapi.Message) bool {
	return b.hasVisualAttachment(msg)
}

// indexPhotoMessageAsync: GPT-4o-mini vision → Postgres + Qdrant с автором отправителя.
func (b *Bot) indexPhotoMessageAsync(msg *tgbotapi.Message, messageType string) {
	if b.aiClient == nil || !b.hasVisualAttachment(msg) || msg.From == nil {
		return
	}
	go func() {
		url, err := b.telegramFileURL(b.photoFileID(msg))
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
		b.logger.Infof("Photo indexed to RAG: %s (id=%d) chat=%d", author, msg.From.ID, msg.Chat.ID)
	}()
}
