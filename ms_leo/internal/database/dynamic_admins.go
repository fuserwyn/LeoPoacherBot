package database

import (
	"fmt"
	"strings"
	"time"
)

// DynamicAdmin — запись администратора, добавленного владельцем через бот.
type DynamicAdmin struct {
	UserID   int64
	Username string
	AddedBy  int64
	AddedAt  time.Time
}

// AddDynamicAdmin — добавить администратора; при повторном добавлении обновляет username.
func (d *Database) AddDynamicAdmin(userID int64, username string, addedBy int64) error {
	if userID == 0 {
		return fmt.Errorf("invalid user_id")
	}
	_, err := d.db.Exec(`
		INSERT INTO dynamic_admins (user_id, username, added_by, added_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id) DO UPDATE
		  SET username = EXCLUDED.username,
		      added_by = EXCLUDED.added_by,
		      added_at = EXCLUDED.added_at
	`, userID, username, addedBy)
	if err != nil {
		return fmt.Errorf("add dynamic admin: %w", err)
	}
	return nil
}

// RemoveDynamicAdmin — удалить администратора. Возвращает (true, nil) если запись была.
func (d *Database) RemoveDynamicAdmin(userID int64) (bool, error) {
	res, err := d.db.Exec(`DELETE FROM dynamic_admins WHERE user_id = $1`, userID)
	if err != nil {
		return false, fmt.Errorf("remove dynamic admin: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListDynamicAdmins — список всех динамических администраторов.
func (d *Database) ListDynamicAdmins() ([]DynamicAdmin, error) {
	rows, err := d.db.Query(`
		SELECT user_id, COALESCE(username,''), added_by, added_at
		FROM dynamic_admins
		ORDER BY added_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list dynamic admins: %w", err)
	}
	defer rows.Close()
	var out []DynamicAdmin
	for rows.Next() {
		var a DynamicAdmin
		if err := rows.Scan(&a.UserID, &a.Username, &a.AddedBy, &a.AddedAt); err != nil {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// IsDynamicAdmin — проверяет, является ли пользователь динамическим администратором.
func (d *Database) IsDynamicAdmin(userID int64) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM dynamic_admins WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("is dynamic admin: %w", err)
	}
	return count > 0, nil
}

// LoadDynamicAdminIDs — загружает все ID динамических администраторов (для кэша).
func (d *Database) LoadDynamicAdminIDs() ([]int64, error) {
	rows, err := d.db.Query(`SELECT user_id FROM dynamic_admins`)
	if err != nil {
		return nil, fmt.Errorf("load dynamic admin ids: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

// FindUserIDByUsername — ищет user_id по username во всех чатах (для панели владельца).
func (d *Database) FindUserIDByUsername(username string) (int64, error) {
	username = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(username)), "@")
	if username == "" {
		return 0, nil
	}
	var id int64
	err := d.db.QueryRow(`
		SELECT user_id FROM training_state
		WHERE LOWER(username) = $1
		LIMIT 1
	`, username).Scan(&id)
	if err != nil {
		return 0, nil
	}
	return id, nil
}
