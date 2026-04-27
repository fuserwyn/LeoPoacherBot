package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// MiniappUserProfile — опциональные поля мини-аппа (стая / MONETIZED_CHAT_ID).
type MiniappUserProfile struct {
	UserID      int64
	PackChatID  int64
	Gender      string
	DisplayName string
	AgeYears    sql.NullInt64
	UpdatedAt   time.Time
}

// GetMiniappUserProfile — профиль.
func (d *Database) GetMiniappUserProfile(userID, packChatID int64) (*MiniappUserProfile, error) {
	const q = `
		SELECT user_id, pack_chat_id, gender, display_name, age_years, updated_at
		FROM miniapp_user_profile
		WHERE user_id = $1 AND pack_chat_id = $2`
	var p MiniappUserProfile
	err := d.db.QueryRow(q, userID, packChatID).Scan(
		&p.UserID, &p.PackChatID, &p.Gender, &p.DisplayName, &p.AgeYears, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertMiniappUserProfile — полная замена полей.
func (d *Database) UpsertMiniappUserProfile(p *MiniappUserProfile) error {
	if p == nil {
		return fmt.Errorf("nil profile")
	}
	if p.PackChatID == 0 {
		return fmt.Errorf("pack chat id")
	}
	const q = `
		INSERT INTO miniapp_user_profile (user_id, pack_chat_id, gender, display_name, age_years, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, pack_chat_id) DO UPDATE SET
			gender = EXCLUDED.gender,
			display_name = EXCLUDED.display_name,
			age_years = EXCLUDED.age_years,
			updated_at = EXCLUDED.updated_at`
	_, err := d.db.Exec(q, p.UserID, p.PackChatID, p.Gender, p.DisplayName, p.AgeYears, time.Now().UTC())
	return err
}

// PatchTrainingStateGenderIfExists — дублирует пол в training_state (чат стаи).
func (d *Database) PatchTrainingStateGenderIfExists(userID, packChatID int64, gender string) error {
	gender = strings.TrimSpace(strings.ToLower(gender))
	if gender != "m" && gender != "f" && gender != "" {
		return nil
	}
	_, err := d.db.Exec(
		`UPDATE training_state SET gender = $1, updated_at = $2 WHERE user_id = $3 AND chat_id = $4`,
		gender, time.Now().UTC(), userID, packChatID,
	)
	return err
}

// GetTimezoneOffsetFromTrainingState — текущее смещение пользователя относительно МСК (часы) для пары (user, pack).
// 0 если строки нет (или ещё не онбординг) — клиент может это интерпретировать как «по умолчанию МСК».
func (d *Database) GetTimezoneOffsetFromTrainingState(userID, packChatID int64) (int, error) {
	const q = `SELECT COALESCE(timezone_offset_from_moscow, 0)
	             FROM training_state
	             WHERE user_id = $1 AND chat_id = $2`
	var offset int
	if err := d.db.QueryRow(q, userID, packChatID).Scan(&offset); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return offset, nil
}

// PatchTrainingStateTimezoneOffsetIfExists — точечно меняет TZ в training_state без перезаписи остальных полей.
// Если строки нет — молча no-op (онбординг ещё не прошёл, TZ не нужен).
func (d *Database) PatchTrainingStateTimezoneOffsetIfExists(userID, packChatID int64, offset int) error {
	if offset < -12 || offset > 12 {
		return fmt.Errorf("timezone offset out of range: %d", offset)
	}
	_, err := d.db.Exec(
		`UPDATE training_state SET timezone_offset_from_moscow = $1, updated_at = $2 WHERE user_id = $3 AND chat_id = $4`,
		offset, time.Now().UTC(), userID, packChatID,
	)
	return err
}
