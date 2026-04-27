package bot

import (
	"strings"
	"testing"
)

// Повторный вход: одна кнопка «Войти в группу»; новая ссылка — /start или /rejoin (бот создаёт без срока по времени, при сбое — запас на ~24 ч).
func TestRetryEntry_RejoinInviteKeyboard(t *testing.T) {
	invite := "https://t.me/+testInviteLink"
	kb := freshRejoinInviteKeyboard(invite)
	if kb == nil {
		t.Fatal("keyboard is nil")
	}
	if len(kb.InlineKeyboard) != 1 {
		t.Fatalf("want 1 keyboard row, got %d", len(kb.InlineKeyboard))
	}

	join := kb.InlineKeyboard[0][0]
	if join.Text != "📩 Войти в группу" {
		t.Fatalf("join button text: got %q", join.Text)
	}
	if join.URL == nil || *join.URL != invite {
		t.Fatalf("join button url: got %+v want %q", join.URL, invite)
	}
	if join.CallbackData != nil {
		t.Fatalf("join button must not use callback_data")
	}
}

func TestRetryEntry_PaidStartHintLine(t *testing.T) {
	s := paidPrivateRetryEntryHintLine()
	if !strings.Contains(s, "Мини-приложение") {
		t.Fatalf("hint should mention mini app: %q", s)
	}
	if !strings.Contains(s, "/start") {
		t.Fatalf("hint should mention /start for fresh link: %q", s)
	}
}
