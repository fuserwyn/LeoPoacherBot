package database

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"leo-bot/internal/utils"
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

// AdminPackUserListRow — строка списка пользователей в админке.
type AdminPackUserListRow struct {
	UserID           int64
	Username         string
	DisplayName      string
	Cups             int
	StreakDays       int
	IsDeleted        bool
	HasActivePaywall bool
}

// AdminPaywallPaymentRow — строка оплаты для админки.
type AdminPaywallPaymentRow struct {
	ID            int64
	UserID        int64
	Username      string
	DisplayName   string
	Status        string
	CreatedAt     time.Time
	CompletedAt   sql.NullTime
	AmountMinor   sql.NullInt64
	Currency      sql.NullString
	AccessActive  bool
}

func (d *Database) CountPackUsersForAdmin(packChatID int64) (int, error) {
	if packChatID == 0 {
		return 0, nil
	}
	var n int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM training_state WHERE chat_id = $1`, packChatID).Scan(&n)
	return n, err
}

func (d *Database) ListPackUsersForAdmin(packChatID int64, offset, limit int) ([]AdminPackUserListRow, error) {
	if packChatID == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 8
	}
	if offset < 0 {
		offset = 0
	}
	const query = `
		SELECT ts.user_id,
		       COALESCE(NULLIF(BTRIM(ts.username), ''), ''),
		       COALESCE(NULLIF(BTRIM(p.display_name), ''), ''),
		       COALESCE(ts.cups_earned, 0),
		       COALESCE(ts.streak_days, 0),
		       ts.is_deleted,
		       EXISTS (
		           SELECT 1 FROM paywall_access_requests par
		           WHERE par.user_id = ts.user_id
		             AND par.monetized_chat_id = ts.chat_id
		             AND par.status = 'completed'
		             AND par.access_expires_at IS NOT NULL
		             AND par.access_expires_at > NOW()
		       )
		FROM training_state ts
		LEFT JOIN miniapp_user_profile p
			ON p.user_id = ts.user_id AND p.pack_chat_id = ts.chat_id
		WHERE ts.chat_id = $1
		ORDER BY ts.is_deleted ASC, ts.cups_earned DESC, ts.user_id ASC
		OFFSET $2 LIMIT $3
	`
	rows, err := d.db.Query(query, packChatID, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminPackUserListRow
	for rows.Next() {
		var r AdminPackUserListRow
		if err := rows.Scan(&r.UserID, &r.Username, &r.DisplayName, &r.Cups, &r.StreakDays, &r.IsDeleted, &r.HasActivePaywall); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (d *Database) CountPaywallPaymentsForAdmin(packChatID int64) (int, error) {
	if packChatID == 0 {
		return 0, nil
	}
	var n int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM paywall_access_requests WHERE monetized_chat_id = $1`, packChatID).Scan(&n)
	return n, err
}

