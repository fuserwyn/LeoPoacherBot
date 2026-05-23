package bot

import (
	"fmt"
	"strings"

	"leo-bot/internal/domain"
	"leo-bot/internal/prompts"
)

// aiQuestionSections — блоки structured knowledge для промпта Лео.
type aiQuestionSections struct {
	users  strings.Builder
	rules  strings.Builder
	facts  strings.Builder
	thread strings.Builder
}

func (s *aiQuestionSections) initRules() {
	if s.rules.Len() > 0 {
		return
	}
	s.rules.WriteString(prompts.GameRulesBlock())
}

func formatUserEntityLine(u *domain.MessageLog, cups int, remaining string) string {
	if u == nil {
		return ""
	}
	name := strings.TrimSpace(u.Username)
	if name == "" {
		name = fmt.Sprintf("id=%d", u.UserID)
	}
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}
	status := "✅ активен"
	switch {
	case u.HasSickLeave:
		status = "🏥 больничный"
	case u.HasHealthy:
		status = "💚 после #healthy"
	}
	line := fmt.Sprintf("👤 %s (id=%d) | 🏆 кубки: %d | 💪 стрик: %d %s | %s",
		name, u.UserID, cups, u.StreakDays, daysWordForm(u.StreakDays), status)
	if u.LastTrainingDate != nil && strings.TrimSpace(*u.LastTrainingDate) != "" {
		line += fmt.Sprintf(" | 📅 последняя: %s", strings.TrimSpace(*u.LastTrainingDate))
	}
	if remaining != "" {
		line += fmt.Sprintf(" | ⏳ %s", remaining)
	}
	g := strings.TrimSpace(strings.ToLower(u.Gender))
	if g == "m" {
		line += " | пол: м"
	} else if g == "f" {
		line += " | пол: ж"
	}
	return line
}

func formatTrainingFactLine(createdAt, username string, userID int64, messageType, text string) string {
	name := strings.TrimSpace(username)
	if name == "" {
		name = fmt.Sprintf("id=%d", userID)
	}
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}
	tag := strings.TrimSpace(messageType)
	switch tag {
	case "training_done":
		tag = "отчёт"
	case "sick_leave":
		tag = "больничный"
	case "healthy":
		tag = "выздоровление"
	case "ai_reply":
		tag = "ответ Лео"
	default:
		if tag == "" || tag == "general" {
			tag = "текст"
		}
	}
	t := strings.TrimSpace(text)
	if len(t) > 400 {
		t = t[:400] + "…"
	}
	return fmt.Sprintf("[%s] %s (id=%d) [%s]: %s", createdAt, name, userID, tag, t)
}
