package bot

import (
	"testing"
	"time"

	"leo-bot/internal/domain"
	"leo-bot/internal/utils"
)

// Баг-сценарий: остаток 3 дня на старте больничного, болел месяц, вышел через #healthy
// → должно остаться ~3 дня (а раньше из-за рассинхрона флагов кикало сразу).
func TestInactivityKickDeadline_SickLeaveLeftoverPreserved(t *testing.T) {
	moscow := time.FixedZone("MSK", 3*3600)
	lastTraining := time.Date(2026, 6, 1, 12, 0, 0, 0, moscow)
	d0 := removalDeadlineLocal(lastTraining, 0)        // 9 июня 00:00 MSK
	sickStart := d0.Add(-3 * 24 * time.Hour)           // остаток = 3 дня
	sickEnd := sickStart.Add(30 * 24 * time.Hour)      // болел месяц
	now := sickEnd.Add(time.Minute)                    // только что #healthy

	ml := &domain.MessageLog{
		TimerStartTime:           strPtr(utils.FormatMoscowTime(lastTraining)),
		TimezoneOffsetFromMoscow: 0,
		HasSickLeave:             false,
		HasHealthy:               true,
		SickLeaveStartTime:       strPtr(utils.FormatMoscowTime(sickStart)),
		SickLeaveEndTime:         strPtr(utils.FormatMoscowTime(sickEnd)),
	}
	deadline, ok := inactivityKickDeadline(ml, now)
	if !ok {
		t.Fatal("expected ok=true")
	}
	remaining := deadline.Sub(now)
	if remaining < 2*24*time.Hour || remaining > 4*24*time.Hour {
		t.Errorf("expected ~3 days remaining after recovery, got %v", remaining)
	}
}

// Рассинхрон флагов (легаси-импорт ставит оба флага, end_time нет) не должен давать
// мгновенный кик по старому дедлайну — защитный грейс до ближайшей полуночи.
func TestInactivityKickDeadline_InconsistentFlagsNoImmediateKick(t *testing.T) {
	moscow := time.FixedZone("MSK", 3*3600)
	lastTraining := time.Date(2026, 5, 1, 12, 0, 0, 0, moscow) // D0 давно в прошлом
	sickStart := time.Date(2026, 5, 5, 12, 0, 0, 0, moscow)
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, moscow)

	ml := &domain.MessageLog{
		TimerStartTime:     strPtr(utils.FormatMoscowTime(lastTraining)),
		HasSickLeave:       true, // оба флага — рассинхрон
		HasHealthy:         true,
		SickLeaveStartTime: strPtr(utils.FormatMoscowTime(sickStart)),
		SickLeaveEndTime:   nil,
	}
	deadline, ok := inactivityKickDeadline(ml, now)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !deadline.After(now) {
		t.Errorf("expected future grace deadline, got %v (now %v) — immediate-kick regression", deadline, now)
	}
}

func TestRemovalDeadlineLocal(t *testing.T) {
	moscowLoc := time.FixedZone("MSK", 3*3600)

	cases := []struct {
		name        string
		lastTraining time.Time
		tzOffset    int // hours relative to Moscow
		wantDay     int // expected day-of-month of deadline (00:00 local)
		wantMonth   time.Month
		wantHour    int // must be 0 (midnight)
	}{
		{
			name:        "MSK noon → kick 9th at 00:00 MSK",
			lastTraining: time.Date(2024, 1, 1, 12, 0, 0, 0, moscowLoc),
			tzOffset:    0,
			wantDay:     9,
			wantMonth:   time.January,
			wantHour:    0,
		},
		{
			name:        "MSK 23:30 → still kicks 9th (same 7-day window)",
			lastTraining: time.Date(2024, 1, 1, 23, 30, 0, 0, moscowLoc),
			tzOffset:    0,
			wantDay:     9,
			wantMonth:   time.January,
			wantHour:    0,
		},
		{
			name:        "UTC+5 user (tzOffset=+2), training noon local → kicks 9th at 00:00 UTC+5",
			lastTraining: time.Date(2024, 1, 1, 12, 0, 0, 0, time.FixedZone("UTC+5", 5*3600)),
			tzOffset:    2, // UTC+5 = MSK+2
			wantDay:     9,
			wantMonth:   time.January,
			wantHour:    0,
		},
		{
			name:        "month boundary: Jan 25 training → kicks Feb 2",
			lastTraining: time.Date(2024, 1, 25, 10, 0, 0, 0, moscowLoc),
			tzOffset:    0,
			wantDay:     2,
			wantMonth:   time.February,
			wantHour:    0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := removalDeadlineLocal(c.lastTraining, c.tzOffset)
			loc := userLocalLoc(c.tzOffset)
			gotLocal := got.In(loc)

			if gotLocal.Hour() != c.wantHour || gotLocal.Minute() != 0 || gotLocal.Second() != 0 {
				t.Errorf("expected midnight, got %v", gotLocal)
			}
			if gotLocal.Day() != c.wantDay || gotLocal.Month() != c.wantMonth {
				t.Errorf("got %v, want %d %s", gotLocal, c.wantDay, c.wantMonth)
			}
		})
	}
}

// Активный больничный «замораживает» остаток: сколько бы ни шло время болезни,
// до кика остаётся столько же, сколько было на старте больничного (здесь ~3 дня).
func TestInactivityKickDeadline_ActiveSickLeaveFreezesRemaining(t *testing.T) {
	moscow := time.FixedZone("MSK", 3*3600)
	lastTraining := time.Date(2026, 6, 1, 12, 0, 0, 0, moscow)
	d0 := removalDeadlineLocal(lastTraining, 0)
	sickStart := d0.Add(-3 * 24 * time.Hour) // остаток 3 дня на старте больничного

	ml := &domain.MessageLog{
		TimerStartTime:     strPtr(utils.FormatMoscowTime(lastTraining)),
		HasSickLeave:       true,
		HasHealthy:         false,
		SickLeaveStartTime: strPtr(utils.FormatMoscowTime(sickStart)),
	}
	for _, daysSick := range []int{10, 40, 90} {
		now := sickStart.Add(time.Duration(daysSick) * 24 * time.Hour)
		dl, ok := inactivityKickDeadline(ml, now)
		if !ok {
			t.Fatalf("expected ok at day %d", daysSick)
		}
		remaining := dl.Sub(now)
		if remaining < 2*24*time.Hour || remaining > 4*24*time.Hour {
			t.Errorf("day %d: expected ~3 days frozen remaining, got %v", daysSick, remaining)
		}
	}
}

// Свежевосстановленный (через админку) или вернувшийся юзер: timer_start = NOW, флаги
// больничного сняты → должно быть полноценное недельное окно, а не мгновенный кик.
func TestInactivityKickDeadline_RestoredUserHasFreshWindow(t *testing.T) {
	moscow := time.FixedZone("MSK", 3*3600)
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, moscow)

	ml := &domain.MessageLog{
		TimerStartTime: strPtr(utils.FormatMoscowTime(now)),
		HasSickLeave:   false,
		HasHealthy:     false,
	}
	dl, ok := inactivityKickDeadline(ml, now)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if remaining := dl.Sub(now); remaining < 6*24*time.Hour {
		t.Errorf("restored user must get a fresh ~week window, got %v", remaining)
	}
}
