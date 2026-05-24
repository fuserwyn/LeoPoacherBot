package database

import (
	"fmt"
	"time"

	"leo-bot/internal/domain"
)

// InsertMiniappPackGroupMessage — одно сообщение в общем чате мини-аппа.
func (d *Database) InsertMiniappPackGroupMessage(packChatID, fromUserID int64, username string, isLeo bool, messageText string) (int64, error) {
	const q = `
		INSERT INTO miniapp_pack_group_chat (pack_chat_id, from_user_id, username, is_leo, message_text)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var id int64
	err := d.db.QueryRow(q, packChatID, fromUserID, username, isLeo, messageText).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert miniapp pack group: %w", err)
	}
	return id, nil
}

// ListMiniappPackGroupChat — последние сообщения общего чата.
// sinceUTC — если не nil, только сообщения не раньше этого момента (личная граница истории в мини-аппе).
func (d *Database) ListMiniappPackGroupChat(packChatID int64, limit int, sinceUTC *time.Time) ([]*domain.PackGroupChatMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	whereSince := ""
	args := []interface{}{packChatID, limit}
	if sinceUTC != nil {
		whereSince = " AND created_at >= $3"
		args = append(args, *sinceUTC)
	}
	q := `
		SELECT id, from_user_id, COALESCE(username, ''), is_leo, message_text, created_at
		FROM miniapp_pack_group_chat
		WHERE pack_chat_id = $1
		` + whereSince + `
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list miniapp pack group: %w", err)
	}
	defer rows.Close()
	var items []*domain.PackGroupChatMessage
	for rows.Next() {
		var m domain.PackGroupChatMessage
		var t time.Time
		if err := rows.Scan(&m.ID, &m.UserID, &m.Username, &m.IsLeo, &m.Text, &t); err != nil {
			return nil, err
		}
		m.CreatedAt = t.UTC().Format("2006-01-02T15:04:05Z07:00")
		items = append(items, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	if items == nil {
		items = []*domain.PackGroupChatMessage{}
	}
	return items, nil
}

// DeleteMiniappPackGroupMessageByAuthor — удалить своё сообщение (не Лео) в общем чате мини-аппа.
func (d *Database) DeleteMiniappPackGroupMessageByAuthor(packChatID, messageID, actorUserID int64) (bool, error) {
	if packChatID == 0 || messageID == 0 || actorUserID == 0 {
		return false, nil
	}
	res, err := d.db.Exec(
		`DELETE FROM miniapp_pack_group_chat
		 WHERE id = $1 AND pack_chat_id = $2 AND from_user_id = $3 AND is_leo = FALSE`,
		messageID, packChatID, actorUserID,
	)
	if err != nil {
		return false, fmt.Errorf("delete miniapp pack group by author: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
