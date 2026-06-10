package bot

import (
	"strings"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// PackMember — участник стаи для экрана «Друзья» в мини-аппе.
type PackMember struct {
	UserID     int64  `json:"user_id"`
	Name       string `json:"name"`
	StreakDays int    `json:"streak_days"`
	Following  bool   `json:"following"`
}

// ListPackMembersForViewer — участники стаи (кроме самого viewer) с флагом подписки.
func (b *Bot) ListPackMembersForViewer(viewerUserID int64, initD initdata.InitData) ([]PackMember, error) {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return nil, err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return nil, err
	}
	chatID := b.config.MonetizedChatID
	rows, err := b.db.ListPackMembersForSubscriptions(chatID, viewerUserID)
	if err != nil {
		return nil, err
	}
	out := make([]PackMember, 0, len(rows))
	for _, r := range rows {
		name := strings.TrimSpace(r.DisplayName)
		if name == "" {
			name = normalizeUserDisplayName(r.Username)
		}
		if strings.TrimSpace(name) == "" {
			name = "Участник стаи"
		}
		out = append(out, PackMember{
			UserID:     r.UserID,
			Name:       name,
			StreakDays: r.StreakDays,
			Following:  r.Following,
		})
	}
	return out, nil
}

// FollowFriend / UnfollowFriend — подписка viewer на участника стаи.
func (b *Bot) FollowFriend(viewerUserID int64, initD initdata.InitData, targetUserID int64) error {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return err
	}
	if targetUserID == 0 || targetUserID == viewerUserID {
		return ErrTrainingFeedParentNotFound
	}
	chatID := b.config.MonetizedChatID
	// Подписываться можно только на реального участника стаи.
	ok, err := b.db.UserInPackOrPaid(targetUserID, chatID, b.config.PaywallEnabled)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTrainingFeedParentNotFound
	}
	return b.db.AddFriendSubscription(viewerUserID, targetUserID, chatID)
}

func (b *Bot) UnfollowFriend(viewerUserID int64, initD initdata.InitData, targetUserID int64) error {
	if err := b.AssertMiniAppPackChatAligns(initD); err != nil {
		return err
	}
	if err := b.assertPackFeedSocialViewer(viewerUserID); err != nil {
		return err
	}
	if targetUserID == 0 {
		return ErrTrainingFeedParentNotFound
	}
	chatID := b.config.MonetizedChatID
	return b.db.RemoveFriendSubscription(viewerUserID, targetUserID, chatID)
}

// enrichPackFeedFriends — проставляет IsFriend на карточках авторов, на которых подписан viewer.
// Один запрос за все подписки viewer'а; стоит дёшево даже на большой ленте.
func (b *Bot) enrichPackFeedFriends(items []PackFeedItem, viewerUserID, packChatID int64) []PackFeedItem {
	if b == nil || b.db == nil || viewerUserID == 0 || packChatID == 0 || len(items) == 0 {
		return items
	}
	targetIDs, err := b.db.ListFriendTargetIDs(viewerUserID, packChatID)
	if err != nil {
		b.logger.Warnf("pack feed friends enrich: %v", err)
		return items
	}
	if len(targetIDs) == 0 {
		return items
	}
	friends := make(map[int64]struct{}, len(targetIDs))
	for _, id := range targetIDs {
		friends[id] = struct{}{}
	}
	for i := range items {
		if _, ok := friends[items[i].UserID]; ok {
			items[i].IsFriend = true
		}
	}
	return items
}
