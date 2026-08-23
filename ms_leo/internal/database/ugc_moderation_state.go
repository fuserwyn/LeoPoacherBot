package database

import (
	"database/sql"
	"fmt"
	"time"
)

// UGCModerationState — счётчик нарушений и мьют UGC-поверхностей.
type UGCModerationState struct {
	ViolationCount int
	MutedUntil     *time.Time
}

func (d *Database) GetUGCModerationState(userID, packChatID int64) (UGCModerationState, error) {
	var out UGCModerationState
	if d == nil || userID == 0 || packChatID == 0 {
		return out, nil
	}
	var mutedUntil sql.NullTime
	err := d.db.QueryRow(
		`SELECT COALESCE(ugc_violation_count, 0),
		        ugc_muted_until
		   FROM training_state
		  WHERE user_id = $1 AND chat_id = $2`,
		userID, packChatID,
	).Scan(&out.ViolationCount, &mutedUntil)
	if err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("get ugc moderation state: %w", err)
	}
	if mutedUntil.Valid {
		t := mutedUntil.Time
		out.MutedUntil = &t
	}
	return out, nil
}

func (d *Database) IsUserUGCMuted(userID, packChatID int64) (bool, error) {
	st, err := d.GetUGCModerationState(userID, packChatID)
	if err != nil {
		return false, err
	}
	if st.MutedUntil == nil {
		return false, nil
	}
	return st.MutedUntil.After(time.Now()), nil
}

func (d *Database) IncrementUGCViolationCount(userID, packChatID int64) (int, error) {
	if userID == 0 || packChatID == 0 {
		return 0, nil
	}
	var count int
	err := d.db.QueryRow(
		`UPDATE training_state
		    SET ugc_violation_count = COALESCE(ugc_violation_count, 0) + 1,
		        updated_at = NOW() AT TIME ZONE 'Europe/Moscow'
		  WHERE user_id = $1 AND chat_id = $2
		  RETURNING COALESCE(ugc_violation_count, 0)`,
		userID, packChatID,
	).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("increment ugc violation: %w", err)
	}
	return count, nil
}

func (d *Database) MuteUserUGCUntil(userID, packChatID int64, until time.Time) error {
	if userID == 0 || packChatID == 0 {
		return nil
	}
	_, err := d.db.Exec(
		`UPDATE training_state
		    SET ugc_muted_until = $3,
		        updated_at = NOW() AT TIME ZONE 'Europe/Moscow'
		  WHERE user_id = $1 AND chat_id = $2`,
		userID, packChatID, until.UTC(),
	)
	if err != nil {
		return fmt.Errorf("mute user ugc: %w", err)
	}
	return nil
}

func (d *Database) UnmuteUserUGC(userID, packChatID int64) error {
	if userID == 0 || packChatID == 0 {
		return nil
	}
	_, err := d.db.Exec(
		`UPDATE training_state
		    SET ugc_muted_until = NULL,
		        updated_at = NOW() AT TIME ZONE 'Europe/Moscow'
		  WHERE user_id = $1 AND chat_id = $2`,
		userID, packChatID,
	)
	if err != nil {
		return fmt.Errorf("unmute user ugc: %w", err)
	}
	return nil
}
