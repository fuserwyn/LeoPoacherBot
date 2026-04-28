package sickleave

import (
	"strings"

	"leo-bot/internal/textwords"
)

// Оценка запроса больничного: эвристики без модели ИИ + явные слова-паразиты («работа без болезни», лень → отказ целых слов/фраз).
var (
	positiveStemSubstrings = []string{
		"болен", "болею", "болит", "заболел", "заболела", "забол", "заболева",
		"простыл", "простуд", "температур", "кашля", "кашель", "грипп", "орви", "ангин",
		"плохо", "лежу", "честно", "правда", "шанс", "выздоров", "выздоравли", "таблет", "врач",
		"болезн", "недомог", "жар",
		"сон", "боляч", "мигрен", "лихорад", "fever", "flu", "cold", "ill", "sick",
		// ЖКТ / острое (частые причины — раньше «понос», «диарея», «поносило» падали через ИИ только)
		"понос", "диаре", "поносило", "рвота", "тошни", "кишечник", "желуд", "абдомин", "жкт", "кишечн",
		"абсцесс",
	}

	supportStemSubstrings = []string{
		"дай шанс", "прошу", "пожалуйста", "исправлюсь", "буду тренироваться",
		"честно-честно", "умоляю", "верь", "поверь", "обещаю",
	}

	negativeWholeWords = []string{
		"делами", "работаю", "работа", "работе", "работой", "прогулку", "лень",
		// «прогул»: целым словом (гулять вместо отчёта), не режем случайные суффиксы через короткий «работ»
		"прогул",
		"прогулялся", "прогулялась",
	}

	negativeSubstringsFlexible = []string{
		"work", "workout", "воркаут",
		"обман", "схитрить", "хитр",
		"занят", "занята",
	}

	negativePhraseSubstrings = []string{
		"просто не", "не хочу", "другие дела",
	}
)

// EvaluateHeuristics — true/false по эвристикам; hasNegative — явный отказ без вызова ИИ.
func EvaluateHeuristics(text string) (approved bool, hasNegative bool) {
	if text == "" {
		return false, false
	}
	lower := strings.ToLower(text)

	for _, ph := range negativePhraseSubstrings {
		if ph != "" && strings.Contains(lower, ph) {
			return false, true
		}
	}
	if textwords.ContainsAnyWord(lower, negativeWholeWords) {
		return false, true
	}
	for _, neg := range negativeSubstringsFlexible {
		if neg != "" && strings.Contains(lower, neg) {
			return false, true
		}
	}

	score := 0
	for _, pos := range positiveStemSubstrings {
		if pos != "" && strings.Contains(lower, pos) {
			score++
		}
	}
	if strings.Contains(lower, "боле") {
		score++
	}
	if strings.Contains(lower, "забол") {
		score++
	}
	if strings.Contains(lower, "простуд") {
		score++
	}
	if strings.Contains(lower, "температ") {
		score++
	}
	if strings.Contains(lower, "кашл") {
		score++
	}
	if strings.Contains(lower, "плохое самочувствие") {
		score++
	}
	for _, sup := range supportStemSubstrings {
		if sup != "" && strings.Contains(lower, sup) {
			score++
		}
	}
	return score >= 1, false
}
