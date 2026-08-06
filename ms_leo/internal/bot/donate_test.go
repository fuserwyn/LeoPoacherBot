package bot

import (
	"strings"
	"testing"
)

func TestParseDonatePayload(t *testing.T) {
	cases := []struct {
		in     string
		wantID int64
		wantOK bool
	}{
		{"dn_1", 1, true},
		{"dn_4242", 4242, true},
		{"  dn_7  ", 7, true},
		{"dn_0", 0, false},
		{"dn_abc", 0, false},
		{"dn_", 0, false},
		// Платёж за возврат (paywall) не должен попасть в обработчик донатов и наоборот.
		{"pw_5", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			id, ok := parseDonatePayload(tc.in)
			if ok != tc.wantOK || id != tc.wantID {
				t.Fatalf("parseDonatePayload(%q) = (%d, %v), want (%d, %v)", tc.in, id, ok, tc.wantID, tc.wantOK)
			}
			if IsDonatePayload(tc.in) != tc.wantOK {
				t.Fatalf("IsDonatePayload(%q) = %v, want %v", tc.in, !tc.wantOK, tc.wantOK)
			}
		})
	}
}

// Роутинг платёжных апдейтов держится на том, что префиксы не пересекаются.
func TestDonateAndPaywallPayloadsAreDisjoint(t *testing.T) {
	payload := donatePayload(12)
	if payload != "dn_12" {
		t.Fatalf("donatePayload(12) = %q", payload)
	}
	if _, ok := parsePaywallPayload(payload); ok {
		t.Fatalf("donate payload %q распознан как paywall", payload)
	}
	if IsDonatePayload("pw_12") {
		t.Fatal("paywall payload распознан как донат")
	}
}

func TestDonateAmountHuman(t *testing.T) {
	cases := []struct {
		amountMinor int
		currency    string
		want        string
	}{
		{150, "XTR", "150 ⭐"},
		{30000, "RUB", "300 ₽"},
		{30050, "RUB", "300,50 ₽"},
		{1000, "USD", "10 USD"},
		{0, "XTR", ""},
		{-5, "RUB", ""},
		{100, "", ""},
	}
	for _, tc := range cases {
		got := donateAmountHuman(tc.amountMinor, tc.currency)
		if got != tc.want {
			t.Fatalf("donateAmountHuman(%d, %q) = %q, want %q", tc.amountMinor, tc.currency, got, tc.want)
		}
	}
}

// Счёт Telegram обрезает title до 32 символов и description до 255 — иначе sendInvoice падает.
func TestDonateStarsInvoiceTextsFitTelegramLimits(t *testing.T) {
	title := paywallInvoiceClipTitle("Поддержать Fat Leopard")
	if n := len([]rune(title)); n == 0 || n > 32 {
		t.Fatalf("title runes = %d: %q", n, title)
	}
	desc := paywallInvoiceClipDescription(donateStarsInvoiceDescription(500))
	if n := len([]rune(desc)); n == 0 || n > 255 {
		t.Fatalf("description runes = %d", n)
	}
	if !strings.Contains(desc, "500") {
		t.Fatalf("в описании счёта нет суммы: %q", desc)
	}
}

func TestDonateThanksTextMentionsAmount(t *testing.T) {
	txt := donateThanksText(150, "XTR")
	if !strings.Contains(txt, "150 ⭐") {
		t.Fatalf("нет суммы в тексте благодарности: %q", txt)
	}
	// Сумма неизвестна (например, пришла пустая валюта) — текст всё равно осмысленный.
	if strings.Contains(donateThanksText(0, ""), " на ") {
		t.Fatal("без суммы не должно быть «на »")
	}
}
