package bot

import (
	"strings"
	"unicode/utf8"
)

func looksLikeDailyWisdomSpam(text string) bool {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return false
	}
	if strings.Contains(low, "мудрость дня") {
		return true
	}
	if strings.Contains(low, "сегодняшний день") && (strings.Contains(low, "возможност") || strings.Contains(low, "осознан")) {
		return true
	}
	if strings.Contains(low, "гонись за скорост") || strings.Contains(low, "найди свой ритм") {
		return true
	}
	return paragraphLooksLikeLoop(text)
}

// sanitizeAIReply — шаг 6: обрезка петель и шаблонов.
func sanitizeAIReply(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "**", ""))
	if text == "" {
		return ""
	}
	if looksLikeDailyWisdomSpam(text) {
		return ""
	}
	return trimReplyLoops(text)
}

// shouldPersistAIReply — шаг 7: не индексировать плохие ответы в Qdrant/Postgres.
func shouldPersistAIReply(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if utf8.RuneCountInString(text) < 3 {
		return false
	}
	if looksLikeDailyWisdomSpam(text) {
		return false
	}
	if replyLooksLikeLoop(text) {
		return false
	}
	return true
}

func trimReplyLoops(text string) string {
	paras := strings.Split(text, "\n\n")
	var kept []string
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" || paragraphLooksLikeLoop(p) {
			continue
		}
		kept = append(kept, p)
	}
	if len(kept) == 0 {
		return strings.TrimSpace(text)
	}
	return strings.Join(kept, "\n\n")
}

func replyLooksLikeLoop(text string) bool {
	sents := splitWisdomSentences(text)
	if len(sents) < 2 {
		return false
	}
	seen := make(map[string]int)
	for _, s := range sents {
		k := normalizeContextKey(s)
		if len(k) < 16 {
			continue
		}
		seen[k]++
		if seen[k] >= 2 {
			return true
		}
	}
	return false
}

func paragraphLooksLikeLoop(p string) bool {
	low := strings.ToLower(p)
	banned := []string{
		"сегодняшний день — это возможность",
		"сегодня не гонись за скоростью",
		"найди свой ритм",
		"продуктивность придет сама",
	}
	for _, b := range banned {
		if strings.Contains(low, b) {
			return true
		}
	}
	return false
}
