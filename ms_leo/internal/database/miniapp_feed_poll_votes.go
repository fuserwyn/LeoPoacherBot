package database

import (
	"fmt"

	"github.com/lib/pq"
)

// MiniappFeedPollVoteSummary — агрегат по одному варианту ответа опроса.
type MiniappFeedPollVoteSummary struct {
	OptionIndex int
	Count       int
}

// ListMiniappFeedPollVoteSummaries — агрегаты голосов по набору poll user_message_id.
func (d *Database) ListMiniappFeedPollVoteSummaries(userMessageIDs []int64) (map[int64][]MiniappFeedPollVoteSummary, error) {
	out := make(map[int64][]MiniappFeedPollVoteSummary)
	if len(userMessageIDs) == 0 {
		return out, nil
	}
	const q = `
		SELECT user_message_id, option_index, COUNT(*)::int AS cnt
		FROM miniapp_feed_poll_votes
		WHERE user_message_id = ANY($1)
		GROUP BY user_message_id, option_index
		ORDER BY user_message_id, option_index
	`
	rows, err := d.db.Query(q, pq.Array(userMessageIDs))
	if err != nil {
		return nil, fmt.Errorf("list miniapp feed poll vote summaries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var userMessageID int64
		var item MiniappFeedPollVoteSummary
		if err := rows.Scan(&userMessageID, &item.OptionIndex, &item.Count); err != nil {
			return nil, err
		}
		out[userMessageID] = append(out[userMessageID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ListMiniappFeedPollViewerVotes — выбранный вариант текущего viewer по набору опросов.
func (d *Database) ListMiniappFeedPollViewerVotes(userMessageIDs []int64, viewerUserID int64) (map[int64]int, error) {
	out := make(map[int64]int)
	if viewerUserID == 0 || len(userMessageIDs) == 0 {
		return out, nil
	}
	const q = `
		SELECT user_message_id, option_index
		FROM miniapp_feed_poll_votes
		WHERE user_id = $1 AND user_message_id = ANY($2)
	`
	rows, err := d.db.Query(q, viewerUserID, pq.Array(userMessageIDs))
	if err != nil {
		return nil, fmt.Errorf("list miniapp feed poll viewer votes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var userMessageID int64
		var optionIndex int
		if err := rows.Scan(&userMessageID, &optionIndex); err != nil {
			return nil, err
		}
		out[userMessageID] = optionIndex
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertMiniappFeedPollVote — один голос пользователя на один опрос с возможностью перевыбора.
func (d *Database) UpsertMiniappFeedPollVote(userMessageID, userID int64, optionIndex int) error {
	const q = `
		INSERT INTO miniapp_feed_poll_votes (user_message_id, user_id, option_index)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_message_id, user_id)
		DO UPDATE SET option_index = EXCLUDED.option_index, created_at = NOW()
	`
	if _, err := d.db.Exec(q, userMessageID, userID, optionIndex); err != nil {
		return fmt.Errorf("upsert miniapp feed poll vote: %w", err)
	}
	return nil
}
