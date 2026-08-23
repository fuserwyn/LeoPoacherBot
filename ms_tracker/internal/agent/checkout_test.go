package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckoutTaskBranchUsesFetchHead(t *testing.T) {
	remote := initGitRepo(t, "remote")
	mustGit(t, remote, "checkout", "-B", "tracker/13-21")
	writeCommit(t, remote, "task.txt", "from task branch")
	mustGit(t, remote, "checkout", "main")

	local := filepath.Join(t.TempDir(), "local")
	mustRun(t, "", "git", "clone", "--depth", "1", remote, local)
	if gitRevParse(t, local, "origin/tracker/13-21") != "" {
		t.Fatal("shallow clone should not have origin/tracker/13-21")
	}
	if err := checkoutTaskBranch(local, "tracker/13-21", "main"); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, filepath.Join(local, "task.txt"))
	if got != "from task branch" {
		t.Fatalf("got %q", got)
	}
}

func TestCheckoutTaskBranchStartsFromMainWhenMissing(t *testing.T) {
	remote := initGitRepo(t, "remote")
	writeCommit(t, remote, "base.txt", "from main")

	local := filepath.Join(t.TempDir(), "local")
	mustRun(t, "", "git", "clone", "--depth", "1", remote, local)
	if err := checkoutTaskBranch(local, "tracker/13-99", "main"); err != nil {
		t.Fatal(err)
	}
	branch := strings.TrimSpace(mustGitOut(t, local, "branch", "--show-current"))
	if branch != "tracker/13-99" {
		t.Fatalf("branch %q", branch)
	}
	if readFile(t, filepath.Join(local, "base.txt")) != "from main" {
		t.Fatal("should start from main")
	}
}

func TestCheckoutTaskBranchOldOriginRefFails(t *testing.T) {
	remote := initGitRepo(t, "remote")
	mustGit(t, remote, "checkout", "-B", "tracker/13-21")
	writeCommit(t, remote, "task.txt", "from task branch")
	mustGit(t, remote, "checkout", "main")

	local := filepath.Join(t.TempDir(), "local")
	mustRun(t, "", "git", "clone", "--depth", "1", remote, local)
	mustGit(t, local, "fetch", "origin", "tracker/13-21", "--depth", "1")
	err := run(local, "git", "checkout", "-B", "tracker/13-21", "origin/tracker/13-21")
	if err == nil {
		t.Skip("this git creates origin/tracker after fetch; old crash not reproduced")
	}
	if !strings.Contains(err.Error(), "origin/tracker/13-21") {
		t.Fatalf("unexpected: %v", err)
	}
	if err := checkoutTaskBranch(local, "tracker/13-21", "main"); err != nil {
		t.Fatal(err)
	}
}

func initGitRepo(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "init", "-b", "main")
	mustGit(t, dir, "config", "user.email", "tracker@test")
	mustGit(t, dir, "config", "user.name", "Tracker Test")
	writeCommit(t, dir, "README", "main")
	return dir
}

func writeCommit(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", name)
	mustGit(t, dir, "commit", "-m", name)
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	mustRun(t, dir, "git", args...)
}

func mustGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func gitRevParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
