package agent

import (
	"strings"
	"testing"
)

func TestConfigLooksStub(t *testing.T) {
	if !configLooksStub(`func Load() (*Config, error) {
	return &Config{DonateStarsTiers: parseAmountTiers("1,5,10," + getEnv("DONATE_STARS_TIERS", "50"))}, nil
}`) {
		t.Fatal("без функций — заглушка")
	}
	if configLooksStub(`func parseAmountTiers(raw string) []int { return nil }
func getEnv(key, defaultValue string) string { return defaultValue }
		DonateStarsTiers: parseAmountTiers("1,5,10," + getEnv("DONATE_STARS_TIERS", "50,150,500")),
`) {
		t.Fatal("нормальный файл")
	}
}

func TestImplCheckFailDonate(t *testing.T) {
	good := `		DonateStarsTiers:   parseAmountTiers("1,5,10,1000," + getEnv("DONATE_STARS_TIERS", "50,150,500")),
		DonateCardTiersRub: parseAmountTiers("1000," + getEnv("DONATE_CARD_TIERS_RUB", "100,300,1000")),
func parseAmountTiers(raw string) []int { return nil }
func getEnv(key, defaultValue string) string { return "" }
`
	if got := implCheckFail("Сделай Донат 1000", good); got != "" {
		t.Fatalf("1000 есть: %s", got)
	}
	if got := implCheckFail("Сделай Донат 25 звезд", good); got == "" {
		t.Fatal("25⭐ нет — должен завалить")
	}
	if got := implCheckFail("Удали кнопку Донат 150", good); got == "" {
		t.Fatal("150 не скрыт — должен завалить")
	}
	hidden := good + `		parseAmountTiers("150," + getEnv("DONATE_STARS_HIDDEN", "")),
`
	if got := implCheckFail("Удали кнопку Донат 150", hidden); got != "" {
		t.Fatalf("скрыт: %s", got)
	}
	stub := `		DonateStarsTiers: parseAmountTiers("1,5,10,1000," + getEnv("DONATE_STARS_TIERS", "50")),
	// остальные поля конфига...
`
	if got := implCheckFail("Сделай Донат 1000", stub); got == "" {
		t.Fatal("заглушка должна падать")
	}
}

func TestVitalSourceBroken(t *testing.T) {
	if got := vitalSourceBroken("miniapp/src/components/ProfileScreen.tsx", "const x=1\n", 800); got == "" {
		t.Fatal("короткий профиль должен падать")
	}
	long := strings.Repeat("x\n", 900)
	if got := vitalSourceBroken("miniapp/src/components/ProfileScreen.tsx", long, 800); got != "" {
		t.Fatalf("длинный файл ок: %s", got)
	}
}

func TestLineHasAmount(t *testing.T) {
	line := `parseAmountTiers("1,5,10,1000," + getEnv("DONATE_STARS_TIERS", "50,150,500"))`
	if !lineHasAmount(line, 1000) || !lineHasAmount(line, 10) || lineHasAmount(line, 25) {
		t.Fatal(line)
	}
}