func (d *Database) ListPaywallPaymentsForAdmin(packChatID int64, offset, limit int) ([]AdminPaywallPaymentRow, error) {
	if packChatID == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 6
	}
	if offset < 0 {
		offset = 0
	}
	const query = `
		SELECT par.id, par.user_id,
		       COALESCE(NULLIF(BTRIM(ts.username), ''), ''),
		       COALESCE(NULLIF(BTRIM(p.display_name), ''), ''),
		       par.status, par.created_at, par.completed_at,
		       par.total_amount_minor, par.currency,
		       (par.status = 'completed' AND par.access_expires_at IS NOT NULL AND par.access_expires_at > NOW())
		FROM paywall_access_requests par
		LEFT JOIN training_state ts
			ON ts.user_id = par.user_id AND ts.chat_id = par.monetized_chat_id
		LEFT JOIN miniapp_user_profile p
			ON p.user_id = par.user_id AND p.pack_chat_id = par.monetized_chat_id
		WHERE par.monetized_chat_id = $1
		ORDER BY par.id DESC
		OFFSET $2 LIMIT $3
	`
	rows, err := d.db.Query(query, packChatID, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminPaywallPaymentRow
	for rows.Next() {
		var r AdminPaywallPaymentRow
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.Username, &r.DisplayName, &r.Status, &r.CreatedAt, &r.CompletedAt,
			&r.AmountMinor, &r.Currency, &r.AccessActive,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (d *Database) ListPaywallPaymentsForUserAdmin(userID, packChatID int64, limit int) ([]AdminPaywallPaymentRow, error) {
	if userID == 0 || packChatID == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	const query = `
		SELECT par.id, par.user_id,
		       COALESCE(NULLIF(BTRIM(ts.username), ''), ''),
		       COALESCE(NULLIF(BTRIM(p.display_name), ''), ''),
		       par.status, par.created_at, par.completed_at,
		       par.total_amount_minor, par.currency,
		       (par.status = 'completed' AND par.access_expires_at IS NOT NULL AND par.access_expires_at > NOW())
		FROM paywall_access_requests par
		LEFT JOIN training_state ts
			ON ts.user_id = par.user_id AND ts.chat_id = par.monetized_chat_id
		LEFT JOIN miniapp_user_profile p
			ON p.user_id = par.user_id AND p.pack_chat_id = par.monetized_chat_id
		WHERE par.user_id = $1 AND par.monetized_chat_id = $2
		ORDER BY par.id DESC
		LIMIT $3
	`
	rows, err := d.db.Query(query, userID, packChatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminPaywallPaymentRow
	for rows.Next() {
		var r AdminPaywallPaymentRow
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.Username, &r.DisplayName, &r.Status, &r.CreatedAt, &r.CompletedAt,
			&r.AmountMinor, &r.Currency, &r.AccessActive,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ApplyStreakSaveForUserScope — спасение стрика: сдвигает last_training_date и при необходимости
// восстанавливает streak_days на pack-row и private-row (chat_id = user_id).
func (d *Database) ApplyStreakSaveForUserScope(userID, packChatID int64, newLastTrainingDate string, restoreStreakDays int) error {
	if userID == 0 || packChatID == 0 {
		return nil
	}
	newLastTrainingDate = strings.TrimSpace(newLastTrainingDate)
	if newLastTrainingDate == "" {
		return fmt.Errorf("empty last training date")
	}
	if restoreStreakDays < 0 {
		restoreStreakDays = 0
	}
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(`
		UPDATE training_state
		SET last_training_date = $3,
		    streak_days = CASE
		        WHEN COALESCE(streak_days, 0) > 0 THEN streak_days
		        ELSE $4
		    END,
		    updated_at = $5
		WHERE user_id = $1 AND (chat_id = $2 OR chat_id = $1)
	`, userID, packChatID, newLastTrainingDate, restoreStreakDays, moscowTime)
	if err != nil {
		return fmt.Errorf("apply streak save for user scope: %w", err)
	}
	return nil
}

// SubtractCupsForUserScope уменьшает кубки на pack-row и private-row (chat_id = user_id), не ниже 0.
func (d *Database) SubtractCupsForUserScope(userID, packChatID int64, cups int) error {
	if userID == 0 || packChatID == 0 || cups <= 0 {
		return nil
	}
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(`
		UPDATE training_state
		SET cups_earned = GREATEST(COALESCE(cups_earned, 0) - $3, 0),
		    updated_at = $4
		WHERE user_id = $1 AND (chat_id = $2 OR chat_id = $1)
	`, userID, packChatID, cups, moscowTime)
	if err != nil {
		return fmt.Errorf("subtract cups for user scope: %w", err)
	}
	return nil
}

// SubtractStreakDaysForUserScope уменьшает streak_days на pack-row и private-row, не ниже 0.
// max_streak_days (рекорд) не трогаем.
func (d *Database) SubtractStreakDaysForUserScope(userID, packChatID int64, days int) error {
	if userID == 0 || packChatID == 0 || days <= 0 {
		return nil
	}
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(`
		UPDATE training_state
		SET streak_days = GREATEST(COALESCE(streak_days, 0) - $3, 0),
		    updated_at = $4
		WHERE user_id = $1 AND (chat_id = $2 OR chat_id = $1)
	`, userID, packChatID, days, moscowTime)
	if err != nil {
		return fmt.Errorf("subtract streak for user scope: %w", err)
	}
	return nil
}

// SetAchievementsForUserScope выставляет ачивки на pack-row и private-row (chat_id = user_id).
func (d *Database) SetAchievementsForUserScope(userID, packChatID int64, achievementCount, lastAchievementStreakLevel int) error {
	if userID == 0 || packChatID == 0 {
		return nil
	}
	if achievementCount < 0 {
		achievementCount = 0
	}
	if achievementCount > 8 {
		achievementCount = 8
	}
	if lastAchievementStreakLevel < 0 {
		lastAchievementStreakLevel = 0
	}
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(`
		UPDATE training_state
		SET achievement_count = $3,
		    last_achievement_streak_level = $4,
		    updated_at = $5
		WHERE user_id = $1 AND (chat_id = $2 OR chat_id = $1)
	`, userID, packChatID, achievementCount, lastAchievementStreakLevel, moscowTime)
	if err != nil {
		return fmt.Errorf("set achievements for user scope: %w", err)
	}
	return nil
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
	if _, err := d.db.Exec(
		`DELETE FROM miniapp_feed_reports
		 WHERE pack_chat_id = $1 AND target_type = 'pack_group_message' AND user_message_id = $2`,
		packChatID, messageID,
	); err != nil {
		return false, fmt.Errorf("delete pack group feed reports: %w", err)
	}
	return n > 0, nil
}

// AdminHideFeedUserMessage — soft hide поста ленты (остаётся в БД для аудита).
func (d *Database) AdminHideFeedUserMessage(packChatID, messageID int64) (bool, error) {
	if packChatID == 0 || messageID == 0 {
		return false, nil
	}
	res, err := d.db.Exec(
		`UPDATE user_messages SET is_hidden = TRUE
		  WHERE id = $1 AND chat_id = $2 AND COALESCE(is_hidden, FALSE) = FALSE`,
		messageID, packChatID,
	)
	if err != nil {
		return false, fmt.Errorf("hide user_message: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// AdminHideTrainingFeedThreadReply — soft hide комментария в треде.
func (d *Database) AdminHideTrainingFeedThreadReply(packChatID, threadReplyID int64) (bool, error) {
	if packChatID == 0 || threadReplyID == 0 {
		return false, nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		`UPDATE miniapp_training_feed_thread SET is_hidden = TRUE
		  WHERE id = $1 AND pack_chat_id = $2 AND COALESCE(is_hidden, FALSE) = FALSE`,
		threadReplyID, packChatID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}
	if _, err := tx.Exec(
		`DELETE FROM miniapp_training_thread_unread WHERE thread_reply_id = $1`,
		threadReplyID,
	); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// AdminHidePackGroupMessage — soft hide сообщения общего чата.
func (d *Database) AdminHidePackGroupMessage(packChatID, messageID int64) (bool, error) {
	if packChatID == 0 || messageID == 0 {
		return false, nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		`UPDATE miniapp_pack_group_chat SET is_hidden = TRUE
		  WHERE id = $1 AND pack_chat_id = $2 AND COALESCE(is_hidden, FALSE) = FALSE`,
		messageID, packChatID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}
	if _, err := tx.Exec(
		`DELETE FROM miniapp_pack_group_unread WHERE pack_message_id = $1`,
		messageID,
	); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// AdminDeleteAllFeedMessagesByUser — удаляет все посты пользователя из ленты вместе с тредами,
// реакциями, голосами и жалобами. Возвращает количество удалённых постов.
func (d *Database) AdminDeleteAllFeedMessagesByUser(packChatID, userID int64) (int64, error) {
	if packChatID == 0 || userID == 0 {
		return 0, nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		DELETE FROM miniapp_training_thread_unread
		WHERE thread_reply_id IN (
			SELECT t.id FROM miniapp_training_feed_thread t
			JOIN user_messages m ON m.id = t.user_message_id
			WHERE m.user_id = $1 AND m.chat_id = $2
		)`, userID, packChatID); err != nil {
		return 0, fmt.Errorf("del thread unread: %w", err)
	}
	if _, err := tx.Exec(`
		DELETE FROM miniapp_training_feed_thread_likes
		WHERE thread_reply_id IN (
			SELECT t.id FROM miniapp_training_feed_thread t
			JOIN user_messages m ON m.id = t.user_message_id
			WHERE m.user_id = $1 AND m.chat_id = $2
		)`, userID, packChatID); err != nil {
		return 0, fmt.Errorf("del thread likes: %w", err)
	}
	if _, err := tx.Exec(`
		DELETE FROM miniapp_training_feed_thread
		WHERE pack_chat_id = $2 AND user_message_id IN (
			SELECT id FROM user_messages WHERE user_id = $1 AND chat_id = $2
		)`, userID, packChatID); err != nil {
		return 0, fmt.Errorf("del threads: %w", err)
	}
	if _, err := tx.Exec(`
		DELETE FROM miniapp_training_feed_reactions
		WHERE pack_chat_id = $2 AND user_message_id IN (
			SELECT id FROM user_messages WHERE user_id = $1 AND chat_id = $2
		)`, userID, packChatID); err != nil {
		return 0, fmt.Errorf("del reactions: %w", err)
	}
	if _, err := tx.Exec(`
		DELETE FROM miniapp_feed_poll_votes
		WHERE user_message_id IN (
			SELECT id FROM user_messages WHERE user_id = $1 AND chat_id = $2
		)`, userID, packChatID); err != nil {
		return 0, fmt.Errorf("del poll votes: %w", err)
	}
	if _, err := tx.Exec(`
		DELETE FROM miniapp_feed_reports
		WHERE pack_chat_id = $2 AND user_message_id IN (
			SELECT id FROM user_messages WHERE user_id = $1 AND chat_id = $2
		)`, userID, packChatID); err != nil {
		return 0, fmt.Errorf("del feed reports: %w", err)
	}
	res, err := tx.Exec(`DELETE FROM user_messages WHERE user_id = $1 AND chat_id = $2`, userID, packChatID)
	if err != nil {
		return 0, fmt.Errorf("del user_messages: %w", err)
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

// AdminDeleteAllPackGroupMessagesByUser — удаляет все сообщения пользователя из чата стаи.
// Возвращает количество удалённых сообщений.
func (d *Database) AdminDeleteAllPackGroupMessagesByUser(packChatID, userID int64) (int64, error) {
	if packChatID == 0 || userID == 0 {
		return 0, nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		DELETE FROM miniapp_pack_group_unread
		WHERE pack_message_id IN (
			SELECT id FROM miniapp_pack_group_chat WHERE user_id = $1 AND pack_chat_id = $2
		)`, userID, packChatID); err != nil {
		return 0, fmt.Errorf("del pack group unread: %w", err)
	}
	if _, err := tx.Exec(`
		DELETE FROM miniapp_pack_group_reactions
		WHERE pack_chat_id = $2 AND pack_message_id IN (
			SELECT id FROM miniapp_pack_group_chat WHERE user_id = $1 AND pack_chat_id = $2
		)`, userID, packChatID); err != nil {
		return 0, fmt.Errorf("del pack group reactions: %w", err)
	}
	if _, err := tx.Exec(`
		DELETE FROM miniapp_feed_reports
		WHERE pack_chat_id = $2 AND target_type = 'pack_group_message' AND user_message_id IN (
			SELECT id FROM miniapp_pack_group_chat WHERE user_id = $1 AND pack_chat_id = $2
		)`, userID, packChatID); err != nil {
		return 0, fmt.Errorf("del pack group reports: %w", err)
	}
	res, err := tx.Exec(`
		DELETE FROM miniapp_pack_group_chat WHERE user_id = $1 AND pack_chat_id = $2
	`, userID, packChatID)
	if err != nil {
		return 0, fmt.Errorf("del pack group messages: %w", err)
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}
