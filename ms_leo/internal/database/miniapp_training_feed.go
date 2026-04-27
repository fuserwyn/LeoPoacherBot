package database

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

// GetUserMessageTypeByIDForChat — проверка, что строка user_messages принадлежит чату.
func (d *Database) GetUserMessageTypeByIDForChat(id, chatID int64) (messageType string, ok bool, err error) {
	err = d.db.QueryRow(
		`SELECT message_type FROM user_messages WHERE id = $1 AND chat_id = $2`,
		id, chatID,
	).Scan(&messageType)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return messageType, true, nil
}

// TrainingFeedReactionAgg — агрегат реакций по одному user_message_id.
type TrainingFeedReactionAgg struct {
	Emoji string
	Count int
}

// ListTrainingFeedReactionAggs — подсчёт реакций по списку parent id; viewerID для флага «моя».
func (d *Database) ListTrainingFeedReactionAggs(packChatID int64, userMessageIDs []int64, viewerUserID int64) (map[int64][]TrainingFeedReactionAgg, map[int64]string, error) {
	out := make(map[int64][]TrainingFeedReactionAgg)
	me := make(map[int64]string)
	if len(userMessageIDs) == 0 {
		return out, me, nil
	}
	q := `
		SELECT user_message_id, emoji, COUNT(*)::int AS cnt
		FROM miniapp_training_feed_reactions
		WHERE pack_chat_id = $1 AND user_message_id = ANY($2)
		GROUP BY user_message_id, emoji
		ORDER BY user_message_id, emoji
	`
	rows, err := d.db.Query(q, packChatID, pq.Array(userMessageIDs))
	if err != nil {
		return nil, nil, fmt.Errorf("training feed reaction aggs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var mid int64
		var emoji string
		var cnt int
		if err := rows.Scan(&mid, &emoji, &cnt); err != nil {
			return nil, nil, err
		}
		out[mid] = append(out[mid], TrainingFeedReactionAgg{Emoji: emoji, Count: cnt})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	qMe := `
		SELECT user_message_id, emoji FROM miniapp_training_feed_reactions
		WHERE pack_chat_id = $1 AND user_message_id = ANY($2) AND user_id = $3
	`
	r2, err := d.db.Query(qMe, packChatID, pq.Array(userMessageIDs), viewerUserID)
	if err != nil {
		return nil, nil, fmt.Errorf("training feed reaction me: %w", err)
	}
	defer r2.Close()
	for r2.Next() {
		var mid int64
		var emoji string
		if err := r2.Scan(&mid, &emoji); err != nil {
			return nil, nil, err
		}
		me[mid] = emoji
	}
	return out, me, r2.Err()
}

// SetTrainingFeedReaction — поставить реакцию; та же эмодзи повторно снимает.
func (d *Database) SetTrainingFeedReaction(packChatID, userMessageID, userID int64, username, emoji string) error {
	emoji = strings.TrimSpace(emoji)
	var cur sql.NullString
	err := d.db.QueryRow(
		`SELECT emoji FROM miniapp_training_feed_reactions WHERE user_message_id = $1 AND user_id = $2`,
		userMessageID, userID,
	).Scan(&cur)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil && cur.Valid && cur.String == emoji {
		_, err := d.db.Exec(
			`DELETE FROM miniapp_training_feed_reactions WHERE user_message_id = $1 AND user_id = $2`,
			userMessageID, userID,
		)
		return err
	}
	const upsert = `
		INSERT INTO miniapp_training_feed_reactions (pack_chat_id, user_message_id, user_id, username, emoji)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_message_id, user_id) DO UPDATE SET
			emoji = EXCLUDED.emoji,
			username = EXCLUDED.username,
			updated_at = NOW()
	`
	_, err = d.db.Exec(upsert, packChatID, userMessageID, userID, strings.TrimSpace(username), emoji)
	return err
}

// TrainingFeedThreadRow — одна реплика в треде.
type TrainingFeedThreadRow struct {
	ID           int64
	UserMessageID int64
	FromUserID   int64
	Username     string
	MessageText  string
	CreatedAt    time.Time
}

// InsertTrainingFeedThreadReply — комментарий под отчётом.
func (d *Database) InsertTrainingFeedThreadReply(packChatID, userMessageID, fromUserID int64, username, text string) (int64, error) {
	const q = `
		INSERT INTO miniapp_training_feed_thread (pack_chat_id, user_message_id, from_user_id, username, message_text)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var id int64
	err := d.db.QueryRow(q, packChatID, userMessageID, fromUserID, strings.TrimSpace(username), text).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert training thread: %w", err)
	}
	return id, nil
}

// ListTrainingFeedThreadByMessages — все реплики для данных parent id, по времени по возрастанию.
func (d *Database) ListTrainingFeedThreadByMessages(userMessageIDs []int64) (map[int64][]TrainingFeedThreadRow, error) {
	res := make(map[int64][]TrainingFeedThreadRow)
	if len(userMessageIDs) == 0 {
		return res, nil
	}
	const q = `
		SELECT id, user_message_id, from_user_id, COALESCE(username, ''), message_text, created_at
		FROM miniapp_training_feed_thread
		WHERE user_message_id = ANY($1)
		ORDER BY user_message_id, created_at ASC
	`
	rows, err := d.db.Query(q, pq.Array(userMessageIDs))
	if err != nil {
		return nil, fmt.Errorf("list training thread: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r TrainingFeedThreadRow
		if err := rows.Scan(&r.ID, &r.UserMessageID, &r.FromUserID, &r.Username, &r.MessageText, &r.CreatedAt); err != nil {
			return nil, err
		}
		res[r.UserMessageID] = append(res[r.UserMessageID], r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// SortReactionAggsForDisplay — стабильный порядок эмодзи в UI.
func SortReactionAggsForDisplay(aggs []TrainingFeedReactionAgg, preferredOrder []string) []TrainingFeedReactionAgg {
	idx := make(map[string]int, len(preferredOrder))
	for i, e := range preferredOrder {
		idx[e] = i
	}
	cp := append([]TrainingFeedReactionAgg(nil), aggs...)
	sort.SliceStable(cp, func(i, j int) bool {
		ai, okA := idx[cp[i].Emoji]
		bi, okB := idx[cp[j].Emoji]
		if okA && okB {
			return ai < bi
		}
		if okA {
			return true
		}
		if okB {
			return false
		}
		return cp[i].Emoji < cp[j].Emoji
	})
	return cp
}
