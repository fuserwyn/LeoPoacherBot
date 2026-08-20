package when

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	inMinutes = regexp.MustCompile(`(?i)^через\s+(\d+)\s*мин`)
	dateTime  = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})[ T](\d{1,2}):(\d{2})`)
)

func Moscow() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.FixedZone("MSK", 3*60*60)
	}
	return loc
}

func Parse(raw string) (at time.Time, label string, err error) {
	s := strings.TrimSpace(raw)
	now := time.Now().In(Moscow())
	if s == "" || strings.EqualFold(s, "сейчас") || strings.EqualFold(s, "now") {
		at = now.Add(-time.Second)
		return at, Format(now), nil
	}
	if m := inMinutes.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n <= 0 {
			at = now.Add(-time.Second)
			return at, Format(now), nil
		}
		at = now.Add(time.Duration(n) * time.Minute)
		return at, Format(at), nil
	}
	if m := dateTime.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		h, _ := strconv.Atoi(m[4])
		min, _ := strconv.Atoi(m[5])
		at = time.Date(y, time.Month(mo), d, h, min, 0, 0, Moscow())
		return at, Format(at), nil
	}
	return time.Time{}, "", fmt.Errorf("укажи время: «сейчас» или «через 5 мин»")
}

func Format(at time.Time) string {
	if at.IsZero() {
		return "—"
	}
	return at.In(Moscow()).Format("02.01 15:04")
}
