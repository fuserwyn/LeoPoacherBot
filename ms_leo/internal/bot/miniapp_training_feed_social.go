package bot

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"leo-bot/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// Допустимые реакции на отчёт в ленте мини-аппа (порядок — отображение).
var trainingFeedAllowedEmojis = []string{"🔥", "💪", "👏", "❤️", "🎉", "🦁", "⭐", "👍", "🙌", "✨", "🤝", "⚡", "🎯", "😤", "👀", "🙏", "😱"}

var (
	// ErrTrainingFeedSocialForbidden — нет доступа к ленте.
	ErrTrainingFeedSocialForbidden = errors.New("training feed social forbidden")
	// ErrTrainingFeedInvalidEmoji — эмодзи не из списка.
	ErrTrainingFeedInvalidEmoji = errors.New("invalid emoji")
	// ErrTrainingFeedParentNotFound — нет такого отчёта в стае.
	ErrTrainingFeedParentNotFound = errors.New("parent message not found")
	// ErrTrainingFeedThreadEmpty — пустой текст треда.
	ErrTrainingFeedThreadEmpty = errors.New("thread text empty")
	// ErrTrainingFeedThreadTooLong — слишком длинный комментарий.
	ErrTrainingFeedThreadTooLong = errors.New("thread text too long")
	// ErrTrainingFeedThreadDeleteNotFound — не найден комментарий или чужой / Лео.
	ErrTrainingFeedThreadDeleteNotFound = errors.New("training thread reply not found or forbidden")
)

func allowedTrainingFeedEmoji(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, e := range trainingFeedAllowedEmojis {
		if s == e {
			return e, true
		}
	}
	return "", false
}

func (b *Bot) assertPackFeedSocialViewer(viewerUserID int64) error {
	if b == nil {
		return ErrTrainingFeedSocialForbidden
	}
	chatID := b.config.MonetizedChatID
	if chatID == 0 {
		return ErrTrainingFeedSocialForbidden
	}
	if b.config.OwnerID != 0 && viewerUserID == b.config.OwnerID {
		return nil
	}
	ok, err := b.db.UserInPackOrPaid(viewerUserID, chatID, b.config.PaywallEnabled)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTrainingFeedSocialForbidden
	}
	return nil
}

// PackTrainingFeedReact — реакция на training_done (повтор с той же эмодзи снимает).
func (b *Bot) PackTrainingFeedReact(viewerUserID int64, initD initdata.InitData, userMessageID int64, emoji string) error {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return err
	}
	em, ok := allowedTrainingFeedEmoji(emoji)
	if !ok {
		return ErrTrainingFeedInvalidEmoji
	}
	chatID := b.config.MonetizedChatID
	typ, has, err := b.db.GetUserMessageTypeByIDForChat(userMessageID, chatID)
	if err != nil {
		return err
	}
	if !has || typ != "training_done" {
		return ErrTrainingFeedParentNotFound
	}
	uname := displayNameFromInitData(initD)
	return b.db.SetTrainingFeedReaction(chatID, userMessageID, viewerUserID, uname, em)
}

// PackTrainingFeedThreadPost — комментарий в треде под training_done.
func (b *Bot) PackTrainingFeedThreadPost(viewerUserID int64, initD initdata.InitData, userMessageID int64, text string) error {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ErrTrainingFeedThreadEmpty
	}
	if utf8.RuneCountInString(text) > 2000 {
		return ErrTrainingFeedThreadTooLong
	}
	chatID := b.config.MonetizedChatID
	typ, has, err := b.db.GetUserMessageTypeByIDForChat(userMessageID, chatID)
	if err != nil {
		return err
	}
	if !has || typ != "training_done" {
		return ErrTrainingFeedParentNotFound
	}
	uname := displayNameFromInitData(initD)
	threadID, err := b.db.InsertTrainingFeedThreadReply(chatID, userMessageID, viewerUserID, uname, text)
	if err != nil {
		return err
	}
	b.afterPackTrainingThreadInserted(chatID, userMessageID, viewerUserID, uname, text, threadID)
	return nil
}

func truncateForDM(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maxRunes {
		return string(r)
	}
	return string(r[:maxRunes]) + "…"
}

// Уведомление в личку Telegram автору отчёта + строка для бейджа «Стая» в мини-аппе.
func (b *Bot) afterPackTrainingThreadInserted(packChatID, userMessageID, commenterUserID int64, commenterName, commentText string, threadReplyID int64) {
	if b == nil || b.db == nil {
		return
	}
	authorID, ok, err := b.db.GetUserMessageAuthorUserID(packChatID, userMessageID)
	if err != nil {
		b.logger.Warnf("training thread author lookup: %v", err)
		return
	}
	if !ok || authorID == 0 || authorID == commenterUserID {
		return
	}
	if err := b.db.InsertTrainingThreadUnread(authorID, packChatID, threadReplyID); err != nil {
		b.logger.Warnf("training thread unread insert: %v", err)
	}
	preview := truncateForDM(commentText, 160)
	cn := strings.TrimSpace(commenterName)
	if cn == "" {
		cn = "Участник стаи"
	}
	body := "💬 " + cn + " прокомментировал(а) твою тренировку в стае.\n\n«" + preview + "»\n\nОткрой мини-апп → вкладка «Стая»."
	b.sendTrainingThreadCommentDM(authorID, body)
}

