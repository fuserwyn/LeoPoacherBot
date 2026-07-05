package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// LikeNotificationSettings — настройка «уведомлять о лайках в ленте» для пары (user, стая).
type LikeNotificationSettings struct {
	Enabled bool
}

// GetLikeNotificationSettings — настройка уведомлений о лайках. Если строки нет — дефолт (ВЫКЛ).
// По умолчанию выключено: уведомления приходят только тем, кто сам включил галочку в мини-аппе.
func (d *Database) GetLikeNotificationSettings(userID, packChatID int64) (LikeNotificationSettings, error) {
	def := LikeNotificationSettings{Enabled: false}
	if d == nil || userID == 0 || packChatID == 0 {
		return def, nil
	}
	const q = `SELECT enabled FROM miniapp_like_notifications WHERE user_id = $1 AND pack_chat_id = $2`
	var s LikeNotificationSettings
	err := d.db.QueryRow(q, userID, packChatID).Scan(&s.Enabled)
	if err == sql.ErrNoRows {
		return def, nil
	}
	if err != nil {
		return def, fmt.Errorf("get like notification settings: %w", err)
	}
	return s, nil
}

// SaveLikeNotificationSettings — upsert настройки уведомлений о лайках.
func (d *Database) SaveLikeNotificationSettings(userID, packChatID int64, s LikeNotificationSettings) error {
	if d == nil || userID == 0 || packChatID == 0 {
		return nil
	}
	const q = `
		INSERT INTO miniapp_like_notifications (user_id, pack_chat_id, enabled, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, pack_chat_id) DO UPDATE SET
			enabled    = EXCLUDED.enabled,
			updated_at = NOW()`
	if _, err := d.db.Exec(q, userID, packChatID, s.Enabled); err != nil {
		return fmt.Errorf("save like notification settings: %w", err)
	}
	return nil
}

// IsLikeNotificationEnabled — быстрый чек для пути доставки: включил ли получатель уведомления.
func (d *Database) IsLikeNotificationEnabled(userID, packChatID int64) (bool, error) {
	s, err := d.GetLikeNotificationSettings(userID, packChatID)
	if err != nil {
		return false, err
	}
	return s.Enabled, nil
}

// MarkFeedLikeNotified — фиксирует факт отправки DM по паре (получатель, лайкнувший, цель).
// Возвращает firstTime=true, если запись создана впервые (значит DM отправлять можно);
// false — если уже уведомляли раньше (переставленный лайк не должен спамить личку).
func (d *Database) MarkFeedLikeNotified(recipientUserID, packChatID, likerUserID int64, targetKind string, targetID int64) (firstTime bool, err error) {
	if d == nil || recipientUserID == 0 || packChatID == 0 || likerUserID == 0 || targetID == 0 {
		return false, nil
	}
	targetKind = strings.TrimSpace(targetKind)
	if targetKind == "" {
		return false, nil
	}
	const q = `
		INSERT INTO miniapp_feed_like_notify_log
			(recipient_user_id, pack_chat_id, liker_user_id, target_kind, target_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (recipient_user_id, liker_user_id, target_kind, target_id) DO NOTHING`
	res, err := d.db.Exec(q, recipientUserID, packChatID, likerUserID, targetKind, targetID)
	if err != nil {
		return false, fmt.Errorf("mark feed like notified: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		// Драйвер не сообщил число строк — считаем, что уведомлять не стоит (безопаснее не спамить).
		return false, nil
	}
	return n > 0, nil
}
