package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Вход в десктопное приложение: подтверждение в чате вместо initData.
// Подробности схемы — в миграции 75.

// DesktopTokenHash — в базе держим только хеш токена.
func DesktopTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

// DesktopLoginStart — приложение объявило nonce и ждёт подтверждения.
func (d *Database) DesktopLoginStart(nonce string, ttl time.Duration) error {
	if d == nil || d.db == nil || strings.TrimSpace(nonce) == "" {
		return fmt.Errorf("nonce пуст")
	}
	_, err := d.db.Exec(`
		INSERT INTO desktop_logins (nonce, status, expires_at)
		VALUES ($1, 'pending', NOW() + $2::interval)
		ON CONFLICT (nonce) DO UPDATE
			SET status = 'pending', user_id = NULL, token_plain = NULL,
			    created_at = NOW(), expires_at = NOW() + $2::interval
	`, nonce, fmt.Sprintf("%d seconds", int(ttl.Seconds())))
	return err
}

// DesktopLoginConfirm — человек нажал «Войти» в чате: выдаём токен сессии.
// Возвращает false, если попытка чужая или протухла.
func (d *Database) DesktopLoginConfirm(nonce string, userID int64, token string) (bool, error) {
	if d == nil || d.db == nil || userID == 0 {
		return false, nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	err = tx.QueryRow(`
		SELECT status FROM desktop_logins
		WHERE nonce = $1 AND expires_at > NOW()
		FOR UPDATE
	`, nonce).Scan(&status)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if status != "pending" {
		return false, nil
	}
	if _, err := tx.Exec(`
		INSERT INTO desktop_sessions (token_hash, user_id) VALUES ($1, $2)
		ON CONFLICT (token_hash) DO NOTHING
	`, DesktopTokenHash(token), userID); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`
		UPDATE desktop_logins SET status = 'ok', user_id = $2, token_plain = $3 WHERE nonce = $1
	`, nonce, userID, token); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// DesktopLoginPoll — приложение забирает токен. Отдаём один раз: token_plain
// стирается сразу, дальше приложение живёт с сохранённым у себя токеном.
// status: pending | ok | expired.
func (d *Database) DesktopLoginPoll(nonce string) (status string, userID int64, token string, err error) {
	if d == nil || d.db == nil {
		return "expired", 0, "", nil
	}
	var (
		uid   sql.NullInt64
		tok   sql.NullString
		state string
		exp   time.Time
	)
	err = d.db.QueryRow(`
		SELECT status, user_id, token_plain, expires_at FROM desktop_logins WHERE nonce = $1
	`, nonce).Scan(&state, &uid, &tok, &exp)
	if err == sql.ErrNoRows {
		return "expired", 0, "", nil
	}
	if err != nil {
		return "", 0, "", err
	}
	if time.Now().After(exp) {
		return "expired", 0, "", nil
	}
	if state != "ok" {
		return "pending", 0, "", nil
	}
	if !tok.Valid || tok.String == "" {
		// Токен уже забрали: повторный опрос не должен выдавать его снова.
		return "expired", 0, "", nil
	}
	if _, err := d.db.Exec(`UPDATE desktop_logins SET token_plain = NULL WHERE nonce = $1`, nonce); err != nil {
		return "", 0, "", err
	}
	return "ok", uid.Int64, tok.String, nil
}

// DesktopSessionOwner — чей это токен. 0 — сессия неизвестна или отозвана.
func (d *Database) DesktopSessionOwner(token string) (int64, error) {
	if d == nil || d.db == nil {
		return 0, nil
	}
	var uid int64
	err := d.db.QueryRow(`
		SELECT user_id FROM desktop_sessions
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, DesktopTokenHash(token)).Scan(&uid)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	_, _ = d.db.Exec(`UPDATE desktop_sessions SET last_used_at = NOW() WHERE token_hash = $1`,
		DesktopTokenHash(token))
	return uid, nil
}

// DesktopSessionRevoke — выйти из приложения.
func (d *Database) DesktopSessionRevoke(token string) error {
	if d == nil || d.db == nil {
		return nil
	}
	_, err := d.db.Exec(`
		UPDATE desktop_sessions SET revoked_at = NOW()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, DesktopTokenHash(token))
	return err
}
