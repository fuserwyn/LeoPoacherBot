package database

import (
	"fmt"
	"time"

	"leo-bot/internal/domain"
)

// Единая лента = тренировки/системные карточки (user_messages) + сообщения общего
// чата (miniapp_pack_group_chat), слитые по created_at. Здесь — запросы с курсором
// по времени для обоих источников; сам мёрж делает bot-слой.

// packActivityFeedSelect — общий SELECT+JOIN для строк ленты из user_messages.
// Плейсхолдер $1 = chat_id. Дальнейшие условия/порядок добавляются вызывающим.
const packActivityFeedSelect = `
	SELECT
		um.id,
		um.user_id,
		um.chat_id,
		COALESCE(NULLIF(BTRIM(p.display_name), ''), NULLIF(BTRIM(um.username), ''), 'user' || um.user_id::text),
		um.message_text,
		um.message_type,
		um.created_at,
		COALESCE(ml.streak_days, 0)::int,
		COALESCE(um.training_photo_url, ''),
		um.pinned_at,
		um.edited_at
	FROM user_messages um
	LEFT JOIN training_state ml
		ON ml.user_id = um.user_id AND ml.chat_id = um.chat_id AND ml.is_deleted = FALSE
	LEFT JOIN miniapp_user_profile p
		ON p.user_id = um.user_id AND p.pack_chat_id = um.chat_id
	WHERE um.chat_id = $1
	  AND COALESCE(um.is_hidden, FALSE) = FALSE
	  AND um.message_type IN ('training_done', 'pack_join', 'pack_rejoin', 'daily_wisdom', 'pack_removed', 'admin_post', 'admin_poll')
`

func scanPackActivityRows(d *Database, q string, args ...interface{}) ([]*domain.PackActivityRow, error) {
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("unified pack activity feed: %w", err)
	}
	defer rows.Close()
	var out []*domain.PackActivityRow
	for rows.Next() {
		var r domain.PackActivityRow
		var createdAt time.Time
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.ChatID, &r.Username, &r.MessageText, &r.MessageType, &createdAt, &r.StreakDays, &r.TrainingPhotoURL, &r.PinnedAt, &r.EditedAt,
		); err != nil {
			return nil, err
		}
		r.CreatedAt = createdAt
		out = append(out, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []*domain.PackActivityRow{}
	}
	return out, nil
}

func clampFeedLimit(limit int) int {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return limit
}

// ListPackActivityFeedDesc — записи ленты, самые новые сверху (created_at DESC).
// beforeTS != nil — строго старше указанного времени (подгрузка вниз / курсор).
func (d *Database) ListPackActivityFeedDesc(chatID int64, beforeTS *time.Time, limit int) ([]*domain.PackActivityRow, error) {
	limit = clampFeedLimit(limit)
	if beforeTS != nil {
		q := packActivityFeedSelect + ` AND um.created_at < $2 ORDER BY um.created_at DESC, um.id DESC LIMIT $3`
		return scanPackActivityRows(d, q, chatID, beforeTS.UTC(), limit)
	}
	q := packActivityFeedSelect + ` ORDER BY um.created_at DESC, um.id DESC LIMIT $2`
	return scanPackActivityRows(d, q, chatID, limit)
}

// ListPackActivityFeedAfterTS — записи ленты строго новее sinceTS (polling новых).
func (d *Database) ListPackActivityFeedAfterTS(chatID int64, sinceTS time.Time, limit int) ([]*domain.PackActivityRow, error) {
	limit = clampFeedLimit(limit)
	q := packActivityFeedSelect + ` AND um.created_at > $2 ORDER BY um.created_at DESC, um.id DESC LIMIT $3`
	return scanPackActivityRows(d, q, chatID, sinceTS.UTC(), limit)
}

// packGroupFeedSelect — SELECT для сообщений чата, попадающих в единую ленту.
// $1 = pack_chat_id, $2 = baseline id (в ленту только id > baseline).
const packGroupFeedSelect = `
	SELECT id, from_user_id, COALESCE(username, ''), is_leo, message_text, created_at, reply_to_id, edited_at, COALESCE(photo_url, '')
	FROM miniapp_pack_group_chat
	WHERE pack_chat_id = $1
	  AND COALESCE(is_hidden, FALSE) = FALSE
	  AND id > $2
`

func scanPackGroupRows(d *Database, q string, args ...interface{}) ([]PackGroupChatRow, error) {
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("unified pack group feed: %w", err)
	}
	defer rows.Close()
	var items []PackGroupChatRow
	for rows.Next() {
		var m PackGroupChatRow
		if err := rows.Scan(&m.ID, &m.FromUserID, &m.Username, &m.IsLeo, &m.MessageText, &m.CreatedAt, &m.ReplyToID, &m.EditedAt, &m.PhotoURL); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []PackGroupChatRow{}
	}
	return items, nil
}

// ListPackGroupChatFeedDesc — сообщения чата для единой ленты, новые сверху.
// baselineID отсекает старую историю; beforeTS != nil — строго старше курсора.
func (d *Database) ListPackGroupChatFeedDesc(chatID, baselineID int64, beforeTS *time.Time, limit int) ([]PackGroupChatRow, error) {
	limit = clampFeedLimit(limit)
	if beforeTS != nil {
		q := packGroupFeedSelect + ` AND created_at < $3 ORDER BY created_at DESC, id DESC LIMIT $4`
		return scanPackGroupRows(d, q, chatID, baselineID, beforeTS.UTC(), limit)
	}
	q := packGroupFeedSelect + ` ORDER BY created_at DESC, id DESC LIMIT $3`
	return scanPackGroupRows(d, q, chatID, baselineID, limit)
}

// ListPackGroupChatFeedAfterTS — сообщения чата строго новее sinceTS (polling новых).
func (d *Database) ListPackGroupChatFeedAfterTS(chatID, baselineID int64, sinceTS time.Time, limit int) ([]PackGroupChatRow, error) {
	limit = clampFeedLimit(limit)
	q := packGroupFeedSelect + ` AND created_at > $3 ORDER BY created_at DESC, id DESC LIMIT $4`
	return scanPackGroupRows(d, q, chatID, baselineID, sinceTS.UTC(), limit)
}

// GetPackMessageBaselineID — граница отсечки старых сообщений чата (см. миграцию 71).
// Если таблицы/строки нет — 0 (в ленту попадут все сообщения).
func (d *Database) GetPackMessageBaselineID() (int64, error) {
	var baseline int64
	err := d.db.QueryRow(`SELECT pack_message_baseline_id FROM unified_feed_meta WHERE id = 1`).Scan(&baseline)
	if err != nil {
		return 0, err
	}
	return baseline, nil
}
