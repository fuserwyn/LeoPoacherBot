package ai

import (
	"regexp"
	"strings"
)

// Преамбулы, которые модели иногда добавляют перед смысловым текстом.
var reUserRequestAnswerHeader = regexp.MustCompile(`(?is)^\s*(?:ответ\s+на\s+запрос\s+пользователя|ответ\s+пользователю\s+на\s+запрос)\s*:\s*`)

// Заголовки вида «### Ответ на сообщение …» и чистые markdown-заголовки в начале строки.
var reMarkdownHeadingLine = regexp.MustCompile(`(?m)^[ \t]*#{1,6}\s.*$`)
var reAnswerOnMessageLine = regexp.MustCompile(`(?mi)^[ \t]*(?:#{1,6}\s*)?(?:ответ\s+на\s+сообщение|ответ\s+на\s+запрос)[^\n]*$`)

// Строка целиком в виде *реплика* / *Рычит* (и опц. эмодзи) — сценарная ремарка, убираем.
var reStarOnlyLine = regexp.MustCompile(`(?m)^[ \t]*\*[^*]{1,200}\*(?:[ \t]*[🐆🦁🐯])?[ \t]*\r?$`)

// Ведущая «театральная» ремарка в круглых скобках одной строкой или абзацем (модели всё равно добавляют).
var reLeadingParenStageBlock = regexp.MustCompile(`(?s)^[ \t]*\([^)]{2,500}\)[ \t]*(?:\n{1,2}|$)`)

// Хвостовая саморефлексия модели в скобках (stage=, символы, «выбрал тон») — не для пользователя.
var reTrailingAIMetaParen = regexp.MustCompile(`(?is)[\s\n]*\([^)]*(?:stage\s*=|уложил(?:ся|ась)?\s+в\s+\d+|нейтральн\w*\s+тон|выбрал\w*|запрещённ|без\s+запрещённ|для\s+stage)[^)]{0,400}\)\s*$`)

// *Рычит* / *р-р* / *мр-я* (внутри фразы)
var reStarInlineGimmicks = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:^|[\s,.!?:;—–-]|\(|\)|«|»|])\*рыч(ит|а|у|н(ул|ёшь|ит|ито)?|ь|о)\*(\s*[🐆🦁🐯]?\s*)?`),
	regexp.MustCompile(`(?i)(?:^|[\s,.!?:;—–-]|\(|\)|«|»|])\*рык\*(\s*[🐆🦁🐯❗!?.…]*)?`),
	regexp.MustCompile(`(?i)(?:^|[\s,.!?:;—–-]|\(|\)|«|»|])\*р+[-\s/]?р+[^*]{0,8}\*(\s*[🐆🦁🐯]?\s*)?`),
	regexp.MustCompile(`(?i)(?:^|[\s,.!?:;—–-]|\(|\)|«|»|])\*мр+я+[^*]{0,4}\*(\s*[🐆🦁🐯]?\s*)?`),
}

func stripMarkdownHeadingAndAnswerLines(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			out = append(out, "")
			continue
		}
		if reMarkdownHeadingLine.MatchString(line) || reAnswerOnMessageLine.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.ReplaceAll(strings.Join(out, "\n"), "\n\n\n", "\n\n"))
}

func stripLeadingParenStageBlocks(s string) string {
	s = strings.TrimSpace(s)
	for i := 0; i < 5; i++ {
		next := strings.TrimSpace(reLeadingParenStageBlock.ReplaceAllString(s, ""))
		if next == s || next == "" {
			break
		}
		s = next
	}
	return strings.TrimSpace(s)
}

func stripAsteriskStageRemarks(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	s = reStarOnlyLine.ReplaceAllString(s, "\n")
	for _, p := range reStarInlineGimmicks {
		s = p.ReplaceAllString(s, " ")
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(strings.Join(strings.Fields(line), " "))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// SanitizeTextForUser удаляет служебные артефакты/утечки промптов из AI-текста перед отправкой пользователю.
func SanitizeTextForUser(text string) string {
	clean := strings.TrimSpace(strings.ReplaceAll(text, "**", ""))
	clean = reUserRequestAnswerHeader.ReplaceAllString(clean, "")
	clean = strings.TrimSpace(clean)
	clean = stripMarkdownHeadingAndAnswerLines(clean)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return ""
	}

	clean = stripLeadingParenStageBlocks(clean)
	for i := 0; i < 3; i++ {
		next := strings.TrimSpace(reTrailingAIMetaParen.ReplaceAllString(clean, ""))
		if next == clean {
			break
		}
		clean = next
	}

	lower := strings.ToLower(clean)
	blockedStarts := []string{
		"сделай ",
		"напиши ",
		"дай ",
		"отвечай ",
		"вопрос пользователя:",
	}
	for _, s := range blockedStarts {
		if strings.HasPrefix(lower, s) {
			return ""
		}
	}

	blockedContains := []string{
		"без markdown",
		"1–2 предложения",
		"1-2 предложения",
		"приписку к предупреждению",
		"критическому предупреждению",
		"не повторяй цифры и факты",
		"если не отправит отчёт о тренировке в мини-аппе",
		"stage=day_",
		"stage = day",
		"уложился в",
		"уложилась в",
		"нейтральный тон для stage",
		"выбрал нейтральный",
		"запрещённых слов",
		"без запрещённых",
		"для stage=",
	}
	for _, s := range blockedContains {
		if strings.Contains(lower, s) {
			return ""
		}
	}

	clean = stripAsteriskStageRemarks(clean)
	if strings.TrimSpace(clean) == "" {
		return ""
	}

	return clean
}
