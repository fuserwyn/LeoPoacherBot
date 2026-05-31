package database

import (
	"database/sql"
	"fmt"
	"strings"

	"leo-bot/internal/utils"
)

// ImportPackUserInput — одна строка ручного переноса из старого приложения.
type ImportPackUserInput struct {
	UserID   int64
	Username string // пусто → NULL в training_state; ник подтянется при первом заходе в miniapp
}

// ImportPackUserOpts — поведение импорта.
type ImportPackUserOpts struct {
	// SkipPaywall — не трогать paywall_access_requests (PAYWALL_ENABLED=false).
	SkipPaywall bool
	// StartTimer — записать timer_start_time = сейчас (окно неактивности 8 дней).
	StartTimer bool
}

// ImportPackUserResult — что реально вставили (ON CONFLICT / уже есть → false).
type ImportPackUserResult struct {
	PaywallInserted  bool
	TrainingInserted bool
	ProfileInserted  bool
}

// ImportPackUser заполняет paywall_access_requests, training_state, miniapp_user_profile
// для одного Telegram user_id. Идемпотентно: существующие активные записи не дублируются.
func (d *Database) ImportPackUser(packChatID int64, in ImportPackUserInput, opts ImportPackUserOpts) (ImportPackUserResult, error) {
	var out ImportPackUserResult
	if d == nil || d.db == nil {
		return out, fmt.Errorf("database is nil")
	}
	if packChatID == 0 {
		return out, fmt.Errorf("pack chat id is required (MONETIZED_CHAT_ID)")
	}
	if in.UserID == 0 {
		return out, fmt.Errorf("user id is required")
	}

	username := strings.TrimSpace(in.Username)

	tx, err := d.db.Begin()
	if err != nil {
		return out, err
	}
	defer tx.Rollback()

	if !opts.SkipPaywall {
		inserted, err := importPackUserPaywall(tx, in.UserID, packChatID)
		if err != nil {
			return out, err
		}
		out.PaywallInserted = inserted
	}

	trainingInserted, err := importPackUserTrainingState(tx, in.UserID, packChatID, username, opts.StartTimer)
	if err != nil {
		return out, err
	}
	out.TrainingInserted = trainingInserted

	profileInserted, err := importPackUserProfile(tx, in.UserID, packChatID, username)
	if err != nil {
		return out, err
	}
	out.ProfileInserted = profileInserted

	if err := tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}

func importPackUserPaywall(tx *sql.Tx, userID, packChatID int64) (bool, error) {
	const q = `
		INSERT INTO paywall_access_requests (
			user_id, monetized_chat_id, status, completed_at, access_expires_at
		)
		SELECT $1, $2, 'completed', NOW(), 'infinity'::timestamptz
		WHERE NOT EXISTS (
			SELECT 1 FROM paywall_access_requests
			WHERE user_id = $1
			  AND monetized_chat_id = $2
			  AND status = 'completed'
			  AND access_expires_at IS NOT NULL
			  AND access_expires_at > NOW()
		)`
	res, err := tx.Exec(q, userID, packChatID)
	if err != nil {
		return false, fmt.Errorf("paywall_access_requests: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func importPackUserTrainingState(tx *sql.Tx, userID, packChatID int64, username string, startTimer bool) (bool, error) {
	now := utils.FormatMoscowTime(utils.GetMoscowTime())
	var timerStart any
	if startTimer {
		timerStart = now
	}

	const q = `
		INSERT INTO training_state (
			user_id, username, chat_id,
			streak_days, max_streak_days, cups_earned,
			last_message, has_training_done, has_sick_leave, has_healthy, is_deleted,
			timer_start_time, timezone_offset_from_moscow, achievement_count, return_count,
			created_at, updated_at
		) VALUES (
			$1, NULLIF($2, ''), $3,
			0, 0, 0,
			'', FALSE, FALSE, FALSE, FALSE,
			$4, 0, 0, 1,
			$5, $5
		)
		ON CONFLICT (user_id, chat_id) DO NOTHING`

	res, err := tx.Exec(q, userID, username, packChatID, timerStart, now)
	if err != nil {
		return false, fmt.Errorf("training_state: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func importPackUserProfile(tx *sql.Tx, userID, packChatID int64, displayName string) (bool, error) {
	const q = `
		INSERT INTO miniapp_user_profile (
			user_id, pack_chat_id, gender, display_name, age_years, updated_at
		) VALUES ($1, $2, '', $3, NULL, NOW())
		ON CONFLICT (user_id, pack_chat_id) DO NOTHING`

	res, err := tx.Exec(q, userID, packChatID, displayName)
	if err != nil {
		return false, fmt.Errorf("miniapp_user_profile: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}
