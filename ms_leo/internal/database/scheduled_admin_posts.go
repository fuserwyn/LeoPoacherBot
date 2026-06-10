package database

import (
	"time"
)

// ScheduledAdminPost — отложенный админский пост ленты стаи.
// Author: "leo" | "admin" — от чьего имени публиковать.
type ScheduledAdminPost struct {
	ID          int64
	ChatID      int64
	Author      string
	MessageText string
	ScheduledAt time.Time
	CreatedBy   int64
}

// InsertScheduledAdminPost ставит пост в очередь на публикацию в scheduledAt.
func (d *Database) InsertScheduledAdminPost(chatID int64, author, text string, scheduledAt time.Time, createdBy int64) (int64, error) {
	var id int64
	err := d.db.QueryRow(`
		INSERT INTO scheduled_admin_posts (chat_id, author, message_text, scheduled_at, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, chatID, author, text, scheduledAt, createdBy).Scan(&id)
	return id, err
}

// ClaimDueScheduledAdminPosts атомарно забирает посты, срок которых наступил (scheduled_at <= now),
// помечая их published_at, и возвращает их для публикации. FOR UPDATE SKIP LOCKED — на случай
// нескольких воркеров; в текущем сетапе бот один, но так безопаснее.
func (d *Database) ClaimDueScheduledAdminPosts(now time.Time) ([]ScheduledAdminPost, error) {
	rows, err := d.db.Query(`
		UPDATE scheduled_admin_posts
		SET published_at = (NOW() AT TIME ZONE 'Europe/Moscow')
		WHERE id IN (
			SELECT id FROM scheduled_admin_posts
			WHERE published_at IS NULL AND canceled_at IS NULL AND scheduled_at <= $1
			ORDER BY scheduled_at
			LIMIT 20
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, chat_id, author, message_text, scheduled_at, created_by
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ScheduledAdminPost
	for rows.Next() {
		var p ScheduledAdminPost
		if err := rows.Scan(&p.ID, &p.ChatID, &p.Author, &p.MessageText, &p.ScheduledAt, &p.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListPendingScheduledAdminPosts — ещё не опубликованные и не отменённые посты чата (для админ-панели).
func (d *Database) ListPendingScheduledAdminPosts(chatID int64, limit int) ([]ScheduledAdminPost, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.Query(`
		SELECT id, chat_id, author, message_text, scheduled_at, created_by
		FROM scheduled_admin_posts
		WHERE chat_id = $1 AND published_at IS NULL AND canceled_at IS NULL
		ORDER BY scheduled_at
		LIMIT $2
	`, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ScheduledAdminPost
	for rows.Next() {
		var p ScheduledAdminPost
		if err := rows.Scan(&p.ID, &p.ChatID, &p.Author, &p.MessageText, &p.ScheduledAt, &p.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CancelScheduledAdminPost помечает отложенный пост отменённым (только если ещё не опубликован).
// Возвращает true, если строка действительно была отменена.
func (d *Database) CancelScheduledAdminPost(chatID, id int64) (bool, error) {
	res, err := d.db.Exec(`
		UPDATE scheduled_admin_posts
		SET canceled_at = (NOW() AT TIME ZONE 'Europe/Moscow')
		WHERE id = $1 AND chat_id = $2 AND published_at IS NULL AND canceled_at IS NULL
	`, id, chatID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
