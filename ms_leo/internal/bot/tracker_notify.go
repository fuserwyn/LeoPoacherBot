package bot

import "strings"

func trackerNotifyKind(text string) string {
	low := strings.ToLower(text)
	switch {
	case strings.Contains(text, "TRACKER_NO_CODE") || strings.Contains(low, "кода нет") ||
		strings.Contains(low, "репозиторий не менялся"):
		return "plan"
	case strings.Contains(low, "отменен") || strings.Contains(low, "cancelled") || strings.Contains(low, "canceled"):
		return "canceled"
	case strings.Contains(low, "агент не стартовал") || strings.Contains(low, "openrouter"):
		return "error"
	case strings.Contains(low, "выполнен") || strings.Contains(low, "completed") ||
		strings.Contains(low, "готово") || strings.Contains(text, "✅") ||
		strings.Contains(low, "можно на тест") || strings.Contains(low, "тест пройден") ||
		strings.Contains(low, "ревью не принято") || strings.Contains(low, "тест не прошёл") ||
		strings.Contains(low, "тест не прошел") || strings.Contains(low, "донат"):
		return "done"
	case strings.Contains(low, "ошибк") || strings.Contains(low, "не удалось") || strings.Contains(low, "срыв"):
		return "error"
	case strings.Contains(low, "началась") || strings.Contains(low, "взяли в работу"):
		return "started"
	case strings.Contains(low, "донат"):
		return "donate"
	default:
		return ""
	}
}