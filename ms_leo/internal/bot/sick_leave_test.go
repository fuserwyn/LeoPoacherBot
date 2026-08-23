package bot

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestExtractSickLeaveJustification_StripsTags(t *testing.T) {
	msg := &tgbotapi.Message{Text: "#sick_leave температура 38 и кашель"}
	got := extractSickLeaveJustification(msg)
	if got != "температура 38 и кашель" {
		t.Fatalf("unexpected justification: %q", got)
	}
}

func TestExtractSickLeaveJustification_UsesCaption(t *testing.T) {
	msg := &tgbotapi.Message{Caption: "#sickleave болею, нужен отдых"}
	got := extractSickLeaveJustification(msg)
	if got != "болею, нужен отдых" {
		t.Fatalf("unexpected justification from caption: %q", got)
	}
}

func TestExtractSickLeaveJustification_RemovesHealthyNoise(t *testing.T) {
	msg := &tgbotapi.Message{Text: "#healthy #sick_leave грипп и температура"}
	got := extractSickLeaveJustification(msg)
	if got != "грипп и температура" {
		t.Fatalf("unexpected cleaned text: %q", got)
	}
}
