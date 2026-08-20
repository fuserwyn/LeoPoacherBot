package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"leo-tracker/internal/config"
)

func MergeToMain(cfg config.Config, head string, sourceNum int) (string, error) {
	head = strings.TrimSpace(head)
	if head == "" {
		return "", fmt.Errorf("нет ветки задачи")
	}
	if strings.TrimSpace(cfg.GithubToken) == "" {
		return "", fmt.Errorf("нет GITHUB_TOKEN")
	}
	repo := strings.TrimSpace(cfg.Repo)
	if repo == "" {
		return "", fmt.Errorf("не задан репозиторий")
	}
	base := strings.TrimSpace(cfg.Branch)
	if base == "" {
		base = "main"
	}
	if head == base {
		return base, nil
	}
	api := strings.TrimRight(strings.TrimSpace(cfg.GithubAPI), "/")
	if api == "" {
		api = "https://api.github.com"
	}
	msg := fmt.Sprintf("tracker: задача #%d → %s", sourceNum, base)
	body, _ := json.Marshal(map[string]string{
		"base":           base,
		"head":           head,
		"commit_message": msg,
	})
	req, err := http.NewRequest(http.MethodPost, api+"/repos/"+repo+"/merges", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.GithubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "leo-tracker")
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github merge: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK:
		return base, nil
	case http.StatusNoContent:
		return base, nil
	case http.StatusConflict:
		return "", fmt.Errorf("конфликт с %s, влить %s вручную", base, head)
	default:
		return "", fmt.Errorf("github merge HTTP %d: %s", resp.StatusCode, clip(string(raw), 240))
	}
}
