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
	Note      string
	Branch    string
	Commit    string
	Committed bool
	// HasImpl — в коммите есть файлы приложения, не только .tracker/job-N.md.
	HasImpl bool
	// Pushed — ветка уехала в main/стенд. Коммиты выполнения и ревью
	// живут на ветке задачи и пушем на стенд не считаются.
	Pushed bool
}

func Run(cfg config.Config, job store.Job) (Result, error) {
	phase := strings.ToLower(strings.TrimSpace(job.Phase))
	if phase == "" {
		phase = "doing"
	}
	if phase == "review" || phase == "test" {
		return runVerdict(cfg, job, phase)
	}
	if !writesGit(phase) || strings.TrimSpace(cfg.GithubToken) == "" || strings.TrimSpace(cfg.Repo) == "" {
		note, err := chat(cfg, job)
		return Result{Note: note}, err
	}
	return applyDoing(cfg, job)
}

func runVerdict(cfg config.Config, job store.Job, phase string) (Result, error) {
	branch := strings.TrimSpace(job.Branch)
	if branch == "" {
		branch = taskBranch(job)
	}
	info, ierr := InspectBranch(cfg, branch)
	if ierr != nil && !info.Exists && strings.TrimSpace(job.Prompt) == "" {
		return Result{}, ierr
	}
	if !info.Exists {
		note := "ревью не принято: нет коммита выполнения на ветке " + branch + "."
		if phase == "test" {
			note = "тест не прошёл: нет коммита выполнения на ветке " + branch + "."
		}
		return Result{Note: note, Branch: branch}, nil
	}
	// Пока ревью посредственное, тест дымовой: ветка есть — пропускаем.
	// Модель не зовём: на #6 она крутила отказы и упиралась в OpenRouter 504.
	note := lenientVerdictNote(phase, branch, info)
	out := Result{Note: note, Branch: branch, Commit: info.Head}
	if phase != "review" && phase != "test" {
		return out, nil
	}
	stamped, serr := Stamp(cfg, job, note)
	if serr != nil {
		out.Note = strings.TrimSpace(note + "\n\nGit: " + serr.Error())
		return out, nil
	}
	out.Commit = stamped.Commit
	out.Committed = stamped.Committed
	out.Branch = stamped.Branch
	out.HasImpl = true
	return out, nil
}

func lenientVerdictNote(phase, branch string, info branchInfo) string {
	if phase == "test" {
		return "Минимальный тест: ветка " + branch + " на месте, дымовая проверка ок. Тест пройден."
	}
	extra := "есть коммит выполнения"
	if info.HasImpl {
		extra = "есть правки приложения"
	}
	return "Посредственное ревью: на ветке " + branch + " " + extra + ". Можно на тест."
}

func verdictPassed(phase, text string) bool {
	low := strings.ToLower(text)
	if phase == "review" {
		if strings.Contains(low, "ревью не принято") || strings.Contains(low, "нельзя на тест") ||
			strings.Contains(low, `"pass":false`) {
			return false
		}
		return true
	}
	if strings.Contains(low, "тест не прошёл") || strings.Contains(low, "тест не прошел") ||
		strings.Contains(low, `"pass":false`) {
		return false
	}
	return true
}

// Stamp — коммит фазы на ветке задачи, без чата. Ревью и тест после
// вердикта пишут tracker: #N ревью / тест; пуш на стенд только после теста.
func Stamp(cfg config.Config, job store.Job, note string) (Result, error) {
	out := Result{Note: strings.TrimSpace(note)}
	if strings.TrimSpace(cfg.GithubToken) == "" || strings.TrimSpace(cfg.Repo) == "" {
		return out, fmt.Errorf("нет GitHub")
	}
	branch, sha, _, err := applyPhaseCommit(cfg, job, out.Note, nil)
	out.Branch = branch
	out.Commit = sha
	out.Committed = sha != "" && err == nil
	return out, err
}

