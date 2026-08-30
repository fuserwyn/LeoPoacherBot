package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDonateStarsFromPrompt(t *testing.T) {
	got := donateStarsFromPrompt("Задача #28.\n\nСделай 10 звезд донат")
	if len(got) != 1 || got[0] != 10 {
		t.Fatalf("%v", got)
	}
	got = donateStarsFromPrompt("Добавь Донат 1 звезду и 5 звезд")
	if len(got) != 2 || got[0] != 1 || got[1] != 5 {
		t.Fatalf("%v", got)
	}
	if donateStarsFromPrompt("почини кнопку профиля") != nil {
		t.Fatal("not donate")
	}
}

func TestApplyDonateStarsTiersPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ms_leo", "internal", "config", "config.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	src := `		DonateStarsTiers:   parseAmountTiers("1,5," + getEnv("DONATE_STARS_TIERS", "50,150,500")),`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	note, n, err := applyKnownTask(dir, "Сделай 10 звезд донат")
	if err != nil || n != 1 || !strings.Contains(note, "10") {
		t.Fatalf("n=%d note=%q err=%v", n, note, err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), `"1,5,10,"`) {
		t.Fatalf("%s", raw)
	}
}

func TestMergeStarTiers(t *testing.T) {
	if got := mergeStarTiers("1,5,", []int{10, 5}); got != "1,5,10" {
		t.Fatal(got)
	}
}
