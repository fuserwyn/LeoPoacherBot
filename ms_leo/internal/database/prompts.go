package database

import (
	"fmt"
	"strings"
	"time"
)

type PromptOverride struct {
	Key       string
	Body      string
	Filename  string
	UpdatedBy int64
	UpdatedAt time.Time
}

func (d *Database) ListPromptOverrides() (map[string]PromptOverride, error) {
	out := map[string]PromptOverride{}
	if d == nil || d.db == nil {
		return out, fmt.Errorf("база недоступна")
	}
	rows, err := d.db.Query(`
		SELECT key, body, filename, COALESCE(updated_by, 0), updated_at
		FROM leo_prompt_overrides
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p PromptOverride
		if err := rows.Scan(&p.Key, &p.Body, &p.Filename, &p.UpdatedBy, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out[p.Key] = p
	}
	return out, rows.Err()
}

func (d *Database) SavePromptOverride(key, body, filename string, by int64) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("база недоступна")
	}
	key = strings.TrimSpace(key)
	body = strings.TrimSpace(body)
	if key == "" || body == "" {
		return fmt.Errorf("нужны ключ и текст промпта")
	}
	filename = strings.TrimSpace(filename)
	_, err := d.db.Exec(`
		INSERT INTO leo_prompt_overrides (key, body, filename, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (key) DO UPDATE SET
			body = EXCLUDED.body,
			filename = EXCLUDED.filename,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
	`, key, body, filename, by)
	return err
}

func (d *Database) DeletePromptOverride(key string) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("база недоступна")
	}
	_, err := d.db.Exec(`DELETE FROM leo_prompt_overrides WHERE key = $1`, strings.TrimSpace(key))
	return err
}

func PromptOverrideMap(in map[string]PromptOverride) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v.Body
	}
	return out
}
