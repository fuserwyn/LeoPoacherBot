package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"leo-tracker/internal/config"
)

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

func checkBranchImpl(cfg config.Config, branch, prompt string) string {
	src, err := githubFile(cfg, "ms_leo/internal/config/config.go", branch)
	if err != nil {
		return "не прочитал config.go с ветки: " + err.Error()
	}
	return implCheckFail(prompt, src)
}
