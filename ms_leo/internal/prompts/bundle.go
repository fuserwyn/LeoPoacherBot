package prompts

import (
	_ "embed"
	"strings"
)

// Bundle — промпты персонажа Лео для OpenRouter и бота (canonical fat_leopard_prompts v1.3).
type Bundle struct {
	DailySummary            string
	MonthlySummary          string
	AnswerUserQuestion      string // Промт 3: личка
	DailyWisdomTraining     string
	DailyWisdomLangRule     string
	DailyWisdomUserTemplate string
	TrainingChatSuffix      string
	CriticalTimerQuestion   string // legacy alias day_7 burn
	WarningTimerQuestion    string // Промт 4: burn alert
	AchievementMilestone    string // Промт 5
	TrainingEvaluation      string // Промт 1: оценка тренировки (JSON)
	TrainingFeedEncouragement string // комментарий Лео в тред ленты
	PackFeedParticipantRemoved string
}

//go:embed data/daily_summary.txt
var embeddedDailySummary string

//go:embed data/monthly_summary.txt
var embeddedMonthlySummary string

//go:embed data/answer_user_question_body.txt
var embeddedAnswerUserQuestionBody string

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
var embeddedWarningTimerQuestionBody string

//go:embed data/achievement_milestone.txt
var embeddedAchievementMilestoneBody string

//go:embed data/training_evaluation.txt
var embeddedTrainingEvaluationBody string

//go:embed data/training_feed_encouragement.txt
var embeddedTrainingFeedEncouragementBody string

//go:embed data/pack_feed_removed.txt
var embeddedPackFeedParticipantRemoved string

// DefaultBundle возвращает встроенные тексты из каталога data/ (v1.3).
func DefaultBundle() Bundle {
	return Bundle{
		DailySummary:               embeddedDailySummary,
		MonthlySummary:             embeddedMonthlySummary,
		AnswerUserQuestion:         ComposeSystemPrompt(embeddedAnswerUserQuestionBody, true),
		DailyWisdomTraining:        embeddedDailyWisdomTraining,
		DailyWisdomLangRule:        embeddedDailyWisdomLangRule,
		DailyWisdomUserTemplate:    embeddedDailyWisdomUserTemplate,
		TrainingChatSuffix:         embeddedTrainingChatSuffix,
		CriticalTimerQuestion:      ComposeSystemPrompt(embeddedCriticalTimerQuestion, false),
		WarningTimerQuestion:       ComposeSystemPrompt(embeddedWarningTimerQuestionBody, false),
		AchievementMilestone:       ComposeSystemPrompt(embeddedAchievementMilestoneBody, false),
		TrainingEvaluation:         ComposeSystemPrompt(embeddedTrainingEvaluationBody, false),
		TrainingFeedEncouragement:  ComposeSystemPrompt(embeddedTrainingFeedEncouragementBody, false),
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