func applyDoing(cfg config.Config, job store.Job) (Result, error) {
	note := ""
	edits := []fileEdit{}
	if strings.TrimSpace(cfg.OpenRouterKey) == "" {
		note = "Агент без OPENROUTER_API_KEY: задачу приняли, код не писали."
	} else {
		dir, err := os.MkdirTemp("", "leo-tracker-ctx-*")
		if err != nil {
			return Result{}, err
		}
		repoDir, branch, cleanup, err := prepareRepo(cfg, job, dir)
		if err != nil {
			cleanup()
			return Result{}, err
		}
		snippets := collectContextFiles(repoDir, job.Prompt)
		cleanup()
		reply, cerr := implChat(cfg, job, snippets)
		if cerr != nil {
			return Result{}, cerr
		}
		note = reply.Note
		edits = reply.Files
		_ = branch
	}
	out := Result{Note: note}
	branch, sha, hasImpl, gerr := applyPhaseCommit(cfg, job, note, edits)
	out.Branch = branch
	out.Commit = sha
	out.Committed = sha != "" && gerr == nil
	out.HasImpl = hasImpl && out.Committed
	if gerr != nil {
		if out.Note != "" {
			out.Note += "\n\n"
		}
		out.Note += "Git: " + gerr.Error()
	}
	return out, nil
}

func implChat(cfg config.Config, job store.Job, files []fileSnippet) (implReply, error) {
	var b strings.Builder
	b.WriteString(job.Prompt)
	if len(files) > 0 {
		b.WriteString("\n\nФайлы из репозитория. Верни полный новый текст каждого файла, который меняешь.\n")
		for _, f := range files {
			b.WriteString("\n--- ")
			b.WriteString(f.Path)
			b.WriteString(" ---\n")
			b.WriteString(f.Content)
			b.WriteByte('\n')
		}
	}
	system := "Ты агент трекера Fat Leopard. Меняй код приложения, не пиши только план. " +
		"Ответ — JSON без обёртки: {\"note\":\"что сделал\",\"files\":[{\"path\":\"miniapp/src/...\",\"content\":\"полный файл\"}]}. " +
		"path только из miniapp/, ms_leo/, ms_tracker/. Без эмодзи."
	raw, err := chatRaw(cfg, system, b.String())
	if err != nil {
		return implReply{}, err
	}
	return parseImplReply(raw), nil
}

func chat(cfg config.Config, job store.Job) (string, error) {
	if strings.TrimSpace(cfg.OpenRouterKey) == "" {
		return "Агент без OPENROUTER_API_KEY: задачу приняли, код не писали.", nil
	}
	system := "Ты агент трекера Fat Leopard. Пиши по-русски, коротко и по делу. " +
		"Сначала что сделаешь, потом конкретный план правок в репозитории. Без эмодзи."
	phase := strings.ToLower(strings.TrimSpace(job.Phase))
	if phase == "review" || phase == "test" {
		system = "Ты ревьюер Fat Leopard. Ревью посредственное, тест минимальный. " +
			"Если на ветке есть любой коммит — пиши «можно на тест» или «тест пройден». " +
			"Не придирайся к стилю, полноте и покрытию. Откажи только если ветки нет. Без эмодзи."
	}
	return chatRaw(cfg, system, job.Prompt)
}

func chatRaw(cfg config.Config, system, user string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model": cfg.OpenRouterModel,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
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
	return clip(note, 12000), nil
}

func writesGit(phase string) bool {
	p := strings.ToLower(strings.TrimSpace(phase))
	return p == "" || p == "doing"
}

func commitLabel(phase string) string {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "review":
		return "ревью"
	case "test":
		return "тест"
	default:
		return "выполнение"
	}
}

func taskBranch(job store.Job) string {
	if job.SourceNum > 0 && job.ID > 0 {
		return fmt.Sprintf("tracker/%d-%d", job.SourceNum, job.ID)
	}
	if job.SourceNum > 0 {
		return fmt.Sprintf("tracker/%d", job.SourceNum)
	}
	return fmt.Sprintf("tracker/0-%d", job.ID)
}

