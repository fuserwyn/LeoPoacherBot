package bot

import (
	"errors"
	"strings"
	"unicode/utf8"

	"leo-bot/internal/database"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// Допустимые реакции на отчёт в ленте мини-аппа (порядок — отображение).
var trainingFeedAllowedEmojis = []string{"🔥", "💪", "👏", "❤️", "🎉", "🦁", "⭐", "👍"}

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
	_, err = b.db.InsertTrainingFeedThreadReply(chatID, userMessageID, viewerUserID, uname, text)
	return err
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
			for _, t := range thr {
				items[i].Thread = append(items[i].Thread, PackFeedThreadReply{
					ID:        t.ID,
					UserID:    t.FromUserID,
					Username:  t.Username,
					Text:      t.MessageText,
					CreatedAt: t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
					IsYou:     t.FromUserID == viewerUserID,
				})
			}
		}
	}
	return items
}
