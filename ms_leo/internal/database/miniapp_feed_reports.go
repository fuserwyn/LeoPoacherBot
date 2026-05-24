package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/domain"
)

// InsertMiniappFeedReport сохраняет жалобу; threadReplyID=0 для поста в ленте.
func (d *Database) InsertMiniappFeedReport(
	packChatID, reporterUserID int64,
	targetType string,
	userMessageID, threadReplyID, targetUserID int64,
	targetText string,
) (int64, error) {
	const q = `
		INSERT INTO miniapp_feed_reports (
			pack_chat_id, reporter_user_id, target_type, user_message_id,
			thread_reply_id, target_user_id, target_text
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`
	var id int64
	err := d.db.QueryRow(
		q,
		packChatID,
		reporterUserID,
		targetType,
		userMessageID,
		threadReplyID,
		targetUserID,
		strings.TrimSpace(targetText),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert feed report: %w", err)
	}
	return id, nil
}

// CountOpenMiniappFeedReports — открытые жалобы в стае.
func (d *Database) CountOpenMiniappFeedReports(packChatID int64) (int, error) {
	if packChatID == 0 {
		return 0, nil
	}
	var n int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM miniapp_feed_reports WHERE pack_chat_id = $1 AND status = 'open'`,
		packChatID,
	).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ListOpenMiniappFeedReports — последние открытые жалобы.
func (d *Database) ListOpenMiniappFeedReports(packChatID int64, limit int) ([]*domain.MiniappFeedReport, error) {
	if packChatID == 0 {
		return []*domain.MiniappFeedReport{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	const q = `
		SELECT
			r.id,
			r.reporter_user_id,
			COALESCE(NULLIF(BTRIM(pr.display_name), ''), ''),
			r.target_type,
			r.user_message_id,
			r.thread_reply_id,
			r.target_user_id,
			COALESCE(NULLIF(BTRIM(pt.display_name), ''), ''),
			r.target_text,
			r.status,
			r.created_at
		FROM miniapp_feed_reports r
		LEFT JOIN miniapp_user_profile pr
			ON pr.user_id = r.reporter_user_id AND pr.pack_chat_id = r.pack_chat_id
		LEFT JOIN miniapp_user_profile pt
			ON pt.user_id = r.target_user_id AND pt.pack_chat_id = r.pack_chat_id
		WHERE r.pack_chat_id = $1 AND r.status = 'open'
		ORDER BY r.created_at DESC
		LIMIT $2`
	rows, err := d.db.Query(q, packChatID, limit)
	if err != nil {
		return nil, fmt.Errorf("list feed reports: %w", err)
	}
	defer rows.Close()
	return scanMiniappFeedReports(rows)
}

// GetMiniappFeedReport — одна жалоба по id в стае.
func (d *Database) GetMiniappFeedReport(packChatID, reportID int64) (*domain.MiniappFeedReport, error) {
	if packChatID == 0 || reportID == 0 {
		return nil, sql.ErrNoRows
	}
	const q = `
		SELECT
			r.id,
			r.reporter_user_id,
			COALESCE(NULLIF(BTRIM(pr.display_name), ''), ''),
			r.target_type,
			r.user_message_id,
			r.thread_reply_id,
			r.target_user_id,
			COALESCE(NULLIF(BTRIM(pt.display_name), ''), ''),
			r.target_text,
			r.status,
			r.created_at
		FROM miniapp_feed_reports r
		LEFT JOIN miniapp_user_profile pr
			ON pr.user_id = r.reporter_user_id AND pr.pack_chat_id = r.pack_chat_id
		LEFT JOIN miniapp_user_profile pt
			ON pt.user_id = r.target_user_id AND pt.pack_chat_id = r.pack_chat_id
		WHERE r.pack_chat_id = $1 AND r.id = $2`
	row := d.db.QueryRow(q, packChatID, reportID)
	item, err := scanMiniappFeedReportRow(row)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// DismissMiniappFeedReport помечает жалобу обработанной.
func (d *Database) DismissMiniappFeedReport(packChatID, reportID int64) (bool, error) {
	res, err := d.db.Exec(
		`UPDATE miniapp_feed_reports SET status = 'dismissed'
		 WHERE pack_chat_id = $1 AND id = $2 AND status = 'open'`,
		packChatID, reportID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func scanMiniappFeedReports(rows *sql.Rows) ([]*domain.MiniappFeedReport, error) {
	var out []*domain.MiniappFeedReport
	for rows.Next() {
		item, err := scanMiniappFeedReportRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []*domain.MiniappFeedReport{}
	}
	return out, nil
}

func scanMiniappFeedReportRow(row *sql.Row) (*domain.MiniappFeedReport, error) {
	var item domain.MiniappFeedReport
	var created time.Time
	if err := row.Scan(
		&item.ID,
		&item.ReporterUserID,
		&item.ReporterName,
		&item.TargetType,
		&item.UserMessageID,
		&item.ThreadReplyID,
		&item.TargetUserID,
		&item.TargetName,
		&item.TargetText,
		&item.Status,
		&created,
	); err != nil {
		return nil, err
	}
	item.CreatedAt = created.UTC().Format(time.RFC3339)
	return &item, nil
}

func scanMiniappFeedReportRows(rows *sql.Rows) (*domain.MiniappFeedReport, error) {
	var item domain.MiniappFeedReport
	var created time.Time
	if err := rows.Scan(
		&item.ID,
		&item.ReporterUserID,
		&item.ReporterName,
		&item.TargetType,
		&item.UserMessageID,
		&item.ThreadReplyID,
		&item.TargetUserID,
		&item.TargetName,
		&item.TargetText,
		&item.Status,
		&created,
	); err != nil {
		return nil, err
	}
	item.CreatedAt = created.UTC().Format(time.RFC3339)
	return &item, nil
}
