package bot

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	wisdomParenRe     = regexp.MustCompile(`\([^)]*\)`)
	wisdomBracketRe   = regexp.MustCompile(`\[[^\]]*\]`)
	wisdomTyReadyRe   = regexp.MustCompile(`(?i)^ты\s+готов[а]?\s*[.!?…]*\s*$`)
	wisdomTyClosingRe = regexp.MustCompile(`(?i)^ты\s+(готов|можешь|справишься|в\s+силах|сделаешь)`)
	wisdomMetaSubstrRe    = regexp.MustCompile(`(?i)(строго,\s*но|это\s+не\s+приказ|напоминание\s+о|властью\s+над\s+дн|собственной\s+властью)`)
	wisdomImperativeRe    = regexp.MustCompile(`(?i)^(проверь|сделай|разгони|не\s+забудь|начни|остановись|вспомни|затем)\b`)
)

// sanitizeDailyWisdom убирает скобки, мета-пояснения и нежелательные обращения «ты…».
func sanitizeDailyWisdom(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	for i := 0; i < 12 && wisdomParenRe.MatchString(s); i++ {
		s = wisdomParenRe.ReplaceAllString(s, " ")
	}
	s = wisdomBracketRe.ReplaceAllString(s, " ")
	s = collapseWisdomSpaces(s)

	paras := strings.Split(s, "\n\n")
	var kept []string
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" || wisdomLooksLikeMetaAside(p) {
			continue
		}
		p = sanitizeWisdomParagraph(p)
		if p != "" {
			kept = append(kept, p)
		}
	}
	s = strings.TrimSpace(strings.Join(kept, "\n\n"))
	if s == "" {
		return s
	}

	sents := splitWisdomSentences(s)
	var out []string
	for _, sent := range sents {
		if wisdomSentenceDrop(sent) {
			continue
		}
		out = append(out, sent)
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

func sanitizeWisdomParagraph(p string) string {
	sents := splitWisdomSentences(p)
	var out []string
	for _, sent := range sents {
		if wisdomSentenceDrop(sent) {
			continue
		}
		out = append(out, sent)
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

func wisdomLooksLikeMetaAside(p string) bool {
	if strings.HasPrefix(strings.TrimSpace(p), "(") {
		return true
	}
	return wisdomMetaSubstrRe.MatchString(p)
}

func wisdomSentenceDrop(sent string) bool {
	sent = strings.TrimSpace(sent)
	if sent == "" {
		return true
	}
	if wisdomTyReadyRe.MatchString(sent) {
		return true
	}
	if wisdomTyClosingRe.MatchString(sent) {
		return true
	}
	if strings.HasPrefix(sent, "(") || strings.HasSuffix(sent, ")") && strings.Contains(sent, "(") {
		return true
	}
	if wisdomMetaSubstrRe.MatchString(sent) {
		return true
	}
	lower := strings.ToLower(sent)
	if strings.HasPrefix(lower, "ты ") || strings.Contains(lower, " твой ") || strings.Contains(lower, " тебе ") {
		return true
	}
	if wisdomImperativeRe.MatchString(sent) {
		return true
	}
	return false
}

func splitWisdomSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var sents []string
	var b strings.Builder
	flush := func() {
		s := strings.TrimSpace(b.String())
		b.Reset()
		if s != "" {
			sents = append(sents, s)
		}
	}
	for _, r := range text {
		b.WriteRune(r)
		if r == '.' || r == '!' || r == '?' || r == '…' {
			flush()
		}
	}
	flush()
	return sents
}

func collapseWisdomSpaces(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !space {
				b.WriteRune(' ')
				space = true
			}
			continue
		}
		space = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
