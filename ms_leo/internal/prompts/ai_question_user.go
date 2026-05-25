package prompts

import (
	"fmt"
	"strings"
)

// AIQuestionUserPayload — структурированный user-message для AnswerUserQuestion.
type AIQuestionUserPayload struct {
	InterlocutorName string
	InterlocutorID   int64
	UsersBlock       string
	RulesBlock       string
	FactsBlock       string
	ChatThread       string
	RouterHint       string
	Question         string
}

const emptyBlock = "— нет —"

// GameRulesBlock — краткие правила для секции «Правила / политики» (canonical wolf_pack_rules v1.0).
func GameRulesBlock() string {
	return strings.TrimSpace(`Стая / Fat Leopard (canonical v1.0):
• Тренировка — только через мини-апп («+»): тип, 1–480 мин, интенсивность 1–5, текстовая заметка; backdate нет
• Кубки: минимум 1 за любую тренировку; формула не раскрывается; сгорают при удалении из Стаи
• Стрик — дни подряд с тренировкой; попытки (1 старт +1 за уровень, макс. 7) спасают стрик, не удаление
• Неактивность: 7 дней без движения → удаление 00:00 дня 8 (часовой пояс пользователя); напоминания дни 5–7
• Больничный — кнопка в профиле, макс. 14 дней; таймер удаления на паузе; снять: «Я выздоровел» или тренировка
• Возврат после удаления: /start, 99₽; кубки/стрик/уровень с нуля; рекорд стрика и trophy case сохраняются
• Лео: до 20 сообщений/день; при сомнении в правилах приоритет у страницы «Правила Стаи» в мини-аппе`)
}

// FormatAIQuestionUserMessage — user-role prompt с секциями structured knowledge.
func FormatAIQuestionUserMessage(p AIQuestionUserPayload) string {
	name := strings.TrimSpace(p.InterlocutorName)
	if name == "" {
		name = "user"
	}
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}

	users := strings.TrimSpace(p.UsersBlock)
	if users == "" {
		users = emptyBlock
	}
	rules := strings.TrimSpace(p.RulesBlock)
	if rules == "" {
		rules = emptyBlock
	}
	facts := strings.TrimSpace(p.FactsBlock)
	if facts == "" {
		facts = emptyBlock
	}
	thread := strings.TrimSpace(p.ChatThread)
	if thread == "" {
		thread = "— нет недавних сообщений —"
	}

	var b strings.Builder
	b.WriteString("Контекст для этого ответа — ТОЛЬКО секции ниже. Не выдумывай факты вне них.\n\n")
	fmt.Fprintf(&b, "Текущий собеседник: %s (id=%d)\n\n", name, p.InterlocutorID)
	b.WriteString("## Структурированные знания\n\n")
	b.WriteString("### Пользователи / сущности\n")
	b.WriteString(users)
	b.WriteString("\n\n### Правила / политики\n")
	b.WriteString(rules)
	b.WriteString("\n\n### Факты / документы\n")
	b.WriteString(facts)
	b.WriteString("\n\n## Недавняя переписка (хронология, старые → новые)\n")
	b.WriteString(thread)
	if h := strings.TrimSpace(p.RouterHint); h != "" {
		b.WriteString("\n\n## Подсказка роутера (опционально)\n")
		b.WriteString(h)
	}
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "%s (id=%d) спрашивает: %s", name, p.InterlocutorID, strings.TrimSpace(p.Question))
	return b.String()
}

// FormatLegacyUserPrompt — обёртка для старых вызовов с плоским контекстом.
func FormatLegacyUserPrompt(question, flatContext string) string {
	facts := strings.TrimSpace(flatContext)
	if facts == "" {
		facts = emptyBlock
	}
	return FormatAIQuestionUserMessage(AIQuestionUserPayload{
		InterlocutorName: "user",
		UsersBlock:       emptyBlock,
		RulesBlock:       emptyBlock,
		FactsBlock:       facts,
		ChatThread:       "— нет недавних сообщений —",
		Question:         question,
	})
}

// RouterHintForQuestion — лёгкая подсказка модели по типу вопроса.
func RouterHintForQuestion(question string) string {
	q := strings.ToLower(strings.TrimSpace(question))
	if q == "" {
		return ""
	}
	switch {
	case strings.Contains(q, "кубк"), strings.Contains(q, "стрик"), strings.Contains(q, "streak"),
		strings.Contains(q, "статистик"), strings.Contains(q, "сколько"), strings.Contains(q, "таймер"),
		strings.Contains(q, "больнич"), strings.Contains(q, "sick_leave"), strings.Contains(q, "healthy"):
		return "Фактический вопрос — ответь по данным из structured knowledge, кратко и точно."
	case strings.Contains(q, "привет"), strings.Contains(q, "здаров"), q == "hi", q == "hello":
		return "Короткое приветствие — 1–2 фразы, без статистики и без морали про тренировки."
	case strings.Contains(q, "придумай"), strings.Contains(q, "выдумай"), strings.Contains(q, "фантаз"):
		return "Creative mode: можно выдумывать, лёгкий намёк на вымысел по желанию."
	default:
		if len([]rune(q)) < 36 && !strings.Contains(q, "?") {
			return "Короткая реплика — ответь по сути, без лишнего контекста из БД."
		}
	}
	return ""
}
