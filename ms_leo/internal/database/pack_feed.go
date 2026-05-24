package database

import (
	"fmt"
	"time"

	"leo-bot/internal/domain"
)

// ListPackActivityFeed — последние «отчёты» в чате стаи: #training_done и системные события Лео.
// #sick_leave / #healthy сознательно НЕ показываем в ленте — это приватная переписка с Лео.
// streak берётся из training_state на момент выборки.
// sinceUTC — если не nil, только сообщения не раньше этого момента (личная граница истории в мини-аппе).
func (d *Database) ListPackActivityFeed(chatID int64, limit int, sinceUTC *time.Time) ([]*domain.PackActivityRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	whereSince := ""
	args := []interface{}{chatID, limit}
	if sinceUTC != nil {
		whereSince = " AND um.created_at >= $3"
		args = append(args, *sinceUTC)
	}
	q := `
		SELECT
			um.id,
			um.user_id,
			um.chat_id,
			COALESCE(NULLIF(BTRIM(p.display_name), ''), NULLIF(BTRIM(um.username), ''), 'user' || um.user_id::text),
			um.message_text,
			um.message_type,
			um.created_at,
			COALESCE(ml.streak_days, 0)::int,
			COALESCE(um.training_photo_url, '')
		FROM user_messages um
		LEFT JOIN training_state ml
			ON ml.user_id = um.user_id AND ml.chat_id = um.chat_id AND ml.is_deleted = FALSE
		LEFT JOIN miniapp_user_profile p
			ON p.user_id = um.user_id AND p.pack_chat_id = um.chat_id
		WHERE um.chat_id = $1
		  AND um.message_type IN ('training_done', 'pack_join', 'pack_rejoin', 'daily_wisdom', 'pack_removed', 'admin_post', 'admin_poll')
		  ` + whereSince + `
		ORDER BY um.created_at DESC
		LIMIT $2
	`
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("pack activity feed: %w", err)
	}
	defer rows.Close()

	var out []*domain.PackActivityRow
	for rows.Next() {
		var r domain.PackActivityRow
		var createdAt time.Time
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.ChatID, &r.Username, &r.MessageText, &r.MessageType, &createdAt, &r.StreakDays, &r.TrainingPhotoURL,
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

// ListPackActivityFeedAfterID — новые записи ленты с id > sinceID (для polling мини-аппа).
func (d *Database) ListPackActivityFeedAfterID(chatID int64, sinceID int64, limit int) ([]*domain.PackActivityRow, error) {
	if sinceID <= 0 {
		return []*domain.PackActivityRow{}, nil
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 50 {
		limit = 50
	}
	const q = `
		SELECT
			um.id,
			um.user_id,
			um.chat_id,
			COALESCE(NULLIF(BTRIM(p.display_name), ''), NULLIF(BTRIM(um.username), ''), 'user' || um.user_id::text),
			um.message_text,
			um.message_type,
			um.created_at,
			COALESCE(ml.streak_days, 0)::int,
			COALESCE(um.training_photo_url, '')
		FROM user_messages um
		LEFT JOIN training_state ml
			ON ml.user_id = um.user_id AND ml.chat_id = um.chat_id AND ml.is_deleted = FALSE
		LEFT JOIN miniapp_user_profile p
			ON p.user_id = um.user_id AND p.pack_chat_id = um.chat_id
		WHERE um.chat_id = $1
		  AND um.id > $2
		  AND um.message_type IN ('training_done', 'pack_join', 'pack_rejoin', 'daily_wisdom', 'pack_removed', 'admin_post', 'admin_poll')
		ORDER BY um.id DESC
		LIMIT $3
	`
	rows, err := d.db.Query(q, chatID, sinceID, limit)
	if err != nil {
		return nil, fmt.Errorf("pack activity feed after id: %w", err)
	}
	defer rows.Close()

	var out []*domain.PackActivityRow
	for rows.Next() {
		var r domain.PackActivityRow
		var createdAt time.Time
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.ChatID, &r.Username, &r.MessageText, &r.MessageType, &createdAt, &r.StreakDays, &r.TrainingPhotoURL,
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

// UserInPackOrPaid — участник стаи (training_state) или оплаченный доступ.
func (d *Database) UserInPackOrPaid(userID, chatID int64, paywallEnabled bool) (bool, error) {
	if paywallEnabled {
		ok, err := d.UserHasActivePaywallAccess(userID, chatID)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return d.UserHasActiveMessageLogInChat(userID, chatID)
}
