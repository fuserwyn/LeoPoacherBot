package sickleave

import (
	"fmt"
	"strings"

	"leo-bot/internal/domain"
	"leo-bot/internal/logger"
)

type AIClient interface {
	AnswerUserQuestion(question string, userContext string) (string, error)
}

type Evaluator struct {
	ai     AIClient
	logger logger.Logger
}

func NewEvaluator(ai AIClient, log logger.Logger) *Evaluator {
	return &Evaluator{
		ai:     ai,
		logger: log,
	}
}

func (e *Evaluator) Evaluate(text string, messageLog *domain.MessageLog) bool {
	clean := strings.TrimSpace(strings.ToLower(text))
	clean = strings.ReplaceAll(clean, "#sick_leave", "")
	clean = strings.ReplaceAll(clean, "#sickleave", "")
	clean = strings.ReplaceAll(clean, "#healthy", "")
	clean = strings.ReplaceAll(clean, "#здоров", "")

	heuristicsApprove, hasNegative := EvaluateHeuristics(clean)
	if heuristicsApprove {
		return true
	}
	if hasNegative {
		return false
	}
	if e.ai == nil || clean == "" {
		return false
	}

	var ctxBuilder strings.Builder
	ctxBuilder.WriteString("Оцени убедительность больничного запроса.\n")
	if messageLog != nil {
		ctxBuilder.WriteString(fmt.Sprintf("Пользователь: %s\n", messageLog.Username))
		ctxBuilder.WriteString(fmt.Sprintf("StreakDays: %d\n", messageLog.StreakDays))
		ctxBuilder.WriteString(fmt.Sprintf("CalorieStreakDays: %d\n", messageLog.CalorieStreakDays))
		ctxBuilder.WriteString(fmt.Sprintf("HasSickLeave: %t\n", messageLog.HasSickLeave))
		ctxBuilder.WriteString(fmt.Sprintf("HasHealthy: %t\n", messageLog.HasHealthy))
	}
	ctxBuilder.WriteString(fmt.Sprintf("Текст запроса: \"%s\"\n", clean))
	ctxBuilder.WriteString("Эвристика не нашла явных признаков ни болезни, ни обмана.\n")

	question := "Если сообщение описывает реальную болезнь, ответь строго словом APPROVE. " +
		"Если это похоже на отговорку (работа, дела, лень и т.п.), ответь строго словом REJECT. " +
		"Никаких других слов или пояснений."

	answer, err := e.ai.AnswerUserQuestion(question, ctxBuilder.String())
	if err != nil {
		e.logger.Errorf("AI sick leave evaluation failed: %v", err)
		return false
	}

	normalized := strings.ToUpper(strings.TrimSpace(answer))
	switch {
	case strings.Contains(normalized, "APPROVE"):
		return true
	case strings.Contains(normalized, "REJECT"):
		return false
	default:
		return false
	}
}
