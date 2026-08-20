package bot

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Разбор «когда запустить» с формы доски: пресеты («через 1 мин»,
// «завтра 4:20») и datetime-local («2026-08-20 09:00»). Время — московское,
// как и остальная база стаи.

var (
	whenInMinutes = regexp.MustCompile(`(?i)^через\s+(\d+)\s*мин`)
	whenTomorrow  = regexp.MustCompile(`(?i)^завтра\s+(\d{1,2}):(\d{2})$`)
	whenDateTime  = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})[ T](\d{1,2}):(\d{2})`)
)

func trackerMoscow() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.FixedZone("MSK", 3*60*60)
	}
	return loc
}

func parseTrackerWhen(raw string) (at time.Time, label string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		s = "через 1 мин"
	}
	now := time.Now().In(trackerMoscow())

	if m := whenInMinutes.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n <= 0 {
			n = 1
		}
		if n > 7*24*60 {
			return time.Time{}, "", fmt.Errorf("слишком далеко: максимум неделя")
		}
		at = now.Add(time.Duration(n) * time.Minute)
		return at, formatTrackerWhen(at), nil
	}
	if m := whenTomorrow.FindStringSubmatch(s); m != nil {
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		if h > 23 || min > 59 {
			return time.Time{}, "", fmt.Errorf("непонятное время")
		}
		day := now.Add(24 * time.Hour)
		at = time.Date(day.Year(), day.Month(), day.Day(), h, min, 0, 0, trackerMoscow())
		return at, formatTrackerWhen(at), nil
	}
	if m := whenDateTime.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		h, _ := strconv.Atoi(m[4])
		min, _ := strconv.Atoi(m[5])
		if mo < 1 || mo > 12 || d < 1 || d > 31 || h > 23 || min > 59 {
			return time.Time{}, "", fmt.Errorf("непонятная дата")
		}
		at = time.Date(y, time.Month(mo), d, h, min, 0, 0, trackerMoscow())
		return at, formatTrackerWhen(at), nil
	}
	return time.Time{}, "", fmt.Errorf("укажи время: «через 5 мин» или дату")
}

func formatTrackerWhen(at time.Time) string {
	if at.IsZero() {
		return "—"
	}
	return at.In(trackerMoscow()).Format("02.01 15:04")
}
