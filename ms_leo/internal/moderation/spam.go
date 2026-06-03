package moderation

import (
	"regexp"
	"strings"
)

var wordRe = regexp.MustCompile(`\p{L}+`)

func hasExcessiveWordRepetition(text string) bool {
	words := wordRe.FindAllString(strings.ToLower(text), -1)
	if len(words) == 0 {
		return false
	}

	wordCounts := make(map[string]int)
	for _, w := range words {
		if len(w) < 2 {
			continue
		}
		wordCounts[w]++
		if wordCounts[w] >= 3 {
			return true
		}
	}
	return false
}
