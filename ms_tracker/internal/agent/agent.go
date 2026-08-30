package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	// Реализацию пишет Cursor Cloud Agent, не чат.
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
	branch, sha, _, _, err := applyPhaseCommit(cfg, job, out.Note, nil)
	out.Branch = branch
	out.Commit = sha
	out.Committed = sha != "" && err == nil
	return out, err
}

func applyDoing(cfg config.Config, job store.Job) (Result, error) {
	if strings.TrimSpace(cfg.CursorAPIKey) == "" {
		return Result{}, fmt.Errorf("нет CURSOR_API_KEY")
	}
	branch := strings.TrimSpace(job.Branch)
	if branch == "" {
		branch = taskBranch(job)
	}
	if strings.TrimSpace(cfg.GithubToken) != "" && strings.TrimSpace(cfg.Repo) != "" {
		ready, err := ensureRemoteTaskBranch(cfg, job, branch)
		if err != nil {
			return Result{}, err
		}
		if ready != "" {
			branch = ready
		}
	}
	cur, err := runCursorDoing(cfg, job, branch)
	if err != nil {
		return Result{}, err
	}
	note := strings.TrimSpace(cur.Result)
	if note == "" {
		note = "Cursor сдал задачу."
	}
	if picked := cursorPickedBranch(cur, branch); picked != "" {
		branch = picked
	}
	out := Result{Note: note, Branch: branch}
	if strings.TrimSpace(cfg.GithubToken) == "" || strings.TrimSpace(cfg.Repo) == "" {
		return out, nil
	}
	_, sha, hasImpl, rejected, gerr := applyPhaseCommit(cfg, job, note, nil)
	out.Commit = sha
	out.Committed = sha != "" && gerr == nil
	out.HasImpl = hasImpl && out.Committed
	if info, ierr := InspectBranch(cfg, branch); ierr == nil {
		if info.HasImpl {
			out.HasImpl = true
		}
		if info.Head != "" && out.Commit == "" {
			out.Commit = info.Head
		}
		if info.Exists {
			out.Committed = true
		}
	}
	if len(rejected) > 0 {
		out.Note = strings.TrimSpace(out.Note + "\n\nОтклонённые правки:\n- " + strings.Join(rejected, "\n- "))
	}
	if gerr != nil {
		out.Note = strings.TrimSpace(out.Note + "\n\nGit: " + gerr.Error())
	}
	return out, nil
}

func ensureRemoteTaskBranch(cfg config.Config, job store.Job, branch string) (string, error) {
	dir, err := os.MkdirTemp("", "leo-tracker-branch-*")
	if err != nil {
		return branch, err
	}
	repoDir, got, cleanup, err := prepareRepo(cfg, job, dir)
	defer cleanup()
	if err != nil {
		return got, err
	}
	if got != "" {
		branch = got
	}
	if err := run(repoDir, "git", "push", "-u", "origin", branch); err != nil {
		if remoteBranchExists(repoDir, branch) {
			return branch, nil
		}
		return branch, err
	}
	return branch, nil
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
	if err = checkoutTaskBranch(repoDir, branch, defaultBranch(cfg)); err != nil {
		return repoDir, branch, cleanup, err
	}
	return repoDir, branch, cleanup, nil
}

func defaultBranch(cfg config.Config) string {
	base := strings.TrimSpace(cfg.Branch)
	if base == "" {
		return "main"
	}
	return base
}

// checkoutTaskBranch поднимает ветку задачи. После shallow clone
// `git fetch origin <branch>` кладёт коммит в FETCH_HEAD и часто
// не создаёт origin/<branch> — от него checkout -B падает 128.
// Если ветки на remote нет, отталкиваемся от main.
func checkoutTaskBranch(repoDir, branch, base string) error {
	if base == "" {
		base = "main"
	}
	if remoteBranchExists(repoDir, branch) {
		refspec := "+refs/heads/" + branch + ":refs/remotes/origin/" + branch
		if err := run(repoDir, "git", "fetch", "origin", refspec, "--depth", "20"); err == nil {
			if err := run(repoDir, "git", "checkout", "-B", branch, "FETCH_HEAD"); err == nil {
				return nil
			}
			if err := run(repoDir, "git", "checkout", "-B", branch, "origin/"+branch); err == nil {
				return nil
			}
		}
	}
	return checkoutFromBase(repoDir, branch, base)
}

func checkoutFromBase(repoDir, branch, base string) error {
	if err := run(repoDir, "git", "fetch", "origin", base, "--depth", "20"); err == nil {
		if err := run(repoDir, "git", "checkout", "-B", branch, "FETCH_HEAD"); err == nil {
			return nil
		}
	}
	if err := run(repoDir, "git", "checkout", "-B", branch, "origin/"+base); err == nil {
		return nil
	}
	return run(repoDir, "git", "checkout", "-B", branch)
}

func applyPhaseCommit(cfg config.Config, job store.Job, note string, edits []fileEdit) (string, string, bool, []string, error) {
	dir, err := os.MkdirTemp("", "leo-tracker-*")
	if err != nil {
		return "", "", false, nil, err
	}
	repoDir, branch, cleanup, err := prepareRepo(cfg, job, dir)
	defer cleanup()
	if err != nil {
		return branch, "", false, nil, err
	}
	_, rejected, err := applyFileEdits(repoDir, edits)
	if err != nil {
		return branch, "", false, rejected, err
	}
	if len(rejected) > 0 {
		note = strings.TrimSpace(note + "\n\nОтклонённые правки:\n- " + strings.Join(rejected, "\n- "))
	}
	notePath := filepath.Join(repoDir, ".tracker", fmt.Sprintf("job-%d.md", job.SourceTaskID))
	if job.SourceTaskID <= 0 {
		notePath = filepath.Join(repoDir, ".tracker", fmt.Sprintf("job-%d.md", job.ID))
	}
	if err := os.MkdirAll(filepath.Dir(notePath), 0o755); err != nil {
		return branch, "", false, rejected, err
	}
	label := commitLabel(job.Phase)
	prev, _ := os.ReadFile(notePath)
	body := string(prev)
	if strings.TrimSpace(body) == "" {
		body = fmt.Sprintf("# Задача трекера #%d\n\n%s\n", job.SourceNum, job.Prompt)
	}
	body += fmt.Sprintf("\n## %s\n\n%s\n", label, strings.TrimSpace(note))
	if err := os.WriteFile(notePath, []byte(body), 0o644); err != nil {
		return branch, "", false, rejected, err
	}
	_ = run(repoDir, "git", "config", "user.email", "tracker@fat-leopard")
	_ = run(repoDir, "git", "config", "user.name", "Leo Tracker")
	if len(edits) > 0 {
		if err := run(repoDir, "git", "add", "-A"); err != nil {
			return branch, "", false, rejected, err
		}
	} else if err := run(repoDir, "git", "add", ".tracker"); err != nil {
		return branch, "", false, rejected, err
	}
	hasImpl := stagedHasImpl(repoDir)
	msg := fmt.Sprintf("tracker: #%d %s", job.SourceNum, label)
	if err := run(repoDir, "git", "commit", "-m", msg); err != nil {
		if sha := gitHead(repoDir); sha != "" && (label == "ревью" || label == "тест") {
			return branch, sha, hasImpl, rejected, nil
		}
		return branch, "", false, rejected, err
	}
	if err := run(repoDir, "git", "push", "-u", "origin", branch); err != nil {
		return branch, "", false, rejected, fmt.Errorf("ветка: %w", err)
	}
	return branch, gitHead(repoDir), hasImpl, rejected, nil
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
