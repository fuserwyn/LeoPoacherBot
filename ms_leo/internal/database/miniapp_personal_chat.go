package database

import (
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/domain"

	"github.com/lib/pq"
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

// InsertMiniappPersonalChatMessageWithPhoto — то же, что выше, но с прикреплённым фото
// (личный чат с Лео: юзер шлёт снимок для рекомендаций). text может быть пустым.
func (d *Database) InsertMiniappPersonalChatMessageWithPhoto(userID, packChatID int64, role, text, photoURL string) (int64, error) {
	const q = `
		INSERT INTO miniapp_personal_chat (user_id, pack_chat_id, role, message_text, photo_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var id int64
	if err := d.db.QueryRow(q, userID, packChatID, role, text, photoURL).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert miniapp personal chat with photo: %w", err)
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
			SELECT id, role, message_text, COALESCE(photo_url, ''), created_at
			FROM miniapp_personal_chat
			WHERE user_id = $1 AND pack_chat_id = $2 AND id > $3
			ORDER BY id ASC
			LIMIT $4
		`
		return d.queryMiniappPersonalChat(q, userID, packChatID, sinceID, limit)
	}

	// Первое открытие: берём последние limit, потом разворачиваем в хронологический порядок.
	const q = `
		SELECT id, role, message_text, COALESCE(photo_url, ''), created_at
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

// ListMiniappLeoConversations — последние приватные диалоги юзеров с Лео.
func (d *Database) ListMiniappLeoConversations(packChatID int64, limit int) ([]*domain.MiniappSupportConversation, error) {
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
			FROM miniapp_personal_chat c
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
		if err := rows.Scan(&m.ID, &m.Role, &m.Text, &m.PhotoURL, &t); err != nil {
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

// ToggleMiniappPersonalChatLike — toggle лайка на сообщение лички с Лео.
func (d *Database) ToggleMiniappPersonalChatLike(userID, packChatID, actorUserID, messageID int64) error {
	if userID == 0 || packChatID == 0 || actorUserID == 0 || messageID == 0 {
		return nil
	}
	var exists bool
	err := d.db.QueryRow(
		`SELECT EXISTS(
			SELECT 1
			FROM miniapp_personal_chat_likes l
			JOIN miniapp_personal_chat c ON c.id = l.message_id
			WHERE l.message_id = $1 AND l.user_id = $2
			  AND c.user_id = $3 AND c.pack_chat_id = $4
		)`,
		messageID, actorUserID, userID, packChatID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("personal chat like exists: %w", err)
	}
	if exists {
		_, err := d.db.Exec(
			`DELETE FROM miniapp_personal_chat_likes WHERE message_id = $1 AND user_id = $2`,
			messageID, actorUserID,
		)
		return err
	}
	_, err = d.db.Exec(
		`INSERT INTO miniapp_personal_chat_likes (message_id, user_id)
		 SELECT id, $2 FROM miniapp_personal_chat
		 WHERE id = $1 AND user_id = $3 AND pack_chat_id = $4
		 ON CONFLICT (message_id, user_id) DO NOTHING`,
		messageID, actorUserID, userID, packChatID,
	)
	return err
}

// EnrichMiniappPersonalChatLikes — проставляет like_count/like_me на сообщения.
func (d *Database) EnrichMiniappPersonalChatLikes(userID, packChatID, viewerUserID int64, items []*domain.MiniappPersonalChatMessage) error {
	if len(items) == 0 || userID == 0 || packChatID == 0 {
		return nil
	}
	ids := make([]int64, 0, len(items))
	for _, it := range items {
		if it != nil && it.ID > 0 {
			ids = append(ids, it.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := d.db.Query(
		`SELECT l.message_id, COUNT(*)::int
		 FROM miniapp_personal_chat_likes l
		 JOIN miniapp_personal_chat c ON c.id = l.message_id
		 WHERE c.user_id = $1 AND c.pack_chat_id = $2 AND l.message_id = ANY($3)
		 GROUP BY l.message_id`,
		userID, packChatID, pq.Array(ids),
	)
	if err != nil {
		return fmt.Errorf("personal chat like aggs: %w", err)
	}
	defer rows.Close()
	cntMap := map[int64]int{}
	for rows.Next() {
		var id int64
		var cnt int
		if err := rows.Scan(&id, &cnt); err != nil {
			return err
		}
		cntMap[id] = cnt
	}
	if err := rows.Err(); err != nil {
		return err
	}
	meMap := map[int64]bool{}
	if viewerUserID != 0 {
		rows2, err := d.db.Query(
			`SELECT l.message_id
			 FROM miniapp_personal_chat_likes l
			 JOIN miniapp_personal_chat c ON c.id = l.message_id
			 WHERE c.user_id = $1 AND c.pack_chat_id = $2
			   AND l.message_id = ANY($3) AND l.user_id = $4`,
			userID, packChatID, pq.Array(ids), viewerUserID,
		)
		if err != nil {
			return fmt.Errorf("personal chat like me: %w", err)
		}
		defer rows2.Close()
		for rows2.Next() {
			var id int64
			if err := rows2.Scan(&id); err != nil {
				return err
			}
			meMap[id] = true
		}
		if err := rows2.Err(); err != nil {
			return err
		}
	}
	for _, it := range items {
		if it == nil {
			continue
		}
		it.LikeCount = cntMap[it.ID]
		it.LikeMe = meMap[it.ID]
	}
	return nil
}

// CountMiniappPersonalChatUserMessagesOnDate — сообщения пользователя Лео за локальный календарный день.
func (d *Database) CountMiniappPersonalChatUserMessagesOnDate(userID, packChatID int64, localDate string, tzOffsetFromMoscow int) (int, error) {
	if userID == 0 || packChatID == 0 || strings.TrimSpace(localDate) == "" {
		return 0, nil
	}
	var n int
	err := d.db.QueryRow(`
		SELECT COUNT(*)
		FROM miniapp_personal_chat
		WHERE user_id = $1 AND pack_chat_id = $2 AND role = 'user'
		  AND ((created_at AT TIME ZONE 'Europe/Moscow') + ($3 || ' hours')::interval)::date = $4::date
	`, userID, packChatID, tzOffsetFromMoscow, localDate).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count miniapp personal chat on date: %w", err)
	}
	return n, nil
}
