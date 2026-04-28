package bot

// MiniappHealthStatus — на больничном ли пользователь сейчас.
// Источник правды — pack-row (chat_id = PACK_CHAT_ID), т.к. sick_leave.go ведёт стейт именно там.
// Для старых/переходных данных оставляем fallback на private-row (chat_id = userID).
func (b *Bot) MiniappHealthStatus(userID int64) bool {
	if b == nil || b.db == nil || userID == 0 {
		return false
	}
	if b.config != nil && b.config.PackChatID != 0 {
		if ml, err := b.db.GetMessageLog(userID, b.config.PackChatID); err == nil && ml != nil {
			return ml.HasSickLeave && !ml.HasHealthy
		}
	}
	ml, err := b.db.GetMessageLog(userID, userID)
	if err != nil || ml == nil {
		return false
	}
	return ml.HasSickLeave && !ml.HasHealthy
}
