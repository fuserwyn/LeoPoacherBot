package bot

import (
	"testing"

	"leo-bot/internal/config"
	"leo-bot/internal/game/leopardmoney"
)

func TestParsePaywallPriceRub(t *testing.T) {
	minor, err := parsePaywallPriceRub(99)
	if err != nil || minor != 9900 {
		t.Fatalf("99: got %d %v", minor, err)
	}
	if _, err := parsePaywallPriceRub(0); err == nil {
		t.Fatal("0 must be rejected")
	}
	if _, err := parsePaywallPriceRub(-1); err == nil {
		t.Fatal("negative must be rejected")
	}
	if _, err := parsePaywallPriceRub(maxPaywallPriceRub + 1); err == nil {
		t.Fatal("too large must be rejected")
	}
	minor, err = parsePaywallPriceRub(maxPaywallPriceRub)
	if err != nil || minor != maxPaywallPriceRub*100 {
		t.Fatalf("max: got %d %v", minor, err)
	}
}

func TestFormatPaywallAmountShort(t *testing.T) {
	if got := formatPaywallAmountShort(21000, "RUB"); got != "210 ₽" {
		t.Fatalf("got %q", got)
	}
	if got := formatPaywallAmountShort(21050, "RUB"); got != "210,50 ₽" {
		t.Fatalf("got %q", got)
	}
	if got := formatPaywallAmountShort(0, "RUB"); got != "" {
		t.Fatalf("zero: %q", got)
	}
	if got := formatPaywallAmountShort(350, "XTR"); got != "350 ⭐" {
		t.Fatalf("stars: %q", got)
	}
}

func TestAccessPriceRubFallsBackToConfig(t *testing.T) {
	b := &Bot{config: &config.Config{YookassaAmountMinor: 21000, YookassaCurrency: "RUB"}}
	if got := b.AccessPriceRub(); got != 210 {
		t.Fatalf("got %d", got)
	}
	empty := &Bot{}
	if got := empty.AccessPriceRub(); got != leopardmoney.EntryRub {
		t.Fatalf("empty bot: got %d", got)
	}
}

func TestPaywallProviderAmountUsesOverrideOnlyForRUB(t *testing.T) {
	b := &Bot{config: &config.Config{PaymentCurrency: "XTR", PaymentAmountMinorUnits: 350}}
	if got := b.paywallProviderAmountMinor(); got != 350 {
		t.Fatalf("XTR must stay stars, got %d", got)
	}
}

func TestPaywallYookassaReadyUsesEffectiveAmount(t *testing.T) {
	ready := &Bot{config: &config.Config{
		YookassaShopID:      "shop",
		YookassaSecretKey:   "key",
		YookassaAmountMinor: 9900,
		YookassaCurrency:    "RUB",
	}}
	if !ready.paywallYookassaReady() || !ready.paywallPaymentReady() {
		t.Fatal("expected yookassa ready from env amount")
	}
	noAmount := &Bot{config: &config.Config{
		YookassaShopID:    "shop",
		YookassaSecretKey: "key",
		YookassaCurrency:  "RUB",
	}}
	if noAmount.paywallYookassaReady() {
		t.Fatal("zero amount must not be ready without admin override")
	}
}
