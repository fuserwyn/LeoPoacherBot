package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"leo-tracker/internal/config"
)

type branchInfo struct {
	Exists   bool
	Head     string
	Commits  []string
	Files    []string
	HasImpl  bool
}

func inspectBranch(cfg config.Config, branch string) (branchInfo, error) {
	var out branchInfo
	branch = strings.TrimSpace(branch)
	if branch == "" || strings.TrimSpace(cfg.GithubToken) == "" || strings.TrimSpace(cfg.Repo) == "" {
		return out, nil
	}
	base := strings.TrimSpace(cfg.Branch)
	if base == "" {
		base = "main"
	}
	raw, status, err := githubGET(cfg, "/repos/"+cfg.Repo+"/compare/"+base+"..."+branch)
	if err != nil {
		return out, err
	}
	if status == http.StatusNotFound {
		return out, nil
	}
	if status >= 300 {
		return out, fmt.Errorf("github compare HTTP %d", status)
	}
	var parsed struct {
		Status string `json:"status"`
		Commits []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message string `json:"message"`
			} `json:"commit"`
		} `json:"commits"`
		Files []struct {
			Filename string `json:"filename"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return out, err
	}
	out.Exists = true
	for _, c := range parsed.Commits {
		msg := strings.TrimSpace(strings.Split(c.Commit.Message, "\n")[0])
		if msg == "" {
			continue
		}
		out.Commits = append(out.Commits, msg)
		if out.Head == "" && len(c.SHA) >= 7 {
			out.Head = c.SHA[:7]
		}
	}
	if n := len(parsed.Commits); n > 0 && len(parsed.Commits[n-1].SHA) >= 7 {
		out.Head = parsed.Commits[n-1].SHA[:7]
	}
	for _, f := range parsed.Files {
		if strings.TrimSpace(f.Filename) == "" {
			continue
		}
		out.Files = append(out.Files, f.Filename)
		if !strings.HasPrefix(f.Filename, ".tracker/") {
			out.HasImpl = true
		}
	}
	return out, nil
}

func (info branchInfo) promptBlock() string {
	if !info.Exists {
		return "\n\nВ репозитории нет ветки этой задачи."
	}
	var b strings.Builder
	b.WriteString("\n\nКоммиты на ветке задачи:\n")
	if len(info.Commits) == 0 {
		b.WriteString("- нет коммитов относительно main\n")
	}
	for i, c := range info.Commits {
		if i >= 8 {
			break
		}
		b.WriteString("- ")
		b.WriteString(c)
		b.WriteByte('\n')
	}
	b.WriteString("Файлы:\n")
	if len(info.Files) == 0 {
		b.WriteString("- список пуст\n")
	}
	for i, f := range info.Files {
		if i >= 20 {
			b.WriteString("- …\n")
			break
		}
		b.WriteString("- ")
		b.WriteString(f)
		b.WriteByte('\n')
	}
	return b.String()
}

func githubGET(cfg config.Config, path string) ([]byte, int, error) {
	api := strings.TrimRight(strings.TrimSpace(cfg.GithubAPI), "/")
	if api == "" {
		api = "https://api.github.com"
	}
	req, err := http.NewRequest(http.MethodGet, api+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.GithubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "leo-tracker")
	resp, err := (&http.Client{Timeout: 25 * time.Second}).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return raw, resp.StatusCode, nil
}
