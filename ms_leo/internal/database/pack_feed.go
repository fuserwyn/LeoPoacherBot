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
			COALESCE(um.training_photo_url, ''),
			um.pinned_at,
			um.edited_at
		FROM user_messages um
		LEFT JOIN training_state ml
			ON ml.user_id = um.user_id AND ml.chat_id = um.chat_id AND ml.is_deleted = FALSE
		LEFT JOIN miniapp_user_profile p
			ON p.user_id = um.user_id AND p.pack_chat_id = um.chat_id
		WHERE um.chat_id = $1
		  AND um.id > $2
		  AND COALESCE(um.is_hidden, FALSE) = FALSE
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

// ListPinnedAdminPosts — закреплённые админ-объявления стаи (pinned_at IS NOT NULL),
// свежезакреплённые сверху. Только admin_post/admin_poll могут быть закреплены.
func (d *Database) ListPinnedAdminPosts(chatID int64) ([]*domain.PackActivityRow, error) {
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
			COALESCE(um.training_photo_url, ''),
			um.pinned_at,
			um.edited_at
		FROM user_messages um
		LEFT JOIN training_state ml
			ON ml.user_id = um.user_id AND ml.chat_id = um.chat_id AND ml.is_deleted = FALSE
		LEFT JOIN miniapp_user_profile p
			ON p.user_id = um.user_id AND p.pack_chat_id = um.chat_id
		WHERE um.chat_id = $1
		  AND um.pinned_at IS NOT NULL
		  AND COALESCE(um.is_hidden, FALSE) = FALSE
		  AND um.message_type IN ('admin_post', 'admin_poll')
		ORDER BY um.pinned_at DESC
		LIMIT 20
	`
	rows, err := d.db.Query(q, chatID)
	if err != nil {
		return nil, fmt.Errorf("pinned admin posts: %w", err)
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

// SetUserMessagePinned — закрепить/открепить объявление админа (только admin_post/admin_poll).
// pinned=true ставит pinned_at = NOW(), false — обнуляет. Возвращает ok=false, если строки
// нужного типа в чате нет. Открепление сохраняет сам пост в ленте (по created_at).
func (d *Database) SetUserMessagePinned(chatID, userMessageID int64, pinned bool) (bool, error) {
	var q string
	if pinned {
		q = `
			UPDATE user_messages
			SET pinned_at = NOW()
			WHERE id = $1 AND chat_id = $2
			  AND message_type IN ('admin_post', 'admin_poll')
		`
	} else {
		q = `
			UPDATE user_messages
			SET pinned_at = NULL
			WHERE id = $1 AND chat_id = $2
			  AND message_type IN ('admin_post', 'admin_poll')
		`
	}
	res, err := d.db.Exec(q, userMessageID, chatID)
	if err != nil {
		return false, fmt.Errorf("set user message pinned: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UserInPackOrPaid — доступ к мини-аппу.
// При включённом paywall — только активная (не истёкшая) оплата в paywall_access_requests;
// training_state без оплаты не даёт вход (иначе после GDPR-удаления outbox мог оставить строку
// в training_state, и мини-апп открывался бы без новой оплаты).
// Без paywall — как раньше: живая запись training_state в чате стаи.
func (d *Database) UserInPackOrPaid(userID, chatID int64, paywallEnabled bool) (bool, error) {
	if paywallEnabled {
		return d.UserHasActivePaywallAccess(userID, chatID)
	}
	return d.UserHasActiveMessageLogInChat(userID, chatID)
}
