package bot

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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

// notifyFriendSubscribers — DM всем, кто подписан на trainerUserID, когда тот сдал #training_done.
// Вызывается из обработки training_done в отдельной горутине (silent fail на отправке).
func (b *Bot) notifyFriendSubscribers(trainerUserID, packChatID int64, trainerUsername string) {
	if b == nil || b.db == nil || b.api == nil || trainerUserID == 0 || packChatID == 0 {
		return
	}
	subscribers, err := b.db.ListFriendSubscribersOfTarget(trainerUserID, packChatID)
	if err != nil {
		b.logger.Warnf("friend subscribers lookup trainer=%d: %v", trainerUserID, err)
		return
	}
	if len(subscribers) == 0 {
		return
	}
	// Имя: display_name из профиля, иначе @username.
	name := ""
	if _, dn, _ := b.GetMiniappUserProfileJSONForAPI(trainerUserID, packChatID); strings.TrimSpace(dn) != "" {
		name = strings.TrimSpace(dn)
	} else {
		name = normalizeUserDisplayName(trainerUsername)
	}
	if strings.TrimSpace(name) == "" {
		name = "Твой друг по стае"
	}
	body := "🔥 " + name + " только что отметил тренировку в стае!\n\nОткрой мини-апп → вкладка «Стая», поддержи реакцией или комментарием 💪"
	for _, sub := range subscribers {
		if sub == 0 || sub == trainerUserID {
			continue
		}
		b.miniappPersonalPush(sub, body)
		if _, sendErr := b.api.Send(tgbotapi.NewMessage(sub, body)); sendErr != nil {
			b.logger.Warnf("friend training DM sub=%d trainer=%d: %v", sub, trainerUserID, sendErr)
		}
	}
}
