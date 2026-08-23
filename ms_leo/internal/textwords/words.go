package textwords

import (
	"regexp"
	"strings"
)

var wordLetters = regexp.MustCompile(`[^\p{L}\p{N}_]+`)

// LowerWords режет текст на нижнерегистровые слова (буквы/цифры), без знаков пунктуации.
func LowerWords(s string) []string {
	clean := wordLetters.ReplaceAllString(strings.TrimSpace(s), " ")
	if clean == "" {
		return nil
	}
	parts := strings.Fields(clean)
	out := parts[:0]
	for _, p := range parts {
		out = append(out, strings.ToLower(p))
	}
	return out
}

// ContainsAnyWord сообщает, совпало ли хотя бы одно целое слово текста со списком.
func ContainsAnyWord(text string, needles []string) bool {
	haystack := LowerWords(text)
	wset := make(map[string]struct{}, len(haystack))
	for _, w := range haystack {
		wset[w] = struct{}{}
	}
	for _, n := range needles {
		n = strings.TrimSpace(strings.ToLower(n))
		if n == "" {
			continue
		}
		if _, ok := wset[n]; ok {
			return true
		}
	}
	return false
}
