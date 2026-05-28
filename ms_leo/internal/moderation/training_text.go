package moderation

import "strings"

// ExtractUGCText — заметка «Что сделал» из полного отчёта мини-аппа.
// Формат: «бег, 15 мин, инт. 3/5\n\nЖим…» → «Жим…».
func ExtractUGCText(full string) string {
	full = strings.TrimSpace(full)
	if full == "" {
		return ""
	}
	parts := strings.SplitN(full, "\n\n", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return full
}

// TextForTrainingModeration — что проверять PRE-фильтром в отчёте о тренировке.
func TextForTrainingModeration(full string) string {
	note := ExtractUGCText(full)
	if note != "" {
		return note
	}
	return strings.TrimSpace(full)
}
