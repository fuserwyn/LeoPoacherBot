package database

import (
	"database/sql"
	"fmt"
	"time"

	"leo-bot/internal/domain"

	"github.com/lib/pq"
)

// PackGroupChatRow — строка общего чата с опциональной ссылкой reply.
type PackGroupChatRow struct {
	ID          int64
	FromUserID  int64
	Username    string
	IsLeo       bool
	MessageText string
	CreatedAt   time.Time
	ReplyToID   sql.NullInt64
}

// InsertMiniappPackGroupMessage — одно сообщение в общем чате мини-аппа.
func (d *Database) InsertMiniappPackGroupMessage(packChatID, fromUserID int64, username string, isLeo bool, messageText string, replyToID int64) (int64, error) {
	var replyArg interface{}
	if replyToID > 0 {
		replyArg = replyToID
	}
	const q = `
		INSERT INTO miniapp_pack_group_chat (pack_chat_id, from_user_id, username, is_leo, message_text, reply_to_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	var id int64
	err := d.db.QueryRow(q, packChatID, fromUserID, username, isLeo, messageText, replyArg).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert miniapp pack group: %w", err)
	}
	return id, nil
}

// GetMiniappPackGroupMessageInPack — сообщение общего чата в рамках стаи.
func (d *Database) GetMiniappPackGroupMessageInPack(packChatID, messageID int64) (PackGroupChatRow, bool, error) {
	var row PackGroupChatRow
	err := d.db.QueryRow(`
		SELECT id, from_user_id, COALESCE(username, ''), is_leo, message_text, created_at, reply_to_id
		FROM miniapp_pack_group_chat
		WHERE id = $1 AND pack_chat_id = $2
	`, messageID, packChatID).Scan(
		&row.ID, &row.FromUserID, &row.Username, &row.IsLeo, &row.MessageText, &row.CreatedAt, &row.ReplyToID,
	)
	if err == sql.ErrNoRows {
		return PackGroupChatRow{}, false, nil
	}
	if err != nil {
		return PackGroupChatRow{}, false, fmt.Errorf("get miniapp pack group message: %w", err)
	}
	return row, true, nil
}

// ListMiniappPackGroupChatRows — последние сообщения общего чата (сырые строки).
func (d *Database) ListMiniappPackGroupChatRows(packChatID int64, limit int, sinceUTC *time.Time) ([]PackGroupChatRow, error) {
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
		SELECT id, from_user_id, COALESCE(username, ''), is_leo, message_text, created_at, reply_to_id
		FROM miniapp_pack_group_chat
		WHERE pack_chat_id = $1
		  AND COALESCE(is_hidden, FALSE) = FALSE
		` + whereSince + `
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list miniapp pack group: %w", err)
	}
	defer rows.Close()
	var items []PackGroupChatRow
	for rows.Next() {
		var m PackGroupChatRow
		if err := rows.Scan(&m.ID, &m.FromUserID, &m.Username, &m.IsLeo, &m.MessageText, &m.CreatedAt, &m.ReplyToID); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	if items == nil {
		items = []PackGroupChatRow{}
	}
	return items, nil
}

// ListMiniappPackGroupChat — последние сообщения общего чата.
func (d *Database) ListMiniappPackGroupChat(packChatID int64, limit int, sinceUTC *time.Time) ([]*domain.PackGroupChatMessage, error) {
	rows, err := d.ListMiniappPackGroupChatRows(packChatID, limit, sinceUTC)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.PackGroupChatMessage, 0, len(rows))
	for _, r := range rows {
		m := packGroupRowToDomain(r)
		out = append(out, &m)
	}
	return out, nil
}

func packGroupRowToDomain(r PackGroupChatRow) domain.PackGroupChatMessage {
	m := domain.PackGroupChatMessage{
		ID:        r.ID,
		UserID:    r.FromUserID,
		Username:  r.Username,
		Text:      r.MessageText,
		CreatedAt: r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		IsLeo:     r.IsLeo,
	}
	if r.ReplyToID.Valid && r.ReplyToID.Int64 > 0 {
		m.ReplyToID = r.ReplyToID.Int64
	}
	return m
}

// ListMiniappPackGroupMessagesByIDs — родительские сообщения для quote enrichment.
func (d *Database) ListMiniappPackGroupMessagesByIDs(packChatID int64, ids []int64) (map[int64]PackGroupChatRow, error) {
	out := map[int64]PackGroupChatRow{}
	if packChatID == 0 || len(ids) == 0 {
		return out, nil
	}
	rows, err := d.db.Query(`
		SELECT id, from_user_id, COALESCE(username, ''), is_leo, message_text, created_at, reply_to_id
		FROM miniapp_pack_group_chat
		WHERE pack_chat_id = $1 AND id = ANY($2)
	`, packChatID, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("list miniapp pack group by ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m PackGroupChatRow
		if err := rows.Scan(&m.ID, &m.FromUserID, &m.Username, &m.IsLeo, &m.MessageText, &m.CreatedAt, &m.ReplyToID); err != nil {
			return nil, err
		}
		out[m.ID] = m
	}
	return out, rows.Err()
}

// InsertPackGroupUnread — бейдж «Стая» при ответе в общем чате.
func (d *Database) InsertPackGroupUnread(recipientUserID, packChatID, packMessageID int64) error {
	if recipientUserID == 0 || packChatID == 0 || packMessageID == 0 {
		return nil
	}
	_, err := d.db.Exec(`
		INSERT INTO miniapp_pack_group_unread (recipient_user_id, pack_chat_id, pack_message_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (pack_message_id) DO NOTHING
	`, recipientUserID, packChatID, packMessageID)
	if err != nil {
		return fmt.Errorf("insert pack group unread: %w", err)
	}
	return nil
}

// DeletePackGroupUnreadByMessageID — при удалении ответа убрать из непрочитанных.
func (d *Database) DeletePackGroupUnreadByMessageID(packMessageID int64) error {
	if packMessageID == 0 {
		return nil
	}
	_, err := d.db.Exec(`DELETE FROM miniapp_pack_group_unread WHERE pack_message_id = $1`, packMessageID)
	if err != nil {
		return fmt.Errorf("delete pack group unread: %w", err)
	}
	return nil
}

// CountPackGroupUnread — число непросмотренных ответов в общем чате.
func (d *Database) CountPackGroupUnread(recipientUserID, packChatID int64) (int64, error) {
	if recipientUserID == 0 || packChatID == 0 {
		return 0, nil
	}
	var n int64
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM miniapp_pack_group_unread WHERE recipient_user_id = $1 AND pack_chat_id = $2`,
		recipientUserID, packChatID,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count pack group unread: %w", err)
	}
	return n, nil
}

// ClearPackGroupUnread — пользователь открыл общий чат стаи.
func (d *Database) ClearPackGroupUnread(recipientUserID, packChatID int64) error {
	if recipientUserID == 0 || packChatID == 0 {
		return nil
	}
	_, err := d.db.Exec(
		`DELETE FROM miniapp_pack_group_unread WHERE recipient_user_id = $1 AND pack_chat_id = $2`,
		recipientUserID, packChatID,
	)
	if err != nil {
		return fmt.Errorf("clear pack group unread: %w", err)
	}
	return nil
}

// ListPackGroupUnreadMessageIDs — id сообщений с непрочитанными ответами.
func (d *Database) ListPackGroupUnreadMessageIDs(recipientUserID, packChatID int64) ([]int64, error) {
	if recipientUserID == 0 || packChatID == 0 {
		return nil, nil
	}
	rows, err := d.db.Query(
		`SELECT pack_message_id FROM miniapp_pack_group_unread
		 WHERE recipient_user_id = $1 AND pack_chat_id = $2
		 ORDER BY pack_message_id ASC`,
		recipientUserID, packChatID,
	)
	if err != nil {
		return nil, fmt.Errorf("list pack group unread ids: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
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
