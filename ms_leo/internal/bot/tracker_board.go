func trackerStatusMeta(status, col string) (label, icon, phase string) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return "В работе", "🔧", "doing"
	case "reviewing":
		return "Review", "👀", "review"
	case "holding":
		if col == trackerColTest {
			return "Тест", "🧪", "test"
		}
		return "Сборка", "🚀", "deploy"
	case "done", "completed":
		return "Выполнено", "✅", "done"
	case "canceled", "cancelled":
		return "Отменено", "⛔", "canceled"
	case "error":
		return "Ошибка", "⚠️", "todo"
	default:
		return "Ожидает", "⏳", "todo"
	}
}