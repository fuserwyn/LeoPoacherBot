package bot

import (
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/domain"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// telegramUserLabel — стабильная подпись автора для памяти и промптов.
func telegramUserLabel(u *tgbotapi.User) string {
	if u == nil {
		return "unknown"
	}
	if u.UserName != "" {
		return "@" + u.UserName
	}
	name := strings.TrimSpace(strings.TrimSpace(u.FirstName + " " + u.LastName))
	if name != "" {
		return name
	}
	if u.IsBot {
		return "bot"
	}
	return fmt.Sprintf("user_%d", u.ID)
}

func formatChatMemoryLine(ts time.Time, userID int64, username, kind, text string) string {
	user := strings.TrimSpace(username)
	if user == "" {
		user = fmt.Sprintf("user_%d", userID)
	}
	if kind == "" {
		kind = "текст"
	}
	line := fmt.Sprintf("• [%s] %s (id=%d) [%s]: %s",
		ts.Format("2006-01-02 15:04"), user, userID, kind, strings.TrimSpace(text))
	if len(line) > 500 {
		return line[:500] + "…"
	}
	return line
}

func (b *Bot) largestPhotoFileID(msg *tgbotapi.Message) string {
	if msg == nil || len(msg.Photo) == 0 {
		return ""
	}
	best := msg.Photo[0]
	for _, p := range msg.Photo[1:] {
		if p.FileSize > best.FileSize {
			best = p
		}
	}
	return best.FileID
}

func (b *Bot) telegramFileURL(fileID string) (string, error) {
	if fileID == "" {
		return "", fmt.Errorf("empty file id")
	}
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return "", err
	}
	return file.Link(b.api.Token), nil
}

func (b *Bot) collectPhotoURLs(msgs ...*tgbotapi.Message) []string {
	var urls []string
	seen := make(map[string]struct{})
	for _, m := range msgs {
		if m == nil {
			continue
		}
		fid := b.largestPhotoFileID(m)
		if fid == "" {
			continue
		}
		if _, ok := seen[fid]; ok {
			continue
		}
		url, err := b.telegramFileURL(fid)
		if err != nil {
			b.logger.Warnf("photo file url: %v", err)
			continue
		}
		seen[fid] = struct{}{}
		urls = append(urls, url)
	}
	return urls
}

func buildPhotoMemoryText(userID int64, username, caption, description string) string {
	var b strings.Builder
	b.WriteString("[ФОТО] ")
	b.WriteString(strings.TrimSpace(username))
	b.WriteString(fmt.Sprintf(" (id=%d)", userID))
	if strings.TrimSpace(caption) != "" {
		b.WriteString(" | подпись: ")
		b.WriteString(strings.TrimSpace(caption))
	}
	if strings.TrimSpace(description) != "" {
		b.WriteString(" | ")
		b.WriteString(strings.TrimSpace(description))
	}
	return b.String()
}

func (b *Bot) appendReplyThreadContext(ctx *strings.Builder, msg *tgbotapi.Message) {
	if msg == nil || msg.ReplyToMessage == nil {
		return
	}
	r := msg.ReplyToMessage
	author := "неизвестно"
	authorID := int64(0)
	if r.From != nil {
		author = telegramUserLabel(r.From)
		authorID = r.From.ID
	}
	text := strings.TrimSpace(r.Text)
	if text == "" {
		text = strings.TrimSpace(r.Caption)
	}
	kind := "текст"
	if len(r.Photo) > 0 {
		kind = "фото"
		if text == "" {
			text = "(фото без подписи)"
		}
	}
	if text == "" && kind != "фото" {
		return
	}
	ctx.WriteString("\n=== СООБЩЕНИЕ, НА КОТОРОЕ ОТВЕЧАЕТ ПОЛЬЗОВАТЕЛЬ ===\n")
	ctx.WriteString(formatChatMemoryLine(time.Unix(int64(r.Date), 0), authorID, author, kind, text))
	ctx.WriteString("\n")
}

func formatHistoryLine(m *domain.UserMessage) string {
	kind := "текст"
	if strings.HasPrefix(m.MessageText, "[ФОТО]") {
		kind = "фото"
	}
	typeTag := ""
	switch m.MessageType {
	case "training_done":
		typeTag = " [отчёт]"
	case "sick_leave":
		typeTag = " [больничный]"
	case "healthy":
		typeTag = " [выздоровление]"
	case "ai_reply":
		typeTag = " [бот]"
	case "question":
		typeTag = " [вопрос]"
	case "photo":
		kind = "фото"
	}
	line := formatChatMemoryLine(m.CreatedAt, m.UserID, m.Username, kind, m.MessageText)
	if typeTag != "" {
		line = strings.Replace(line, "]:", typeTag+"]:", 1)
	}
	return line
}
