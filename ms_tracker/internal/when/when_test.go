package when

import (
	"testing"
	"time"
)

func TestParseNow(t *testing.T) {
	at, label, err := Parse("сейчас")
	if err != nil {
		t.Fatal(err)
	}
	if at.After(time.Now()) {
		t.Fatalf("сейчас must be due, got %v", at)
	}
	if label == "" || label == "—" {
		t.Fatalf("label: %q", label)
	}
}

func TestParseInMinutes(t *testing.T) {
	at, _, err := Parse("через 5 мин")
	if err != nil {
		t.Fatal(err)
	}
	wait := time.Until(at)
	if wait < 4*time.Minute || wait > 6*time.Minute {
		t.Fatalf("wait: %v", wait)
	}
}
