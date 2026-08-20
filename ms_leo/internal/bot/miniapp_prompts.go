package bot

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"leo-bot/internal/database"
	"leo-bot/internal/prompts"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

const promptOverrideMax = 120000

func (b *Bot) livePrompts() prompts.Bundle {
	base := prompts.DefaultBundle()
	if b == nil || b.db == nil {
		if b != nil && b.config != nil {
			return b.config.Prompts
		}
		return base
	}
	ov, err := b.db.ListPromptOverrides()
	if err != nil {
		return base
	}
	return prompts.ApplyOverrides(base, database.PromptOverrideMap(ov))
}

func (b *Bot) MiniappListLeoPrompts(viewerUserID int64, initD initdata.InitData) ([]map[string]any, error) {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return nil, err
	}
	ov := map[string]database.PromptOverride{}
	if b.db != nil {
		got, err := b.db.ListPromptOverrides()
		if err != nil {
			return nil, err
		}
		ov = got
	}
	out := make([]map[string]any, 0, len(prompts.Catalog()))
	for _, slot := range prompts.Catalog() {
		body := slot.Embedded()
		overridden := false
		filename := ""
		updated := ""
		if p, ok := ov[slot.Key]; ok && strings.TrimSpace(p.Body) != "" {
			body = p.Body
			overridden = true
			filename = p.Filename
			if !p.UpdatedAt.IsZero() {
				updated = p.UpdatedAt.Format("02.01 15:04")
			}
		}
		out = append(out, map[string]any{
			"key":        slot.Key,
			"file":       slot.File,
			"title":      slot.Title,
			"about":      slot.About,
			"body":       body,
			"builtin":    slot.Embedded(),
			"overridden": overridden,
			"filename":   filename,
			"updated_at": updated,
		})
	}
	return out, nil
}

func (b *Bot) MiniappSaveLeoPrompt(viewerUserID int64, initD initdata.InitData, key, body, filename string) error {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return err
	}
	if _, ok := prompts.SlotByKey(key); !ok {
		return fmt.Errorf("нет такого промпта")
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("файл или текст пустой")
	}
	if utf8.RuneCountInString(body) > promptOverrideMax {
		return fmt.Errorf("промпт слишком длинный")
	}
	if b.db == nil {
		return fmt.Errorf("база недоступна")
	}
	return b.db.SavePromptOverride(key, body, filename, viewerUserID)
}

func (b *Bot) MiniappResetLeoPrompt(viewerUserID int64, initD initdata.InitData, key string) error {
	if _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {
		return err
	}
	if _, ok := prompts.SlotByKey(key); !ok {
		return fmt.Errorf("нет такого промпта")
	}
	if b.db == nil {
		return fmt.Errorf("база недоступна")
	}
	return b.db.DeletePromptOverride(key)
}
