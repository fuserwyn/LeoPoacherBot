package moderation

import (
	"regexp"
	"strings"
)

// Критичный список РФ (наркотики / экстремизм) — явные маркеры для PRE + алерт админу.
var criticalRUStems = []string{
	"героин", "метамфетамин", "метамф", "кокаин", "кокс ", "лсд", "mdma", "экстази", "марихуан", "каннабис",
	"закладк", "наркот", "спайс", "соль ", "амфетамин", "опиум", "фентанил", "кетамин", "псилоциб",
	"игил", "исис", "террорист", "теракт", "взрывчат", "убью ", "убить ", "расстрел", "экстремизм",
	"нацизм", "фашизм", "свастик",
}

var criticalRUPatterns []*regexp.Regexp

func init() {
	for _, stem := range criticalRUStems {
		stem = strings.TrimSpace(strings.ToLower(stem))
		if stem == "" {
			continue
		}
		pat := `(?i)(?:^|[^\p{L}])` + regexp.QuoteMeta(stem)
		if re, err := regexp.Compile(pat); err == nil {
			criticalRUPatterns = append(criticalRUPatterns, re)
		}
	}
}

func containsCriticalRU(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, re := range criticalRUPatterns {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}