// Всегда в Telegram-личку, без очереди мини-аппа (уведомления-алерт).
func (b *Bot) sendTrainingThreadCommentDM(telegramUserID int64, text string) {
	if b == nil || b.api == nil || telegramUserID == 0 || strings.TrimSpace(text) == "" {
		return
	}
	m := tgbotapi.NewMessage(telegramUserID, text)
	if _, err := b.api.Send(m); err != nil {
		b.logger.Warnf("training thread comment DM user=%d: %v", telegramUserID, err)
	}
}

// MiniappTrainingThreadUnreadCount — для бейджа на вкладке «Стая».
func (b *Bot) MiniappTrainingThreadUnreadCount(initD initdata.InitData, viewerUserID int64) (int64, error) {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return 0, err
	}
	chatID := b.config.MonetizedChatID
	if chatID == 0 || b.db == nil {
		return 0, nil
	}
	return b.db.CountTrainingThreadUnread(viewerUserID, chatID)
}

// MiniappTrainingThreadUnreadClear — сброс бейджа при открытии ленты.
func (b *Bot) MiniappTrainingThreadUnreadClear(initD initdata.InitData, viewerUserID int64) error {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return err
	}
	chatID := b.config.MonetizedChatID
	if chatID == 0 || b.db == nil {
		return nil
	}
	return b.db.ClearTrainingThreadUnread(viewerUserID, chatID)
}

// PackTrainingFeedThreadDelete — удалить свою реплику в треде (ответы Лео не удаляются).
func (b *Bot) PackTrainingFeedThreadDelete(viewerUserID int64, initD initdata.InitData, threadReplyID int64) (parentUserMessageID int64, err error) {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return 0, err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return 0, err
	}
	if threadReplyID == 0 {
		return 0, ErrTrainingFeedThreadDeleteNotFound
	}
	chatID := b.config.MonetizedChatID
	parentID, deleted, err := b.db.DeleteTrainingFeedThreadReply(chatID, threadReplyID, viewerUserID)
	if err != nil {
		return 0, err
	}
	if !deleted {
		return 0, ErrTrainingFeedThreadDeleteNotFound
	}
	if err := b.db.DeleteTrainingThreadUnreadByReplyID(threadReplyID); err != nil {
		b.logger.Warnf("training thread unread delete: %v", err)
	}
	return parentID, nil
}

func threadDBRowsToPackReplies(rows []database.TrainingFeedThreadRow, viewerUserID int64) []PackFeedThreadReply {
	out := make([]PackFeedThreadReply, 0, len(rows))
	for _, t := range rows {
		out = append(out, PackFeedThreadReply{
			ID:        t.ID,
			UserID:    t.FromUserID,
			Username:  t.Username,
			Text:      t.MessageText,
			CreatedAt: t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			IsYou:     t.FromUserID != 0 && t.FromUserID == viewerUserID,
			IsLeo:     t.FromUserID == 0,
		})
	}
	return out
}

// PackFeedThreadRepliesForViewer — полный тред под одним отчётом (после POST комментария и для согласованности с лентой).
func (b *Bot) PackFeedThreadRepliesForViewer(viewerUserID, userMessageID int64) ([]PackFeedThreadReply, error) {
	if b == nil || b.db == nil {
		return nil, fmt.Errorf("bot unavailable")
	}
	m, err := b.db.ListTrainingFeedThreadByMessages([]int64{userMessageID})
	if err != nil {
		return nil, err
	}
	return threadDBRowsToPackReplies(m[userMessageID], viewerUserID), nil
}

// enrichPackFeedTrainingSocial — реакции и треды для карточек training_done.
func (b *Bot) enrichPackFeedTrainingSocial(items []PackFeedItem, viewerUserID int64, chatID int64) []PackFeedItem {
	trainingIDs := make([]int64, 0)
	for _, it := range items {
		if it.Type == "training_done" {
			trainingIDs = append(trainingIDs, it.ID)
		}
	}
	if len(trainingIDs) == 0 {
		return items
	}
	aggsMap, meMap, err := b.db.ListTrainingFeedReactionAggs(chatID, trainingIDs, viewerUserID)
	if err != nil {
		b.logger.Warnf("pack feed reaction aggs: %v", err)
		return items
	}
	threadMap, err := b.db.ListTrainingFeedThreadByMessages(trainingIDs)
	if err != nil {
		b.logger.Warnf("pack feed thread list: %v", err)
		return items
	}
	for i := range items {
		if items[i].Type != "training_done" {
			continue
		}
		id := items[i].ID
		meEmoji := meMap[id]
		if aggs, ok := aggsMap[id]; ok {
			for _, a := range database.SortReactionAggsForDisplay(aggs, trainingFeedAllowedEmojis) {
				items[i].Reactions = append(items[i].Reactions, PackFeedReaction{
					Emoji: a.Emoji,
					Count: a.Count,
					Me:    meEmoji == a.Emoji,
				})
			}
		}
		if thr, ok := threadMap[id]; ok {
			items[i].Thread = threadDBRowsToPackReplies(thr, viewerUserID)
		}
	}
	return items
}
