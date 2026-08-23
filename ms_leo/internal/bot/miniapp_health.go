package bot

// MiniappHealthStatus — на больничном ли пользователь сейчас (training_state по стае).
func (b *Bot) MiniappHealthStatus(userID int64) bool {
	if b == nil || b.db == nil || userID == 0 {
		return false
	}
	if b.config != nil && b.config.MonetizedChatID != 0 {
		if ml, err := b.db.GetMessageLog(userID, b.config.MonetizedChatID); err == nil && ml != nil {
			return ml.HasSickLeave && !ml.HasHealthy
		}
		return false
	}
	ml, err := b.db.GetMessageLog(userID, userID)
	if err != nil || ml == nil {
		return false
	}
	return ml.HasSickLeave && !ml.HasHealthy
}
