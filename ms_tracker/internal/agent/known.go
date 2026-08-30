package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var donateStarsRe = regexp.MustCompile(`(?i)(\d+)\s*зв[её]зд`)

var donateRubRe = regexp.MustCompile(`(?i)(\d+)\s*(?:руб(?:л|\b)|₽)`)

// «Сделай Донат 1000» / «донат 1000» — без валюты. Иначе трекер не узнаёт задачу
// и отдаёт её Cursor, который пишет заглушку вместо номинала.
var donateBareRe = regexp.MustCompile(`(?i)донат\s+(\d+)`)

var donateTiersCallRe = regexp.MustCompile(
	`parseAmountTiers\("([0-9,]*)" \+ getEnv\("DONATE_STARS_TIERS"`,
)

var donateTiersEnvRe = regexp.MustCompile(
	`parseAmountTiers\(getEnv\("DONATE_STARS_TIERS", "([^"]*)"\)\)`,
)

var donateCardCallRe = regexp.MustCompile(
	`parseAmountTiers\("([0-9,]*)" \+ getEnv\("DONATE_CARD_TIERS_RUB"`,
)

var donateCardEnvRe = regexp.MustCompile(
	`parseAmountTiers\(getEnv\("DONATE_CARD_TIERS_RUB", "([^"]*)"\)\)`,
)

func collectAmounts(re *regexp.Regexp, prompt string) []int {
	seen := map[int]bool{}
	var out []int
	for _, m := range re.FindAllStringSubmatch(prompt, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n <= 0 || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

func donateStarsFromPrompt(prompt string) []int {
	out := collectAmounts(donateStarsRe, prompt)
	if len(out) > 0 {
		return out
	}
	return collectAmounts(donateBareRe, prompt)
}

func donateRubFromPrompt(prompt string) []int {
	out := collectAmounts(donateRubRe, prompt)
	if len(out) > 0 {
		return out
	}
	if donateStarsRe.MatchString(prompt) {
		return nil
	}
	return collectAmounts(donateBareRe, prompt)
}

func mergeStarTiers(base string, extra []int) string {
	seen := map[int]bool{}
	var out []int
	for _, part := range strings.Split(base, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n <= 0 || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	for _, n := range extra {
		if n <= 0 || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	sort.Ints(out)
	parts := make([]string, 0, len(out))
	for _, n := range out {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, ",")
}

func applyTierPrefix(text string, extras []int, callRe, envRe *regexp.Regexp, envKey string) (string, int, error) {
	if len(extras) == 0 {
		return text, 0, nil
	}
	if loc := callRe.FindStringSubmatchIndex(text); len(loc) >= 4 {
		cur := text[loc[2]:loc[3]]
		next := mergeStarTiers(cur, extras)
		if next == "" || next == strings.Trim(cur, ",") {
			return text, 0, nil
		}
		return text[:loc[2]] + next + "," + text[loc[3]:], 1, nil
	}
	if loc := envRe.FindStringSubmatchIndex(text); len(loc) >= 2 {
		prefix := mergeStarTiers("", extras)
		if prefix == "" {
			return text, 0, nil
		}
		oldCall := `parseAmountTiers(getEnv("` + envKey + `"`
		repl := `parseAmountTiers("` + prefix + `," + getEnv("` + envKey + `"`
		old := text[loc[0]:loc[1]]
		updated := strings.Replace(text, old, strings.Replace(old, oldCall, repl, 1), 1)
		if updated == text {
			return text, 0, nil
		}
		return updated, 1, nil
	}
	return text, 0, fmt.Errorf("не нашёл список %s", envKey)
}

func applyDonateStarsTiers(repoDir string, stars []int) (int, error) {
	path := filepath.Join(repoDir, "ms_leo", "internal", "config", "config.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	updated, n, err := applyTierPrefix(string(raw), stars, donateTiersCallRe, donateTiersEnvRe, "DONATE_STARS_TIERS")
	if err != nil || n == 0 {
		return n, err
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return 0, err
	}
	return n, nil
}

func applyDonateCardTiers(repoDir string, rub []int) (int, error) {
	path := filepath.Join(repoDir, "ms_leo", "internal", "config", "config.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	updated, n, err := applyTierPrefix(string(raw), rub, donateCardCallRe, donateCardEnvRe, "DONATE_CARD_TIERS_RUB")
	if err != nil || n == 0 {
		return n, err
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return 0, err
	}
	return n, nil
}

func isDonatePrompt(prompt string) bool {
	return len(donateStarsFromPrompt(prompt)) > 0 || len(donateRubFromPrompt(prompt)) > 0
}

func shouldRunCursor(prompt string, knownEdits int) bool {
	return !isDonatePrompt(prompt) && knownEdits == 0
}

func donateAmountsPresent(src, prompt string) bool {
	stars := donateStarsFromPrompt(prompt)
	rub := donateRubFromPrompt(prompt)
	if len(stars) == 0 && len(rub) == 0 {
		return false
	}
	starLine := donateLine(src, "DONATE_STARS_TIERS")
	cardLine := donateLine(src, "DONATE_CARD_TIERS_RUB")
	for _, n := range stars {
		if !lineHasAmount(starLine, n) {
			return false
		}
	}
	for _, n := range rub {
		if !lineHasAmount(cardLine, n) && !lineHasAmount(starLine, n) {
			return false
		}
	}
	return true
}

func applyKnownTask(repoDir, prompt string) (string, int, error) {
	stars := donateStarsFromPrompt(prompt)
	rub := donateRubFromPrompt(prompt)
	if len(stars) == 0 && len(rub) == 0 {
		return "", 0, nil
	}
	n := 0
	if len(stars) > 0 {
		sn, err := applyDonateStarsTiers(repoDir, stars)
		if err != nil {
			return "", n, err
		}
		n += sn
	}
	if len(rub) > 0 {
		cn, err := applyDonateCardTiers(repoDir, rub)
		if err != nil && n == 0 {
			return "", n, err
		}
		if err == nil {
			n += cn
		}
	}
	var parts []string
	for _, s := range stars {
		parts = append(parts, fmt.Sprintf("%d ⭐", s))
	}
	for _, r := range rub {
		parts = append(parts, fmt.Sprintf("%d ₽", r))
	}
	label := strings.Join(parts, " и ")
	if n == 0 {
		raw, err := os.ReadFile(filepath.Join(repoDir, "ms_leo", "internal", "config", "config.go"))
		if err != nil {
			return "", 0, err
		}
		if donateAmountsPresent(string(raw), prompt) {
			return "Номинал уже есть в config.go: " + label + ".", 0, nil
		}
		return "", 0, fmt.Errorf("не смог вписать донат %s в config.go", label)
	}
	return "Добавил донат " + label + ".", n, nil
}
