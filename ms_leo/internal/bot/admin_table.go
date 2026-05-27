package bot

import (
	"strings"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// adminTable — простая фиксированная таблица для Telegram (<pre> + моноширинный шрифт).
type adminTable struct {
	headers []string
	widths  []int
	rows    [][]string
}

func newAdminTable(headers []string, widths []int) *adminTable {
	if len(headers) != len(widths) {
		panic("adminTable: headers and widths length mismatch")
	}
	return &adminTable{headers: headers, widths: widths}
}

func (t *adminTable) addRow(cells ...string) {
	if len(cells) != len(t.headers) {
		return
	}
	row := make([]string, len(cells))
	for i, c := range cells {
		row[i] = adminTableCell(c, t.widths[i])
	}
	t.rows = append(t.rows, row)
}

func (t *adminTable) render() string {
	if len(t.headers) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(adminTableRow(t.headers, t.widths))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", t.totalWidth()))
	b.WriteString("\n")
	for _, row := range t.rows {
		b.WriteString(adminTableRow(row, t.widths))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (t *adminTable) totalWidth() int {
	n := 0
	for i, w := range t.widths {
		n += w
		if i < len(t.widths)-1 {
			n++ // пробел между колонками
		}
	}
	return n
}

func adminTableRow(cells []string, widths []int) string {
	parts := make([]string, len(cells))
	for i, c := range cells {
		parts[i] = adminPadRunes(c, widths[i])
	}
	return strings.Join(parts, " ")
}

func adminTableCell(s string, width int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return adminTruncateRunes(s, width)
}

func adminPadRunes(s string, width int) string {
	s = adminTruncateRunes(s, width)
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func adminTruncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

func adminEscapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func (b *Bot) sendAdminHTMLPreTable(chatID int64, title, subtitle, table string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	var body strings.Builder
	body.WriteString("<b>")
	body.WriteString(adminEscapeHTML(title))
	body.WriteString("</b>")
	if subtitle != "" {
		body.WriteString("\n")
		body.WriteString(adminEscapeHTML(subtitle))
	}
	if table != "" {
		body.WriteString("\n<pre>")
		body.WriteString(adminEscapeHTML(table))
		body.WriteString("</pre>")
	}
	msg := tgbotapi.NewMessage(chatID, body.String())
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true
	if keyboard != nil {
		msg.ReplyMarkup = keyboard
	}
	b.api.Send(msg)
}
