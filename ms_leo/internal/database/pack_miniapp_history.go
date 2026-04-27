package database

import (
	"database/sql"
	"fmt"
	"time"
)

// PackMiniappHistorySinceUTC — с какого момента пользователю показывать общую историю в мини-аппе (лента + чат стаи).
// Активный платный доступ: с момента зачёта оплаты (минимальный completed_at среди ещё действующих периодов).
// Иначе участник по training_state: с момента появления записи или последнего возврата (returned_at).
func (d *Database) PackMiniappHistorySinceUTC(userID, monetizedChatID int64, paywallEnabled bool) (time.Time, error) {
	if paywallEnabled {
		var payAt sql.NullTime
		err := d.db.QueryRow(`
			SELECT MIN(COALESCE(completed_at, created_at))
			FROM paywall_access_requests
			WHERE user_id = $1 AND monetized_chat_id = $2
			  AND status = 'completed'
			  AND access_expires_at IS NOT NULL
			  AND access_expires_at > NOW()
		`, userID, monetizedChatID).Scan(&payAt)
		if err != nil {
			return time.Time{}, fmt.Errorf("pack miniapp history since (paywall): %w", err)
		}
		if payAt.Valid {
			return payAt.Time.UTC(), nil
		}
	}

	var anchor sql.NullTime
	err := d.db.QueryRow(`
		SELECT GREATEST(
			COALESCE(created_at, '-infinity'::timestamptz),
			COALESCE(returned_at, '-infinity'::timestamptz)
		)
		FROM training_state
		WHERE user_id = $1 AND chat_id = $2 AND is_deleted = FALSE
	`, userID, monetizedChatID).Scan(&anchor)
	if err == sql.ErrNoRows {
		return time.Time{}, fmt.Errorf("pack miniapp history since: no training_state for user %d chat %d", userID, monetizedChatID)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("pack miniapp history since (training_state): %w", err)
	}
	if !anchor.Valid {
		return time.Time{}, fmt.Errorf("pack miniapp history since: null anchor")
	}
	return anchor.Time.UTC(), nil
}
