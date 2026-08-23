package bot

import "strings"

const miniappMediaPathPrefix = "/api/miniapp/media/"

// canonicalMiniappTrainingPhotoURL подставляет MINIAPP_PUBLIC_BASE_URL вместо устаревшего origin в БД
// (127.0.0.1, прошлый домен деплоя), сохраняя путь /api/miniapp/media/<file>.
func (b *Bot) canonicalMiniappTrainingPhotoURL(stored string) string {
	stored = strings.TrimSpace(stored)
	if stored == "" || b == nil || b.config == nil {
		return stored
	}
	idx := strings.Index(stored, miniappMediaPathPrefix)
	if idx < 0 {
		return stored
	}
	base := strings.TrimRight(strings.TrimSpace(b.config.MiniappPublicBaseURL), "/")
	if base == "" {
		return stored
	}
	return base + stored[idx:]
}
