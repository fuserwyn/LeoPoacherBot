package bot

import (
	"errors"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// ErrPackFeedForbidden — смотрящему нельзя видеть ленту (нет в стае / не оплачено).
var ErrPackFeedForbidden = errors.New("pack feed forbidden")

// PackFeedReaction — агрегат реакций на отчёт в мини-аппе.
type PackFeedReaction struct {
	Emoji  string   `json:"emoji"`
	Count  int      `json:"count"`
	Me     bool     `json:"me"`
	Voters []string `json:"voters,omitempty"`
}

// PackFeedThreadReply — реплика в треде под training_done.
type PackFeedThreadReply struct {
	ID              int64  `json:"id"`
	UserID          int64  `json:"user_id"`
	Username        string `json:"username"`
	Text            string `json:"text"`
	CreatedAt       string `json:"created_at"`
	IsYou           bool   `json:"is_you"`
	IsLeo           bool   `json:"is_leo"`
	AuthorPhotoURL  string `json:"author_photo_url,omitempty"`
	ReplyToID       int64  `json:"reply_to_id,omitempty"`
	ReplyToUsername string `json:"reply_to_username,omitempty"`
	ReplyToText     string `json:"reply_to_text,omitempty"`
	ReplyToIsLeo    bool   `json:"reply_to_is_leo,omitempty"`
	LikeCount       int    `json:"like_count,omitempty"`
	LikeMe          bool   `json:"like_me,omitempty"`
}

// PackFeedItem — JSON для мини-апpa.
type PackFeedItem struct {
	ID               int64  `json:"id"`
	UserID           int64  `json:"user_id"`
	Username         string `json:"username"`
	Type             string `json:"type"`
	Text             string `json:"text"`
	CreatedAt        string `json:"created_at"`
	StreakDays       int    `json:"streak_days"`
	IsYou            bool   `json:"is_you"`
	AuthorPhotoURL   string `json:"author_photo_url,omitempty"`
	TrainingPhotoURL string `json:"training_photo_url,omitempty"`
	// PackChatID — id группы «Стая» (MONETIZED_CHAT_ID), с которой синхронизирована лента.
	PackChatID int64                 `json:"pack_chat_id"`
	PackTitle  string                `json:"pack_title,omitempty"`
	Reactions  []PackFeedReaction    `json:"reactions,omitempty"`
	Thread     []PackFeedThreadReply `json:"thread,omitempty"`
}

// PackFeedForViewer — лента «стаи» из user_messages (отчёты) для участника/оплатившего.
// initD сверяется с MONETIZED_CHAT_ID, если в подписи есть group/supergroup.
func (b *Bot) PackFeedForViewer(viewerUserID int64, initD initdata.InitData) ([]PackFeedItem, error) {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return nil, err
	}
	chatID := b.config.MonetizedChatID
	if chatID == 0 {
		return []PackFeedItem{}, nil
	}
	packTitle := ""
	if initD.Chat.ID != 0 && (initD.Chat.Type == initdata.ChatTypeSupergroup || initD.Chat.Type == initdata.ChatTypeGroup) {
		packTitle = initD.Chat.Title
	}
	if b.config.OwnerID != 0 && viewerUserID == b.config.OwnerID {
		// владелец видит ленту без лишних проверок
	} else {
		ok, err := b.db.UserInPackOrPaid(viewerUserID, chatID, b.config.PaywallEnabled)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrPackFeedForbidden
		}
	}
	// Показываем общую историю стаи для всех участников, без персональной отсечки "с момента входа".
	rows, err := b.db.ListPackActivityFeed(chatID, 50, nil)
	if err != nil {
		return nil, err
	}
	out := make([]PackFeedItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, PackFeedItem{
			ID:               r.ID,
			UserID:           r.UserID,
			Username:         r.Username,
			Type:             r.MessageType,
			Text:             r.MessageText,
			CreatedAt:        r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			StreakDays:       r.StreakDays,
			IsYou:            r.UserID == viewerUserID,
			PackChatID:       chatID,
			PackTitle:        packTitle,
			TrainingPhotoURL: b.canonicalMiniappTrainingPhotoURL(r.TrainingPhotoURL),
		})
	}
	out = b.enrichPackFeedTrainingSocial(out, viewerUserID, chatID)
	out = b.enrichPackFeedAuthorPhotos(out, chatID)
	return out, nil
}

func packFeedIsLeoNoticeType(t string) bool {
	switch t {
	case "pack_join", "pack_rejoin", "daily_wisdom", "pack_removed":
		return true
	default:
		return false
	}
}

// enrichPackFeedAuthorPhotos — подмешивает telegram_photo_url из miniapp_user_profile (сохранён при онбординге из initData user.photo_url).
func (b *Bot) enrichPackFeedAuthorPhotos(items []PackFeedItem, chatID int64) []PackFeedItem {
	if b == nil || b.db == nil || chatID == 0 || len(items) == 0 {
		return items
	}
	var ids []int64
	seen := map[int64]struct{}{}
	add := func(uid int64) {
		if uid == 0 {
			return
		}
		if _, ok := seen[uid]; ok {
			return
		}
		seen[uid] = struct{}{}
		ids = append(ids, uid)
	}
	for _, it := range items {
		for _, tr := range it.Thread {
			if !tr.IsLeo {
				add(tr.UserID)
			}
		}
		if !packFeedIsLeoNoticeType(it.Type) {
			add(it.UserID)
		}
	}
	if len(ids) == 0 {
		return items
	}
	m, err := b.db.MiniappTelegramPhotoURLMap(chatID, ids)
	if err != nil {
		b.logger.Warnf("miniapp feed author photos map: %v", err)
		return items
	}
	for i := range items {
		if !packFeedIsLeoNoticeType(items[i].Type) && items[i].UserID != 0 {
			items[i].AuthorPhotoURL = m[items[i].UserID]
		}
		for j := range items[i].Thread {
			if items[i].Thread[j].IsLeo || items[i].Thread[j].UserID == 0 {
				continue
			}
			items[i].Thread[j].AuthorPhotoURL = m[items[i].Thread[j].UserID]
		}
	}
	return items
}

