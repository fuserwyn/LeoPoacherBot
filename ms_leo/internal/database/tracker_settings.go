package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// Настройки доски: ключ-значение рядом с карточками. Лежат там же, где сама
// доска (отдельная БД трекера или база Лео, если второй нет), поэтому
// переживают редеплой сервиса — в отличие от переменных окружения, которые
// админ из мини-аппа поменять не может.

// GetTrackerSetting — значение настройки. Нет строки — пустая строка без ошибки.
func (d *Database) GetTrackerSetting(key string) (string, error) {
	if d == nil || d.trackerDB() == nil {
		return "", fmt.Errorf("база недоступна")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("пустой ключ настройки")
	}
	var value string
	err := d.trackerDB().QueryRow(
		`SELECT value FROM pack_tracker_settings WHERE key = $1`, key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetTrackerSetting — записать настройку от имени админа.
func (d *Database) SetTrackerSetting(key, value string, by int64) error {
	if d == nil || d.trackerDB() == nil {
		return fmt.Errorf("база недоступна")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("пустой ключ настройки")
	}
	var updatedBy any
	if by > 0 {
		updatedBy = by
	}
	_, err := d.trackerDB().Exec(`
		INSERT INTO pack_tracker_settings (key, value, updated_by, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
	`, key, strings.TrimSpace(value), updatedBy)
	return err
}
