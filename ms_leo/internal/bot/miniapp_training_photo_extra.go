package bot

import (
	"strings"
)

func (b *Bot) clearMiniappTrainingPhotoURL(userID int64) {
	if b == nil || userID == 0 {
		return
	}
	b.miniappTrainingPhotoURL.Delete(userID)
}

func (b *Bot) setMiniappTrainingPhotoURL(userID int64, rawURL string) {
	if b == nil || userID == 0 {
		return
	}
	u := strings.TrimSpace(rawURL)
	if u == "" {
		b.miniappTrainingPhotoURL.Delete(userID)
		return
	}
	b.miniappTrainingPhotoURL.Store(userID, u)
}

func (b *Bot) takeMiniappTrainingPhotoURL(userID int64) string {
	if b == nil || userID == 0 {
		return ""
	}
	v, ok := b.miniappTrainingPhotoURL.Load(userID)
	if !ok {
		return ""
	}
	b.miniappTrainingPhotoURL.Delete(userID)
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}
