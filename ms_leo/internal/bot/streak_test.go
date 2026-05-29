package bot

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return d
}

func strPtr(s string) *string { return &s }

func TestComputeStreakDays(t *testing.T) {
	today := "2026-05-29"

	cases := []struct {
		name        string
		last        *string
		prevStreak  int
		wantStreak  int
		wantSameDay bool
	}{
		{"first ever training", nil, 0, 1, false},
		{"trained yesterday -> +1", strPtr("2026-05-28"), 4, 5, false},
		{"same day repeat keeps streak", strPtr("2026-05-29"), 6, 6, true},
		{"same day repeat with zero streak", strPtr("2026-05-29"), 0, 0, true},
		{"gap of 2 days resets to 1", strPtr("2026-05-27"), 9, 1, false},
		{"big gap resets to 1", strPtr("2026-01-01"), 100, 1, false},
		// Регрессия на расхождение путей: админ начислил стрик, но тренировок ещё не было
		// (last_training_date == nil). Стрик должен продолжаться, а не сбрасываться в 1.
		{"admin-granted streak, no training yet -> continues", nil, 10, 11, false},
		// Пустая строка трактуется так же, как nil.
		{"empty last date, no streak", strPtr(""), 0, 1, false},
		{"empty last date with streak continues", strPtr("   "), 3, 4, false},
		// Граница milestone: 6 -> 7 (первая ачивка).
		{"crossing into weekly milestone", strPtr("2026-05-28"), 6, 7, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotStreak, gotSame := ComputeStreakDays(c.last, c.prevStreak, mustDate(t, today))
			if gotStreak != c.wantStreak || gotSame != c.wantSameDay {
				t.Errorf("ComputeStreakDays(%v, %d) = (%d, %v), want (%d, %v)",
					c.last, c.prevStreak, gotStreak, gotSame, c.wantStreak, c.wantSameDay)
			}
		})
	}
}

// Стрик не должен ломаться на границе месяца/года (yesterday вычисляется через time, а не вычитанием строк).
func TestComputeStreakDays_DateBoundaries(t *testing.T) {
	// 1 марта; вчера = 28 февраля (2026 — не високосный).
	got, _ := ComputeStreakDays(strPtr("2026-02-28"), 5, mustDate(t, "2026-03-01"))
	if got != 6 {
		t.Errorf("month boundary: got %d, want 6", got)
	}
	// 1 января; вчера = 31 декабря прошлого года.
	got, _ = ComputeStreakDays(strPtr("2025-12-31"), 5, mustDate(t, "2026-01-01"))
	if got != 6 {
		t.Errorf("year boundary: got %d, want 6", got)
	}
	// Не вчера через границу года — сброс.
	got, _ = ComputeStreakDays(strPtr("2025-12-30"), 5, mustDate(t, "2026-01-01"))
	if got != 1 {
		t.Errorf("year boundary gap: got %d, want 1", got)
	}
}

func TestEffectiveStreakDays(t *testing.T) {
	cases := []struct {
		stored     int
		daysSince  int
		want       int
	}{
		{0, 0, 0},
		{420, -1, 420},  // дата неизвестна — показываем сохранённый
		{420, 0, 420},   // тренировался сегодня
		{420, 1, 420},   // вчера — ещё жив до полночи
		{420, 2, 0},     // пропущен день после последней — сгорел
		{420, 10, 0},
		{1, 2, 0},
	}
	for _, c := range cases {
		if got := EffectiveStreakDays(c.stored, c.daysSince); got != c.want {
			t.Errorf("EffectiveStreakDays(%d, %d) = %d, want %d", c.stored, c.daysSince, got, c.want)
		}
	}
}

func TestStreakSaveAttemptsMaxForLevel(t *testing.T) {
	cases := []struct {
		level int
		want  int
	}{
		{0, 1},  // защита от <1
		{-3, 1}, // защита от отрицательных
		{1, 1},
		{2, 2},
		{7, 7},
		{8, 7},   // кап
		{100, 7}, // кап
	}
	for _, c := range cases {
		if got := StreakSaveAttemptsMaxForLevel(c.level); got != c.want {
			t.Errorf("StreakSaveAttemptsMaxForLevel(%d) = %d, want %d", c.level, got, c.want)
		}
	}
}

func TestDaysWordForm(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "день"},
		{2, "дня"},
		{3, "дня"},
		{4, "дня"},
		{5, "дней"},
		{10, "дней"},
		{11, "дней"}, // особый случай 11-14
		{12, "дней"},
		{14, "дней"},
		{21, "день"},
		{22, "дня"},
		{25, "дней"},
		{100, "дней"},
		{101, "день"},
		{111, "дней"},
	}
	for _, c := range cases {
		if got := daysWordForm(c.n); got != c.want {
			t.Errorf("daysWordForm(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestCupsWordForm(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "кубок"},
		{2, "кубка"},
		{5, "кубков"},
		{11, "кубков"},
		{21, "кубок"},
		{42, "кубка"},
		{420, "кубков"},
	}
	for _, c := range cases {
		if got := cupsWordForm(c.n); got != c.want {
			t.Errorf("cupsWordForm(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

// MilestoneCups должен начислять бонус только при первой тренировке дня (EarnRewards) и точном совпадении порога.
func TestTrainingOutcome_MilestoneCups(t *testing.T) {
	cases := []struct {
		name string
		o    TrainingOutcome
		want int
	}{
		{"7-day milestone", TrainingOutcome{EarnRewards: true, NewStreakDays: 7}, 42},
		{"30-day milestone", TrainingOutcome{EarnRewards: true, NewStreakDays: 30}, 420},
		{"100-day milestone", TrainingOutcome{EarnRewards: true, NewStreakDays: 100}, 4200},
		{"non-milestone day", TrainingOutcome{EarnRewards: true, NewStreakDays: 8}, 0},
		{"milestone but no rewards (same day)", TrainingOutcome{EarnRewards: false, NewStreakDays: 7}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.o.MilestoneCups(); got != c.want {
				t.Errorf("MilestoneCups() = %d, want %d", got, c.want)
			}
		})
	}
}
