package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"leo-tracker/internal/config"
)

// Ключевые файлы, которые агент уже вырезал в заглушки (#13, #29, #30).
// Ниже порога — это не фича, а поломка прода.
var vitalFiles = []struct {
	path string
	min  int
}{
	{"miniapp/src/components/ProfileScreen.tsx", 800},
	{"miniapp/src/components/TrackerScreen.tsx", 400},
	{"miniapp/src/lib/trackerApi.ts", 80},
	{"ms_leo/internal/bot/tracker_agent.go", 400},
	{"ms_leo/internal/bot/tracker_deploy.go", 150},
	{"ms_leo/internal/config/config.go", 300},
}

func fileLineCount(src string) int {
	if src == "" {
		return 0
	}
	return strings.Count(src, "\n") + 1
}

func vitalSourceBroken(path, src string, min int) string {
	n := fileLineCount(src)
	if n < min {
		return fmt.Sprintf("%s обрезан (%d строк, нужно ≥%d)", path, n, min)
	}
	if path == "ms_leo/internal/config/config.go" && configLooksStub(src) {
		return "config.go стал заглушкой: нет parseAmountTiers/getEnv, бот не соберётся"
	}
	return ""
}

func vitalWorktreeBroken(repoDir string) string {
	for _, v := range vitalFiles {
		raw, err := os.ReadFile(filepath.Join(repoDir, filepath.FromSlash(v.path)))
		if err != nil {
			return v.path + ": " + err.Error()
		}
		if reason := vitalSourceBroken(v.path, string(raw), v.min); reason != "" {
			return reason
		}
	}
	return ""
}

// configLooksStub — то, что сдал агент на #29: вызовы parseAmountTiers/getEnv
// остались, самих функций нет. Такой файл компилироваться не будет.
func configLooksStub(src string) bool {
	if strings.TrimSpace(src) == "" {
		return true
	}
	if strings.Contains(src, "остальные поля конфига") ||
		strings.Contains(src, "код загрузки конфига") {
		return true
	}
	return !strings.Contains(src, "func parseAmountTiers") || !strings.Contains(src, "func getEnv")
}

func donateLine(src, key string) string {
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, key) && strings.Contains(line, "parseAmountTiers") {
			return line
		}
	}
	return ""
}

func lineHasAmount(line string, n int) bool {
	if line == "" || n <= 0 {
		return false
	}
	token := strconv.Itoa(n)
	for _, part := range strings.FieldsFunc(line, func(r rune) bool {
		return r < '0' || r > '9'
	}) {
		if part == token {
			return true
		}
	}
	return false
}

// implCheckFail — почему ревью/тест должны завалить сдачу. Пусто — ок.
func implCheckFail(prompt, configSrc string) string {
	if configLooksStub(configSrc) {
		return "config.go стал заглушкой: нет parseAmountTiers/getEnv, бот не соберётся"
	}
	stars := donateStarsFromPrompt(prompt)
	rub := donateRubFromPrompt(prompt)
	if len(stars) == 0 && len(rub) == 0 {
		return ""
	}
	starLine := donateLine(configSrc, "DONATE_STARS_TIERS")
	cardLine := donateLine(configSrc, "DONATE_CARD_TIERS_RUB")
	var missing []string
	for _, n := range stars {
		if !lineHasAmount(starLine, n) {
			missing = append(missing, fmt.Sprintf("%d⭐", n))
		}
	}
	for _, n := range rub {
		if !lineHasAmount(cardLine, n) && !lineHasAmount(starLine, n) {
			missing = append(missing, fmt.Sprintf("%d₽", n))
		}
	}
	if len(missing) == 0 {
		return ""
	}
	return "в config.go нет номинала из задачи: " + strings.Join(missing, ", ")
}

func githubFile(cfg config.Config, path, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.TrimSpace(cfg.Repo) == "" {
		return "", fmt.Errorf("нет ветки или репозитория")
	}
	q := "/repos/" + cfg.Repo + "/contents/" + path + "?ref=" + url.QueryEscape(ref)
	raw, status, err := githubGET(cfg, q)
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("github file HTTP %d", status)
	}
	var parsed struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	enc := strings.ToLower(strings.TrimSpace(parsed.Encoding))
	if enc != "" && enc != "base64" {
		return parsed.Content, nil
	}
	blob := strings.ReplaceAll(parsed.Content, "\n", "")
	dec, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

func CheckBranchImpl(cfg config.Config, branch, prompt string) string {
	return checkBranchImpl(cfg, branch, prompt)
}

func checkBranchImpl(cfg config.Config, branch, prompt string) string {
	for _, v := range vitalFiles {
		src, err := githubFile(cfg, v.path, branch)
		if err != nil {
			return "не прочитал " + v.path + " с ветки: " + err.Error()
		}
		if reason := vitalSourceBroken(v.path, src, v.min); reason != "" {
			return reason
		}
		if v.path == "ms_leo/internal/config/config.go" {
			if reason := implCheckFail(prompt, src); reason != "" {
				return reason
			}
		}
	}
	return ""
}