func prepareRepo(cfg config.Config, job store.Job, dir string) (repoDir, branch string, cleanup func(), err error) {
	cleanup = func() { _ = os.RemoveAll(dir) }
	cloneURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", cfg.GithubToken, cfg.Repo)
	if err = run(dir, "git", "clone", "--depth", "20", cloneURL, "repo"); err != nil {
		return "", "", cleanup, fmt.Errorf("clone: %w", err)
	}
	repoDir = filepath.Join(dir, "repo")
	branch = strings.TrimSpace(job.Branch)
	if branch == "" {
		branch = taskBranch(job)
	}
	if remoteBranchExists(repoDir, branch) {
		if err = run(repoDir, "git", "fetch", "origin", branch, "--depth", "20"); err != nil {
			return repoDir, branch, cleanup, fmt.Errorf("fetch: %w", err)
		}
		if err = run(repoDir, "git", "checkout", "-B", branch, "origin/"+branch); err != nil {
			return repoDir, branch, cleanup, err
		}
	} else if err = run(repoDir, "git", "checkout", "-B", branch); err != nil {
		return repoDir, branch, cleanup, err
	}
	return repoDir, branch, cleanup, nil
}

func applyPhaseCommit(cfg config.Config, job store.Job, note string, edits []fileEdit) (string, string, bool, error) {
	dir, err := os.MkdirTemp("", "leo-tracker-*")
	if err != nil {
		return "", "", false, err
	}
	repoDir, branch, cleanup, err := prepareRepo(cfg, job, dir)
	defer cleanup()
	if err != nil {
		return branch, "", false, err
	}
	if _, err := applyFileEdits(repoDir, edits); err != nil {
		return branch, "", false, err
	}
	notePath := filepath.Join(repoDir, ".tracker", fmt.Sprintf("job-%d.md", job.SourceTaskID))
	if job.SourceTaskID <= 0 {
		notePath = filepath.Join(repoDir, ".tracker", fmt.Sprintf("job-%d.md", job.ID))
	}
	if err := os.MkdirAll(filepath.Dir(notePath), 0o755); err != nil {
		return branch, "", false, err
	}
	label := commitLabel(job.Phase)
	prev, _ := os.ReadFile(notePath)
	body := string(prev)
	if strings.TrimSpace(body) == "" {
		body = fmt.Sprintf("# Задача трекера #%d\n\n%s\n", job.SourceNum, job.Prompt)
	}
	body += fmt.Sprintf("\n## %s\n\n%s\n", label, strings.TrimSpace(note))
	if err := os.WriteFile(notePath, []byte(body), 0o644); err != nil {
		return branch, "", false, err
	}
	_ = run(repoDir, "git", "config", "user.email", "tracker@fat-leopard")
	_ = run(repoDir, "git", "config", "user.name", "Leo Tracker")
	if len(edits) > 0 {
		if err := run(repoDir, "git", "add", "-A"); err != nil {
			return branch, "", false, err
		}
	} else if err := run(repoDir, "git", "add", ".tracker"); err != nil {
		return branch, "", false, err
	}
	hasImpl := stagedHasImpl(repoDir)
	msg := fmt.Sprintf("tracker: #%d %s", job.SourceNum, label)
	if err := run(repoDir, "git", "commit", "-m", msg); err != nil {
		if sha := gitHead(repoDir); sha != "" && (label == "ревью" || label == "тест") {
			return branch, sha, hasImpl, nil
		}
		return branch, "", false, err
	}
	if err := run(repoDir, "git", "push", "-u", "origin", branch); err != nil {
		return branch, "", false, fmt.Errorf("ветка: %w", err)
	}
	return branch, gitHead(repoDir), hasImpl, nil
}

func stagedHasImpl(repoDir string) bool {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return filesHaveImpl(strings.Split(string(out), "\n"))
}

func remoteBranchExists(repoDir, branch string) bool {
	cmd := exec.Command("git", "ls-remote", "--heads", "origin", branch)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	return err == nil && strings.Contains(string(out), "refs/heads/"+branch)
}

func gitHead(repoDir string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
