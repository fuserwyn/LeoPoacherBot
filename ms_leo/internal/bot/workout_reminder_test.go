package bot

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWorkoutReminderText_ShortSingleLine(t *testing.T) {
	const wantBody = "не забудь внести сегодняшнюю тренировку в мини-аппе"

	got := workoutReminderText("")
	if got != "🐆 "+wantBody {
		t.Fatalf("without username = %q, want %q", got, "🐆 "+wantBody)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("expected single line, got %q", got)
	}
	if strings.HasSuffix(got, "!") {
		t.Fatalf("expected no exclamation mark, got %q", got)
	}
	for _, banned := range []string{"стрик", "Открой мини-апп", "💪"} {
		if strings.Contains(got, banned) {
			t.Fatalf("unexpected %q in %q", banned, got)
		}
	}
}

func TestWorkoutReminderText_WithUsername(t *testing.T) {
	got := workoutReminderText("mike")
	want := "🐆 @mike, не забудь внести сегодняшнюю тренировку в мини-аппе"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWorkoutReminderText_FitsTelegramPreview(t *testing.T) {
	longUser := strings.Repeat("a", 30)
	got := workoutReminderText(longUser)
	if n := utf8.RuneCountInString(got); n > 120 {
		t.Fatalf("reminder too long for a short DM: %d runes in %q", n, got)
	}
}
