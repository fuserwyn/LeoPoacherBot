package prompts

import (
	"fmt"
	"strings"

	"leo-bot/internal/moderation"
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

// GameRulesBlock — краткие правила для секции «Правила / политики».
func GameRulesBlock() string {
	return strings.TrimSpace(`Fat Leopard / стая:
• Отчёт о тренировке — #training_done в чате или форма «+» в мини-аппе
• #sick_leave — пауза таймера; #healthy — возврат с тем же остатком дедлайна
• Таймер неактивности ~7 дней; предупреждения перед удалением из чата
• Кубки за отчёты по формуле; стрик — дни подряд с тренировкой; ачивки на порогах стрика`)
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
	fmt.Fprintf(&b, "%s (id=%d) спрашивает:\n", name, p.InterlocutorID)
	b.WriteString(WrapUserQuestion(p.Question))
	return b.String()
}

// WrapUserQuestion — anti-injection обёртка вопроса пользователя.
func WrapUserQuestion(question string) string {
	wrapped := moderation.WrapUserContent("user_question", question)
	if wrapped == "" {
		return emptyBlock
	}
	return wrapped
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
