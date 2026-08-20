package bot

import (
	"strings"
	"testing"
	"time"
)

func TestParseTrackerWhenInMinutes(t *testing.T) {
	at, label, err := parseTrackerWhen("через 5 мин")
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(at) < 4*time.Minute || time.Until(at) > 6*time.Minute {
		t.Fatalf("when: %v", at)
	}
	if label == "" || label == "—" {
		t.Fatalf("label: %q", label)
	}
}

func TestParseTrackerWhenTomorrow(t *testing.T) {
	at, label, err := parseTrackerWhen("завтра 4:20")
	if err != nil {
		t.Fatal(err)
	}
	loc := trackerMoscow()
	got := at.In(loc)
	if got.Hour() != 4 || got.Minute() != 20 {
		t.Fatalf("clock: %v", got)
	}
	now := time.Now().In(loc)
	if got.Day() == now.Day() && got.Month() == now.Month() {
		t.Fatalf("must be tomorrow, got %v", got)
	}
	if !strings.Contains(label, "04:20") && !strings.Contains(label, "4:20") {
		t.Fatalf("label: %q", label)
	}
}

func TestParseTrackerWhenDateTime(t *testing.T) {
	at, _, err := parseTrackerWhen("2026-08-20 09:00")
	if err != nil {
		t.Fatal(err)
	}
	got := at.In(trackerMoscow())
	if got.Year() != 2026 || got.Month() != 8 || got.Day() != 20 || got.Hour() != 9 {
		t.Fatalf("date: %v", got)
	}
	at2, _, err := parseTrackerWhen("2026-08-20T21:15")
	if err != nil {
		t.Fatal(err)
	}
	if at2.In(trackerMoscow()).Hour() != 21 {
		t.Fatalf("T-form: %v", at2)
	}
}

func TestParseTrackerWhenEmptyIsSoon(t *testing.T) {
	at, _, err := parseTrackerWhen("")
	if err != nil {
		t.Fatal(err)
	}
	if at.After(time.Now()) {
		t.Fatalf("empty must start now, got %v", at)
	}
}

func TestParseTrackerWhenNowIsImmediate(t *testing.T) {
	at, label, err := parseTrackerWhen("сейчас")
	if err != nil {
		t.Fatal(err)
	}
	if at.After(time.Now()) {
		t.Fatalf("сейчас must start now, got %v", at)
	}
	if label == "" || label == "—" {
		t.Fatalf("label: %q", label)
	}
	at2, _, err := parseTrackerWhen("now")
	if err != nil || at2.After(time.Now()) {
		t.Fatalf("now: %v err=%v", at2, err)
	}
}

func TestParseTrackerWhenBad(t *testing.T) {
	if _, _, err := parseTrackerWhen("когда-нибудь"); err == nil {
		t.Fatal("expected error")
	}
	if _, _, err := parseTrackerWhen("завтра 25:00"); err == nil {
		t.Fatal("expected bad hour")
	}
}
