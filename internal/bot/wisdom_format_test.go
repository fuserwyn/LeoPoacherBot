package bot

import (
	"strings"
	"testing"
)

func TestSanitizeDailyWisdom(t *testing.T) {
	raw := `Сегодняшний фокус — баланс.

Сила не только в движении, но и в умении замедлиться. Перед рывком — проверь дыхание.

Разгони туман в голове короткой разминкой. Затем — шаг за шагом. Ты готов.

(Строго, но с теплом: это не приказ, а напоминание о твоей собственной власти над днём.)`

	out := sanitizeDailyWisdom(raw)
	if strings.Contains(out, "(") || strings.Contains(out, ")") {
		t.Fatalf("parentheses remain: %q", out)
	}
	if strings.Contains(strings.ToLower(out), "ты готов") {
		t.Fatalf("unwanted closing: %q", out)
	}
	if strings.Contains(strings.ToLower(out), "строго, но") {
		t.Fatalf("meta aside remains: %q", out)
	}
}
