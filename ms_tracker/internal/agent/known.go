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

var donateTiersCallRe = regexp.MustCompile(
	`parseAmountTiers\("([0-9,]*)" \+ getEnv\("DONATE_STARS_TIERS"`,
)

var donateTiersEnvRe = regexp.MustCompile(
	`parseAmountTiers\(getEnv\("DONATE_STARS_TIERS", "([^"]*)"\)\)`,
)

func donateStarsFromPrompt(prompt string) []int {
	seen := map[int]bool{}
	var out []int
	for _, m := range donateStarsRe.FindAllStringSubmatch(prompt, -1) {
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

func applyDonateStarsTiers(repoDir string, stars []int) (int, error) {
	if len(stars) == 0 {
		return 0, nil
	}
	path := filepath.Join(repoDir, "ms_leo", "internal", "config", "config.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	text := string(raw)
	if loc := donateTiersCallRe.FindStringSubmatchIndex(text); len(loc) >= 4 {
		cur := text[loc[2]:loc[3]]
		next := mergeStarTiers(cur, stars)
		if next == "" || next == strings.Trim(cur, ",") {
			return 0, nil
		}
		updated := text[:loc[2]] + next + "," + text[loc[3]:]
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return 0, err
		}
		return 1, nil
	}
	if loc := donateTiersEnvRe.FindStringSubmatchIndex(text); len(loc) >= 2 {
		prefix := mergeStarTiers("", stars)
		if prefix == "" {
			return 0, nil
		}
		repl := `parseAmountTiers("` + prefix + `," + getEnv("DONATE_STARS_TIERS"`
		// parseAmountTiers(getEnv("DONATE_STARS_TIERS", "50,150,500"))
		// → parseAmountTiers("10," + getEnv("DONATE_STARS_TIERS", "50,150,500"))
		old := text[loc[0]:loc[1]]
		updated := strings.Replace(text, old, strings.Replace(old, `parseAmountTiers(getEnv("DONATE_STARS_TIERS"`, repl, 1), 1)
		if updated == text {
			return 0, nil
		}
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return 0, err
		}
		return 1, nil
	}
	return 0, fmt.Errorf("не нашёл список DONATE_STARS_TIERS")
}

func applyKnownTask(repoDir, prompt string) (string, int, error) {
	stars := donateStarsFromPrompt(prompt)
	if len(stars) == 0 {
		return "", 0, nil
	}
	n, err := applyDonateStarsTiers(repoDir, stars)
	if err != nil || n == 0 {
		return "", n, err
	}
	parts := make([]string, 0, len(stars))
	for _, s := range stars {
		parts = append(parts, fmt.Sprintf("%d", s))
	}
	return "Добавил донат " + strings.Join(parts, " и ") + " звёзд.", n, nil
}
