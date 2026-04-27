package database

import (
	"fmt"
	"time"

	"leo-bot/internal/domain"
)

// InsertMiniappPersonalChatMessage пишет одно сообщение приватного чата (юзер↔Лео) в БД.
// role: "user" — юзер пишет в мини-аппе или ЛС с ботом; "leo" — Лео отвечает (включая
// push-warning'и о неактивности и daily wisdom). Все сообщения помечены парой
// (user_id, pack_chat_id), чтобы при мульти-pack-сценарии потом легко изолировать.
func (d *Database) InsertMiniappPersonalChatMessage(userID, packChatID int64, role, text string) (int64, error) {
	const q = `
		INSERT INTO miniapp_personal_chat (user_id, pack_chat_id, role, message_text)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	var id int64
	if err := d.db.QueryRow(q, userID, packChatID, role, text).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert miniapp personal chat: %w", err)
	}
	return id, nil
}

// ListMiniappPersonalChat — возвращает последние limit сообщений в хронологическом порядке
// (старые → новые). sinceID > 0 — отдаём только записи с id > sinceID (для инкрементальной
// подгрузки клиентом). Если sinceID == 0 — отдаём последние limit сообщений (для первого
// открытия чата).
func (d *Database) ListMiniappPersonalChat(userID, packChatID int64, limit int, sinceID int64) ([]*domain.MiniappPersonalChatMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	if sinceID > 0 {
		// Инкрементальная подгрузка: всё, что новее, в хронологическом порядке (ASC).
		const q = `
			SELECT id, role, message_text, created_at
			FROM miniapp_personal_chat
			WHERE user_id = $1 AND pack_chat_id = $2 AND id > $3
			ORDER BY id ASC
			LIMIT $4
		`
		return d.queryMiniappPersonalChat(q, userID, packChatID, sinceID, limit)
	}

	// Первое открытие: берём последние limit, потом разворачиваем в хронологический порядок.
	const q = `
		SELECT id, role, message_text, created_at
		FROM miniapp_personal_chat
		WHERE user_id = $1 AND pack_chat_id = $2
		ORDER BY id DESC
		LIMIT $3
	`
	items, err := d.queryMiniappPersonalChat(q, userID, packChatID, limit)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items, nil
}

func (d *Database) queryMiniappPersonalChat(query string, args ...interface{}) ([]*domain.MiniappPersonalChatMessage, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list miniapp personal chat: %w", err)
	}
	defer rows.Close()
	var items []*domain.MiniappPersonalChatMessage
	for rows.Next() {
		var m domain.MiniappPersonalChatMessage
		var t time.Time
		if err := rows.Scan(&m.ID, &m.Role, &m.Text, &t); err != nil {
			return nil, err
		}
		m.CreatedAt = t.UTC().Format("2006-01-02T15:04:05Z07:00")
		items = append(items, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []*domain.MiniappPersonalChatMessage{}
	}
	return items, nil
}
