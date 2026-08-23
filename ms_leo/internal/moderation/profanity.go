package moderation

import (
	"regexp"
	"strings"
)

// Стемы RU+EN для F1; расширяется без смены логики gate.
var profanityStems = []string{
	// RU
	"бля", "бляд", "блят", "сука", "суки", "суку", "хуй", "хуя", "хуе", "хую", "хуи", "пизд", "еба", "ёба",
	"ебл", "ёбл", "ебан", "ёбан", "ебат", "ёбат", "ебу", "ёбу", "ебё", "мудак", "мудил", "мраз", "гандон",
	"гондон", "пидор", "пидар", "педик", "шлюх", "сучк", "ублюд", "долбо", "дебил", "чмо", "залуп", "манда",
	"хер ", "хрен", "падл", "гавн", "срать", "срал", "жоп", "очк", "соси", "сосать",
	// EN
	"fuck", "fuk", "shit", "bitch", "asshole", "bastard", "cunt", "dick", "whore", "slut", "nigger", "nigga",
	"motherf", "bollock", "wanker",
}

var profanityPatterns []*regexp.Regexp

func init() {
	for _, stem := range profanityStems {
		stem = strings.TrimSpace(strings.ToLower(stem))
		if stem == "" {
			continue
		}
		// Границы слова для кириллицы и латиницы.
		pat := `(?i)(?:^|[^\p{L}])` + regexp.QuoteMeta(stem) + `[a-zA-Zа-яА-ЯёЁ]*(?:[^\p{L}]|$)`
		if re, err := regexp.Compile(pat); err == nil {
			profanityPatterns = append(profanityPatterns, re)
		}
	}
}

func containsProfanity(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, re := range profanityPatterns {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}
