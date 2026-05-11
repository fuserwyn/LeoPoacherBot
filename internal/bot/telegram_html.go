package bot

import (
	"html"
	"strings"
	"unicode"
)

func isLikelyFenceLanguageLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || len(line) > 24 {
		return false
	}
	for _, r := range line {
		if unicode.IsSpace(r) {
			return false
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '+', '-', '#', '.', '_':
			continue
		default:
			return false
		}
	}
	return true
}

// renderAIMarkdownAsTelegramHTML превращает ответ ИИ в Telegram HTML: обычный текст экранируется,
// блоки ```lang ... ``` — в <pre><code> (в клиенте Telegram у блока есть кнопка копирования).
func renderAIMarkdownAsTelegramHTML(ai string) string {
	ai = strings.TrimSpace(ai)
	if ai == "" {
		return ""
	}
	if !strings.Contains(ai, "```") {
		return html.EscapeString(ai)
	}

	var b strings.Builder
	raw := ai
	for {
		start := strings.Index(raw, "```")
		if start == -1 {
			b.WriteString(html.EscapeString(raw))
			break
		}
		b.WriteString(html.EscapeString(raw[:start]))
		rest := raw[start+3:]
		end := strings.Index(rest, "```")
		if end == -1 {
			b.WriteString(html.EscapeString("```"))
			b.WriteString(html.EscapeString(rest))
			break
		}
		codeBody := strings.TrimLeft(rest[:end], "\n\r")
		lines := strings.Split(codeBody, "\n")
		if len(lines) >= 2 && isLikelyFenceLanguageLine(lines[0]) {
			codeBody = strings.Join(lines[1:], "\n")
		}
		codeBody = strings.TrimRight(codeBody, "\n\r\t ")
		b.WriteString("<pre><code>")
		b.WriteString(html.EscapeString(codeBody))
		b.WriteString("</code></pre>")
		raw = rest[end+3:]
	}
	return b.String()
}

// composeCodingReportTelegramHTML — статистика (plain) + HTML-рендер приписки ИИ.
func composeCodingReportTelegramHTML(headerPlain string, aiMarkdown string) string {
	headerPlain = strings.TrimRight(headerPlain, "\n\r\t ")
	aiMarkdown = strings.TrimSpace(aiMarkdown)
	if aiMarkdown == "" {
		return html.EscapeString(headerPlain)
	}
	return html.EscapeString(headerPlain) + "\n\n" + renderAIMarkdownAsTelegramHTML(aiMarkdown)
}
