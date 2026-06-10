package bot

import (
	"errors"
	"fmt"
	"strings"

	initdata "github.com/telegram-mini-apps/init-data-golang"

	"leo-bot/internal/database"
	"leo-bot/internal/domain"
)

// ErrPackFeedForbidden — смотрящему нельзя видеть ленту (нет в стае / не оплачено).
var ErrPackFeedForbidden = errors.New("pack feed forbidden")

// PackVoter — кто отреагировал/лайкнул: имя + аватар (для поповера «кто лайкнул»).
type PackVoter struct {
	Name     string `json:"name"`
	PhotoURL string `json:"photo_url,omitempty"`
}

// packVoters — конвертация голосов в DTO с отрезолвленным аватаром: безопасный
// публичный photo_url из профиля или прокси через Bot API (как у авторов карточек).
func (b *Bot) packVoters(in []database.Voter, initDataRaw string) []PackVoter {
	if len(in) == 0 {
		return nil
	}
	publicBase := ""
	if b != nil && b.config != nil {
		publicBase = strings.TrimSpace(b.config.MiniappPublicBaseURL)
	}
	out := make([]PackVoter, len(in))
	for i, v := range in {
		out[i] = PackVoter{
			Name:     v.Name,
			PhotoURL: packFeedResolveAuthorPhoto(v.PhotoURL, publicBase, v.UserID, initDataRaw),
		}
	}
	return out
}

// PackFeedReaction — агрегат реакций на отчёт в мини-аппе.
type PackFeedReaction struct {
	Emoji  string      `json:"emoji"`
	Count  int         `json:"count"`
	Me     bool        `json:"me"`
	Voters []PackVoter `json:"voters,omitempty"`
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
	LikeCount       int         `json:"like_count,omitempty"`
	LikeMe          bool        `json:"like_me,omitempty"`
	LikeVoters      []PackVoter `json:"like_voters,omitempty"`
}

type PackFeedPollOption struct {
	Label string `json:"label"`
	Votes int    `json:"votes"`
}

type PackFeedPoll struct {
	Question    string               `json:"question"`
	Options     []PackFeedPollOption `json:"options"`
	TotalVotes  int                  `json:"total_votes"`
	MyVoteIndex *int                 `json:"my_vote_index,omitempty"`
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
	Poll       *PackFeedPoll         `json:"poll,omitempty"`
	// IsPinned — объявление закреплено админом (висит сверху ленты с пометкой «объявление»).
	IsPinned bool   `json:"is_pinned,omitempty"`
	PinnedAt string `json:"pinned_at,omitempty"`
}

// PackFeedForViewer — лента стаи из user_messages. sinceID > 0 — только id новее (polling).
// initD сверяется с MONETIZED_CHAT_ID; initDataRaw — для прокси аватаров.
func (b *Bot) PackFeedForViewer(viewerUserID int64, initD initdata.InitData, initDataRaw string, sinceID int64) ([]PackFeedItem, error) {
	if err := b.PackFeedAssertViewerAccess(viewerUserID, initD); err != nil {
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
	var rows []*domain.PackActivityRow
	var err error
	if sinceID > 0 {
		rows, err = b.db.ListPackActivityFeedAfterID(chatID, sinceID, 30)
	} else {
		// Показываем общую историю стаи для всех участников, без персональной отсечки "с момента входа".
		rows, err = b.db.ListPackActivityFeed(chatID, 50, nil)
	}
	if err != nil {
		return nil, err
	}
	out := make([]PackFeedItem, 0, len(rows))
	for _, r := range rows {
		if r.MessageType == "sick_leave" {
			continue
		}
		out = append(out, b.packFeedItemFromRow(r, viewerUserID, chatID, packTitle))
	}
	out = b.enrichPackFeedTrainingSocial(out, viewerUserID, chatID, initDataRaw)
	out = b.enrichPackFeedPolls(out, viewerUserID, chatID)
	out = b.enrichPackFeedAuthorPhotos(out, chatID, initDataRaw)
	return out, nil
}

// packFeedItemFromRow конвертирует строку ленты в PackFeedItem (с пометкой закрепа).
func (b *Bot) packFeedItemFromRow(r *domain.PackActivityRow, viewerUserID, chatID int64, packTitle string) PackFeedItem {
	item := PackFeedItem{
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
	}
	if r.PinnedAt != nil {
		item.IsPinned = true
		item.PinnedAt = r.PinnedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return item
}

// PackFeedPinnedForViewer — текущие закреплённые объявления стаи (свежезакреплённые сверху).
// Возвращается отдельно от хронологической ленты как авторитетный набор для фронта:
// фронт по нему синхронизирует флаг закрепа (в т.ч. снимает закреп при открепе).
func (b *Bot) PackFeedPinnedForViewer(viewerUserID int64, initD initdata.InitData, initDataRaw string) ([]PackFeedItem, error) {
	if err := b.PackFeedAssertViewerAccess(viewerUserID, initD); err != nil {
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
	rows, err := b.db.ListPinnedAdminPosts(chatID)
	if err != nil {
		return nil, err
	}
	out := make([]PackFeedItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, b.packFeedItemFromRow(r, viewerUserID, chatID, packTitle))
	}
	out = b.enrichPackFeedTrainingSocial(out, viewerUserID, chatID, initDataRaw)
	out = b.enrichPackFeedPolls(out, viewerUserID, chatID)
	out = b.enrichPackFeedAuthorPhotos(out, chatID, initDataRaw)
	return out, nil
}

// PackFeedAdminSetPin — админ закрепляет/открепляет объявление в ленте стаи.
// Только админ; только admin_post/admin_poll. Открепление оставляет пост в ленте.
func (b *Bot) PackFeedAdminSetPin(viewerUserID int64, initD initdata.InitData, userMessageID int64, pin bool) error {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return err
	}
	if b == nil || b.db == nil {
		return fmt.Errorf("bot unavailable")
	}
	if !b.isAdminTelegramUser(viewerUserID) {
		return ErrPackFeedForbidden
	}
	if userMessageID <= 0 {
		return ErrTrainingFeedParentNotFound
	}
	chatID := b.config.MonetizedChatID
	if chatID == 0 {
		return ErrTrainingFeedParentNotFound
	}
	ok, err := b.db.SetUserMessagePinned(chatID, userMessageID, pin)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTrainingFeedParentNotFound
	}
	return nil
}

func packFeedIsLeoNoticeType(t string) bool {
	switch t {
	case "pack_join", "pack_rejoin", "daily_wisdom", "pack_removed", "admin_post", "admin_poll":
		return true
	default:
		return false
	}
}

