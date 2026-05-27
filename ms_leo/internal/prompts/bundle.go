package prompts

import (
	_ "embed"
	"strings"
)

// Bundle — промпты персонажа Fat Leopard для OpenRouter и бота (встроенные через //go:embed в data/).
type Bundle struct {
	DailySummary            string
	MonthlySummary          string
	AnswerUserQuestion      string
	DailyWisdomTraining     string
	DailyWisdomLangRule     string
	DailyWisdomUserTemplate string
	TrainingChatSuffix      string
	CriticalTimerQuestion   string
	WarningTimerQuestion    string // предупреждение день 5/6/7 до кика за неактивность (stage-aware)
	AchievementMilestone    string // milestone-ачивка: in_app_text + leo_message (Промт 5)
	TrainingEvaluation      string // комментарий после тренировки JSON (Промт 1)
	// PackFeedParticipantRemoved — карточка в ленте мини‑аппа, когда человек уже не видит её: текст для стаи.
	PackFeedParticipantRemoved string
}

//go:embed data/daily_summary.txt
var embeddedDailySummary string

//go:embed data/monthly_summary.txt
var embeddedMonthlySummary string

//go:embed data/answer_user_question.txt
var embeddedAnswerUserQuestion string

//go:embed data/daily_wisdom_training.txt
var embeddedDailyWisdomTraining string

//go:embed data/daily_wisdom_lang_rule.txt
var embeddedDailyWisdomLangRule string

//go:embed data/daily_wisdom_user_template.txt
var embeddedDailyWisdomUserTemplate string

//go:embed data/training_chat_suffix.txt
var embeddedTrainingChatSuffix string

//go:embed data/critical_timer_question.txt
var embeddedCriticalTimerQuestion string

//go:embed data/warning_timer_question.txt
var embeddedWarningTimerQuestion string

//go:embed data/achievement_milestone.txt
var embeddedAchievementMilestone string

//go:embed data/pack_feed_removed.txt
var embeddedPackFeedParticipantRemoved string

//go:embed data/training_evaluation.txt
var embeddedTrainingEvaluation string

// DefaultBundle возвращает встроенные тексты из каталога data/.
func DefaultBundle() Bundle {
	return Bundle{
		DailySummary:               embeddedDailySummary,
		MonthlySummary:             embeddedMonthlySummary,
		AnswerUserQuestion:         embeddedAnswerUserQuestion,
		DailyWisdomTraining:        embeddedDailyWisdomTraining,
		DailyWisdomLangRule:        embeddedDailyWisdomLangRule,
		DailyWisdomUserTemplate:    embeddedDailyWisdomUserTemplate,
		TrainingChatSuffix:         embeddedTrainingChatSuffix,
		CriticalTimerQuestion:      embeddedCriticalTimerQuestion,
		WarningTimerQuestion:       embeddedWarningTimerQuestion,
		AchievementMilestone:       embeddedAchievementMilestone,
		TrainingEvaluation:         embeddedTrainingEvaluation,
		PackFeedParticipantRemoved: embeddedPackFeedParticipantRemoved,
	}
}

// CombinedChatInstructionSuffix — добавка из training_chat_suffix.txt для ответа в чате.
func (b Bundle) CombinedChatInstructionSuffix() string {
	s := strings.TrimSpace(b.TrainingChatSuffix)
	if s == "" {
		return ""
	}
	return "\n\n" + s
}
