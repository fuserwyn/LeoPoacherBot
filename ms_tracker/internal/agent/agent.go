package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"leo-tracker/internal/config"
	"leo-tracker/internal/store"
)

type Result struct {
	Note   string
	Branch string
	Pushed bool
}

func Run(cfg config.Config, job store.Job) (Result, error) {
	note, err := chat(cfg, job)
	if err != nil {
		return Result{}, err
	}
	out := Result{Note: note}
	if strings.TrimSpace(cfg.GithubToken) == "" || strings.TrimSpace(cfg.Repo) == "" {
		return out, nil
	}
	branch, pushed, gerr := applyAndPush(cfg, job, note)
	out.Branch = branch
	out.Pushed = pushed
	if gerr != nil {
		if out.Note != "" {
			out.Note += "\n\n"
		}
		out.Note += "Git: " + gerr.Error()
	}
	return out, nil
}

func chat(cfg config.Config, job store.Job) (string, error) {
	if strings.TrimSpace(cfg.OpenRouterKey) == "" {
		return "Агент без OPENROUTER_API_KEY: задачу приняли, код не писали.", nil
	}
	body, _ := json.Marshal(map[string]any{
		"model": cfg.OpenRouterModel,
		"messages": []map[string]string{
			{
				"role": "system",
				"content": "Ты агент трекера Fat Leopard. Пиши по-русски, коротко и по делу. " +
					"Сначала что сделаешь, потом конкретный план правок в репозитории. Без эмодзи.",
			},
			{"role": "user", "content": job.Prompt},
		},
	})
	req, err := http.NewRequest(http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.OpenRouterKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("openrouter HTTP %d: %s", resp.StatusCode, clip(string(raw), 240))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openrouter пустой ответ")
	}
	note := strings.TrimSpace(parsed.Choices[0].Message.Content)
	return clip(note, 3500), nil
}

func applyAndPush(cfg config.Config, job store.Job, note string) (string, bool, error) {
	dir, err := os.MkdirTemp("", "leo-tracker-*")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(dir)
	cloneURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", cfg.GithubToken, cfg.Repo)
	if err := run(dir, "git", "clone", "--depth", "1", cloneURL, "repo"); err != nil {
		return "", false, fmt.Errorf("clone: %w", err)
	}
	repoDir := filepath.Join(dir, "repo")
	base := strings.TrimSpace(cfg.Branch)
	if base == "" {
		base = "main"
	}
	branch := fmt.Sprintf("tracker/%d-%d", job.SourceNum, job.ID)
	if err := run(repoDir, "git", "checkout", "-B", branch); err != nil {
		return "", false, err
	}
	notePath := filepath.Join(repoDir, ".tracker", fmt.Sprintf("job-%d.md", job.ID))
	if err := os.MkdirAll(filepath.Dir(notePath), 0o755); err != nil {
		return "", false, err
	}
	body := fmt.Sprintf("# Задача трекера #%d\n\n%s\n\n## Агент\n\n%s\n", job.SourceNum, job.Prompt, note)
	if err := os.WriteFile(notePath, []byte(body), 0o644); err != nil {
		return "", false, err
	}
	_ = run(repoDir, "git", "config", "user.email", "tracker@fat-leopard")
	_ = run(repoDir, "git", "config", "user.name", "Leo Tracker")
	if err := run(repoDir, "git", "add", ".tracker"); err != nil {
		return branch, false, err
	}
	msg := fmt.Sprintf("tracker: задача #%d", job.SourceNum)
	if err := run(repoDir, "git", "commit", "-m", msg); err != nil {
		return branch, false, err
	}
	if err := run(repoDir, "git", "push", "-u", "origin", branch); err != nil {
		return branch, false, fmt.Errorf("push: %w", err)
	}
	return branch, true, nil
}

func run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, clip(string(out), 300))
	}
	return nil
}

func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
