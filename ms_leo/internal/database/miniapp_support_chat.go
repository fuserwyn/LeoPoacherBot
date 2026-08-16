package database

import (
	"fmt"
	"time"

	"leo-bot/internal/domain"
)

// InsertMiniappSupportChatMessage пишет одно сообщение отдельного чата поддержки.
// role: "user" | "support".
func (d *Database) InsertMiniappSupportChatMessage(userID, packChatID int64, role, text string) (int64, error) {
	const q = `
		INSERT INTO miniapp_support_chat (user_id, pack_chat_id, role, message_text)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	var id int64
	if err := d.db.QueryRow(q, userID, packChatID, role, text).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert miniapp support chat: %w", err)
	}
	return id, nil
}

// ListMiniappSupportChat — последние limit сообщений в хронологическом порядке.
func (d *Database) ListMiniappSupportChat(userID, packChatID int64, limit int, sinceID int64) ([]*domain.MiniappSupportChatMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	if sinceID > 0 {
		const q = `
			SELECT id, role, message_text, created_at
			FROM miniapp_support_chat
			WHERE user_id = $1 AND pack_chat_id = $2 AND id > $3
			ORDER BY id ASC
			LIMIT $4
		`
		return d.queryMiniappSupportChat(q, userID, packChatID, sinceID, limit)
	}

	const q = `
		SELECT id, role, message_text, created_at
		FROM miniapp_support_chat
		WHERE user_id = $1 AND pack_chat_id = $2
		ORDER BY id DESC
		LIMIT $3
	`
	items, err := d.queryMiniappSupportChat(q, userID, packChatID, limit)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items, nil
}

func (d *Database) queryMiniappSupportChat(query string, args ...interface{}) ([]*domain.MiniappSupportChatMessage, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list miniapp support chat: %w", err)
	}
	defer rows.Close()
	var items []*domain.MiniappSupportChatMessage
	for rows.Next() {
		var m domain.MiniappSupportChatMessage
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
		items = []*domain.MiniappSupportChatMessage{}
	}
	return items, nil
}

// ListMiniappSupportConversations — последние отдельные диалоги поддержки пользователей.
func (d *Database) ListMiniappSupportConversations(packChatID int64, limit int) ([]*domain.MiniappSupportConversation, error) {
	if packChatID == 0 {
		return []*domain.MiniappSupportConversation{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	const q = `
		WITH last_per_user AS (
			SELECT DISTINCT ON (c.user_id)
				c.user_id,
				COALESCE(NULLIF(BTRIM(p.display_name), ''), NULLIF(BTRIM(ts.username), ''), 'user' || c.user_id::text) AS display_name,
				c.role,
				c.message_text,
				c.created_at
			FROM miniapp_support_chat c
			LEFT JOIN miniapp_user_profile p
				ON p.user_id = c.user_id AND p.pack_chat_id = c.pack_chat_id
			LEFT JOIN training_state ts
				ON ts.user_id = c.user_id AND ts.chat_id = c.pack_chat_id
			WHERE c.pack_chat_id = $1
			ORDER BY c.user_id, c.id DESC
		)
		SELECT user_id, display_name, role, message_text, created_at
		FROM last_per_user
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := d.db.Query(q, packChatID, limit)
	if err != nil {
		return nil, fmt.Errorf("list miniapp support conversations: %w", err)
	}
	defer rows.Close()

	var out []*domain.MiniappSupportConversation
	for rows.Next() {
		var item domain.MiniappSupportConversation
		var created time.Time
		if err := rows.Scan(&item.UserID, &item.DisplayName, &item.LastRole, &item.LastText, &created); err != nil {
			return nil, err
		}
		item.LastCreated = created.UTC().Format("2006-01-02T15:04:05Z07:00")
		item.NeedsReply = item.LastRole == "user"
		out = append(out, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []*domain.MiniappSupportConversation{}
	}
	return out, nil
}

// CountMiniappSupportNeedingReply — диалоги, где последнее сообщение от пользователя.
func (d *Database) CountMiniappSupportNeedingReply(packChatID int64) (int, error) {
	if packChatID == 0 {
		return 0, nil
	}
	const q = `
		SELECT COUNT(*) FROM (
			SELECT DISTINCT ON (c.user_id) c.role
			FROM miniapp_support_chat c
			WHERE c.pack_chat_id = $1
			ORDER BY c.user_id, c.id DESC
		) last_per_user
		WHERE role = 'user'
	`
	var n int
	if err := d.db.QueryRow(q, packChatID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count support needing reply: %w", err)
	}
	return n, nil
}
