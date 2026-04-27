package bot

// MiniappHealthStatus — на больничном ли пользователь сейчас.
// Состояние больничного хранится в message_log, привязанном к chat_id личной переписки бота с пользователем
// (так #sick_leave / #healthy отрабатывают как из TG-лички, так и из мини-аппа через dispatchTextMessageFromUser).
func (b *Bot) MiniappHealthStatus(userID int64) bool {
	if b == nil || b.db == nil || userID == 0 {
		return false
	}
	ml, err := b.db.GetMessageLog(userID, userID)
	if err != nil || ml == nil {
		return false
	}
	return ml.HasSickLeave && !ml.HasHealthy
}
