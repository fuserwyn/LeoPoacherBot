package bot

import (
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/domain"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const recentPhotoMaxPerChat = 30
const recentPhotoLookback = 15 * time.Minute

type chatPhotoRef struct {
	FileID    string
	MessageID int
	UserID    int64
	Username  string
	At        time.Time
}

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

func questionRefersToPhoto(question string) bool {
	q := strings.ToLower(strings.TrimSpace(question))
	markers := []string{
		"на фото", "на картин", "про фото", "это фото", "прислан",
		"изображен", "что на фото", "что там", "опиши фото", "что за фото",
		"видишь фото", "на снимк", "на картинке", "что на картинке",
		"что на изображ", "расскажи про фото", "опиши картин",
	}
	for _, m := range markers {
		if strings.Contains(q, m) {
			return true
		}
	}
	return false
}

// recordChatPhoto сохраняет file_id сразу при приёме — до async vision.
func (b *Bot) recordChatPhoto(msg *tgbotapi.Message) {
	if msg == nil || msg.From == nil {
		return
	}
	fid := b.photoFileID(msg)
	if fid == "" {
		return
	}
	ref := chatPhotoRef{
		FileID:    fid,
		MessageID: msg.MessageID,
		UserID:    msg.From.ID,
		Username:  telegramUserLabel(msg.From),
		At:        time.Now(),
	}
	b.recentPhotosMu.Lock()
	defer b.recentPhotosMu.Unlock()
	if b.recentPhotos == nil {
		b.recentPhotos = make(map[int64][]chatPhotoRef)
	}
	list := append(b.recentPhotos[msg.Chat.ID], ref)
	if len(list) > recentPhotoMaxPerChat {
		list = list[len(list)-recentPhotoMaxPerChat:]
	}
	b.recentPhotos[msg.Chat.ID] = list
}

func (b *Bot) recentPhotoFileIDs(msg *tgbotapi.Message, within time.Duration, preferUserID int64) []string {
	if msg == nil {
		return nil
	}
	b.recentPhotosMu.RLock()
	list := append([]chatPhotoRef(nil), b.recentPhotos[msg.Chat.ID]...)
	b.recentPhotosMu.RUnlock()
	if len(list) == 0 {
		return nil
	}
	cutoff := time.Now().Add(-within)
	var sameUser, anyUser []string
	seen := make(map[string]struct{})
	for i := len(list) - 1; i >= 0; i-- {
		p := list[i]
		if p.At.Before(cutoff) || p.FileID == "" {
			continue
		}
		if _, ok := seen[p.FileID]; ok {
			continue
		}
		seen[p.FileID] = struct{}{}
		if preferUserID != 0 && p.UserID == preferUserID {
			sameUser = append(sameUser, p.FileID)
		} else {
			anyUser = append(anyUser, p.FileID)
		}
	}
	if len(sameUser) > 0 {
		return sameUser[:1]
	}
	if len(anyUser) > 0 {
		return anyUser[:1]
	}
	return nil
}

func (b *Bot) fileIDsToURLs(fileIDs []string) []string {
	var urls []string
	seen := make(map[string]struct{})
	for _, fid := range fileIDs {
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

// resolvePhotoURLsForAI: фото в сообщении, реплае или недавнее в чате (отдельным сообщением).
func (b *Bot) resolvePhotoURLsForAI(msg *tgbotapi.Message, question string) (urls []string, fromRecent bool) {
	var msgs []*tgbotapi.Message
	if msg != nil {
		msgs = append(msgs, msg)
		if msg.ReplyToMessage != nil {
			msgs = append(msgs, msg.ReplyToMessage)
		}
	}
	urls = b.collectPhotoURLs(msgs...)
	if len(urls) > 0 {
		return urls, false
	}
	needRecent := questionRefersToPhoto(question)
	if msg != nil && msg.ReplyToMessage != nil && b.hasVisualAttachment(msg.ReplyToMessage) {
		needRecent = true
	}
	if !needRecent || msg == nil {
		return nil, false
	}
	preferID := int64(0)
	if msg.From != nil {
		preferID = msg.From.ID
	}
	fids := b.recentPhotoFileIDs(msg, recentPhotoLookback, preferID)
	urls = b.fileIDsToURLs(fids)
	return urls, len(urls) > 0
}

// appendTrainingReportPhotoContext — фото к #training_done: нейтральное описание в контекст ИИ.
func (b *Bot) appendTrainingReportPhotoContext(ctx *strings.Builder, msg *tgbotapi.Message, username string) []string {
	if !b.hasVisualAttachment(msg) {
		return nil
	}
	urls := b.collectPhotoURLs(msg)
	if len(urls) == 0 {
		return nil
	}
	ctx.WriteString("К отчёту приложено фото.\n")
	if b.aiClient != nil {
		caption := strings.ReplaceAll(b.messageCaptionOrText(msg), "#training_done", "")
		caption = strings.ReplaceAll(caption, "#writing_done", "")
		caption = strings.TrimSpace(caption)
		if desc, err := b.aiClient.DescribeImageForTrainingReport(urls[0], caption, username, b.config.OpenRouterVisionModel); err == nil {
			desc = strings.TrimSpace(desc)
			if desc != "" {
				ctx.WriteString(fmt.Sprintf("На фото: %s\n", desc))
			}
		}
	}
	return urls
}

func trainingPromptPhotoSuffix(hasPhoto bool, chatType string) string {
	if !hasPhoto {
		return ""
	}
	if chatType == "writing" {
		return " К отчёту приложено фото: в конце добавь одно короткое сдержанное предложение про снимок (что видно), без восторга и восклицаний — спокойный нейтральный тон."
	}
	return " К отчёту приложено фото: в конце добавь одно короткое сдержанное предложение про снимок (что видно), без энтузиазма и пафоса — ровный тон тренера, как констатация факта."
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
