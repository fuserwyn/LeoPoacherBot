package database

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// AdminPackUserSearchHit — результат поиска пользователя в админке.
type AdminPackUserSearchHit struct {
	UserID      int64
	Username    string
	DisplayName string
	IsDeleted   bool
}

// SearchPackUsersForAdmin ищет пользователей стаи по ID, @нику или display_name из мини-аппа.
func (d *Database) SearchPackUsersForAdmin(packChatID int64, rawQuery string, limit int) ([]AdminPackUserSearchHit, error) {
	if d == nil || packChatID == 0 {
		return nil, nil
	}
	q := strings.TrimSpace(rawQuery)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	pattern := "%" + strings.TrimPrefix(q, "@") + "%"
	var exactID int64
	if id, err := strconv.ParseInt(q, 10, 64); err == nil && id > 0 {
		exactID = id
	}

	const query = `
		SELECT ts.user_id,
		       COALESCE(NULLIF(BTRIM(ts.username), ''), ''),
		       COALESCE(NULLIF(BTRIM(p.display_name), ''), ''),
		       ts.is_deleted
		FROM training_state ts
		LEFT JOIN miniapp_user_profile p
			ON p.user_id = ts.user_id AND p.pack_chat_id = ts.chat_id
		WHERE ts.chat_id = $1
		  AND (
		        ($2 > 0 AND ts.user_id = $2)
		     OR ts.username ILIKE $3
		     OR COALESCE(p.display_name, '') ILIKE $3
		  )
		ORDER BY ts.is_deleted ASC, ts.cups_earned DESC, ts.user_id ASC
		LIMIT $4
	`
	rows, err := d.db.Query(query, packChatID, exactID, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("search pack users: %w", err)
	}
	defer rows.Close()

	var out []AdminPackUserSearchHit
	for rows.Next() {
		var hit AdminPackUserSearchHit
		if err := rows.Scan(&hit.UserID, &hit.Username, &hit.DisplayName, &hit.IsDeleted); err != nil {
			return nil, err
		}
		out = append(out, hit)
	}
	return out, rows.Err()
}

// DecrementStreakSaveAttemptsUsed уменьшает счётчик использованных попыток (мин. 0) — даёт +1 доступную попытку.
func (d *Database) DecrementStreakSaveAttemptsUsed(userID, chatID int64, count int) (int, error) {
	if count <= 0 {
		count = 1
	}
	query := `
		UPDATE training_state
		SET streak_save_attempts_used = GREATEST(0, COALESCE(streak_save_attempts_used, 0) - $3),
		    updated_at = NOW() AT TIME ZONE 'Europe/Moscow'
		WHERE user_id = $1 AND chat_id = $2
		RETURNING COALESCE(streak_save_attempts_used, 0)
	`
	var used int
	err := d.db.QueryRow(query, userID, chatID, count).Scan(&used)
	if err != nil {
		return 0, err
	}
	return used, nil
}

// AdminDeleteFeedUserMessage — жёсткое удаление поста ленты (user_messages) и связанных реакций/треда.
func (d *Database) AdminDeleteFeedUserMessage(packChatID, messageID int64) (bool, error) {
	if packChatID == 0 || messageID == 0 {
		return false, nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	if err := tx.QueryRow(
		`SELECT 1 FROM user_messages WHERE id = $1 AND chat_id = $2`,
		messageID, packChatID,
	).Scan(&exists); err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("check user_message: %w", err)
	}

	if _, err := tx.Exec(
		`DELETE FROM miniapp_training_thread_unread WHERE thread_reply_id IN (
			SELECT id FROM miniapp_training_feed_thread WHERE user_message_id = $1 AND pack_chat_id = $2
		)`,
		messageID, packChatID,
	); err != nil {
		return false, fmt.Errorf("delete thread unread: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM miniapp_training_feed_thread_likes WHERE thread_reply_id IN (
			SELECT id FROM miniapp_training_feed_thread WHERE user_message_id = $1 AND pack_chat_id = $2
		)`,
		messageID, packChatID,
	); err != nil {
		return false, fmt.Errorf("delete thread likes: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM miniapp_training_feed_thread WHERE user_message_id = $1 AND pack_chat_id = $2`,
		messageID, packChatID,
	); err != nil {
		return false, fmt.Errorf("delete thread: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM miniapp_training_feed_reactions WHERE user_message_id = $1 AND pack_chat_id = $2`,
		messageID, packChatID,
	); err != nil {
		return false, fmt.Errorf("delete reactions: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM miniapp_feed_poll_votes WHERE user_message_id = $1`, messageID); err != nil {
		return false, fmt.Errorf("delete poll votes: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM miniapp_feed_reports WHERE user_message_id = $1 AND pack_chat_id = $2`,
		messageID, packChatID,
	); err != nil {
		return false, fmt.Errorf("delete feed reports: %w", err)
	}
	res, err := tx.Exec(`DELETE FROM user_messages WHERE id = $1 AND chat_id = $2`, messageID, packChatID)
	if err != nil {
		return false, fmt.Errorf("delete user_message: %w", err)
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n > 0, nil
}

// AdminDeleteTrainingFeedThreadReply — удалить комментарий в треде (без проверки автора).
func (d *Database) AdminDeleteTrainingFeedThreadReply(packChatID, threadReplyID int64) (bool, error) {
	if packChatID == 0 || threadReplyID == 0 {
		return false, nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`DELETE FROM miniapp_training_thread_unread WHERE thread_reply_id = $1`,
		threadReplyID,
	); err != nil {
		return false, err
	}
	if _, err := tx.Exec(
		`DELETE FROM miniapp_training_feed_thread_likes WHERE thread_reply_id = $1`,
		threadReplyID,
	); err != nil {
		return false, err
	}
	res, err := tx.Exec(
		`DELETE FROM miniapp_training_feed_thread WHERE id = $1 AND pack_chat_id = $2`,
		threadReplyID, packChatID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if _, err := tx.Exec(
		`DELETE FROM miniapp_feed_reports WHERE thread_reply_id = $1 AND pack_chat_id = $2`,
		threadReplyID, packChatID,
	); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n > 0, nil
}

// AdminDeletePackGroupMessage — удалить сообщение общего чата стаи (без проверки автора).
func (d *Database) AdminDeletePackGroupMessage(packChatID, messageID int64) (bool, error) {
	if packChatID == 0 || messageID == 0 {
		return false, nil
	}
	if _, err := d.db.Exec(
		`DELETE FROM miniapp_pack_group_unread WHERE pack_message_id = $1`,
		messageID,
	); err != nil {
		return false, fmt.Errorf("delete pack group unread: %w", err)
	}
	res, err := d.db.Exec(
		`DELETE FROM miniapp_pack_group_chat WHERE id = $1 AND pack_chat_id = $2`,
		messageID, packChatID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
