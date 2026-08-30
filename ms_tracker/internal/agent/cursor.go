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
	"leo-tracker/internal/store"
)

const (
	cursorAPIDefault = "https://api.cursor.com"
	cursorWait       = 20 * time.Minute
	cursorPollEvery  = 5 * time.Second
	cursorHTTPWait   = 60 * time.Second
	cursorDefaultModel = "composer-2.5"
)

type cursorCreateResp struct {
	Agent struct {
		ID          string `json:"id"`
		LatestRunID string `json:"latestRunId"`
		URL         string `json:"url"`
	} `json:"agent"`
	Run struct {
		ID      string `json:"id"`
		AgentID string `json:"agentId"`
		Status  string `json:"status"`
	} `json:"run"`
}

type cursorRunResp struct {
	ID      string `json:"id"`
	AgentID string `json:"agentId"`
	Status  string `json:"status"`
	Result  string `json:"result"`
	Git     struct {
		Branches []struct {
			RepoURL string `json:"repoUrl"`
			Branch  string `json:"branch"`
			PRURL   string `json:"prUrl"`
		} `json:"branches"`
	} `json:"git"`
}

func cursorAPI(cfg config.Config) string {
	base := strings.TrimRight(strings.TrimSpace(cfg.CursorAPI), "/")
	if base == "" {
		return cursorAPIDefault
	}
	return base
}

func cursorModelID(cfg config.Config, job store.Job) string {
	for _, raw := range []string{job.Model, cfg.CursorModel} {
		m := strings.TrimSpace(raw)
		if m == "" || strings.EqualFold(m, "cursor-composer") {
			continue
		}
		return m
	}
	return cursorDefaultModel
}

func cursorRepoURL(repo string) string {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return ""
	}
	if strings.HasPrefix(repo, "http://") || strings.HasPrefix(repo, "https://") {
		return repo
	}
	return "https://github.com/" + strings.TrimPrefix(repo, "github.com/")
}

func cursorDoingPrompt(job store.Job, branch string) string {
	var b strings.Builder
	b.WriteString("Ты агент трекера Fat Leopard. Репозиторий уже на ветке задачи.\n")
	b.WriteString("Сделай задачу инструментами Cursor: правь файлы точечно, не переписывай файл целиком без нужды.\n")
	b.WriteString("Коммить и пушь только в текущую ветку ")
	b.WriteString(branch)
	b.WriteString(". Не открывай PR. Не создавай новую ветку cursor/*.\n")
	b.WriteString("В конце кратко напиши, что сделал. Без эмодзи.\n\n")
	b.WriteString(strings.TrimSpace(job.Prompt))
	return b.String()
}

func isCursorRunTerminal(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "FINISHED", "ERROR", "CANCELLED", "EXPIRED":
		return true
	default:
		return false
	}
}

func cursorPickedBranch(run cursorRunResp, want string) string {
	want = strings.TrimSpace(want)
	for _, br := range run.Git.Branches {
		name := strings.TrimSpace(br.Branch)
		if name == "" {
			continue
		}
		if want == "" || name == want {
			return name
		}
	}
	if want != "" {
		return want
	}
	if len(run.Git.Branches) > 0 {
		return strings.TrimSpace(run.Git.Branches[0].Branch)
	}
	return ""
}

func runCursorDoing(cfg config.Config, job store.Job, branch string) (cursorRunResp, error) {
	if strings.TrimSpace(cfg.CursorAPIKey) == "" {
		return cursorRunResp{}, fmt.Errorf("нет CURSOR_API_KEY")
	}
	repo := cursorRepoURL(cfg.Repo)
	if repo == "" {
		return cursorRunResp{}, fmt.Errorf("нет BOARD_REPO")
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = taskBranch(job)
	}
	name := fmt.Sprintf("Leo #%d", job.SourceNum)
	if job.SourceNum <= 0 {
		name = fmt.Sprintf("Leo job %d", job.ID)
	}
	body := map[string]any{
		"prompt":              map[string]string{"text": cursorDoingPrompt(job, branch)},
		"name":                clip(name, 100),
		"model":               map[string]string{"id": cursorModelID(cfg, job)},
		"env":                 map[string]string{"type": "cloud"},
		"repos":               []map[string]string{{"url": repo, "startingRef": branch}},
		"workOnCurrentBranch": true,
		"autoCreatePR":        false,
	}
	raw, err := cursorJSON(cfg, http.MethodPost, "/v1/agents", body)
	if err != nil {
		return cursorRunResp{}, err
	}
	var created cursorCreateResp
	if err := json.Unmarshal(raw, &created); err != nil {
		return cursorRunResp{}, fmt.Errorf("cursor create: %w", err)
	}
	agentID := strings.TrimSpace(created.Agent.ID)
	runID := strings.TrimSpace(created.Run.ID)
	if runID == "" {
		runID = strings.TrimSpace(created.Agent.LatestRunID)
	}
	if agentID == "" || runID == "" {
		return cursorRunResp{}, fmt.Errorf("cursor create: нет agent/run id")
	}
	deadline := time.Now().Add(cursorWait)
	var last cursorRunResp
	for time.Now().Before(deadline) {
		got, gerr := cursorGetRun(cfg, agentID, runID)
		if gerr != nil {
			return cursorRunResp{}, gerr
		}
		last = got
		if isCursorRunTerminal(got.Status) {
			if strings.EqualFold(got.Status, "FINISHED") {
				return got, nil
			}
			note := strings.TrimSpace(got.Result)
			if note == "" {
				note = got.Status
			}
			return got, fmt.Errorf("cursor run %s: %s", strings.ToLower(got.Status), clip(note, 240))
		}
		time.Sleep(cursorPollEvery)
	}
	return last, fmt.Errorf("cursor run timeout agent=%s run=%s", agentID, runID)
}

func cursorGetRun(cfg config.Config, agentID, runID string) (cursorRunResp, error) {
	path := "/v1/agents/" + agentID + "/runs/" + runID
	raw, err := cursorJSON(cfg, http.MethodGet, path, nil)
	if err != nil {
		return cursorRunResp{}, err
	}
	var out cursorRunResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return cursorRunResp{}, fmt.Errorf("cursor run: %w", err)
	}
	return out, nil
}

func cursorJSON(cfg config.Config, method, path string, body any) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, cursorAPI(cfg)+path, rdr)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(strings.TrimSpace(cfg.CursorAPIKey), "")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: cursorHTTPWait}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cursor: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cursor HTTP %d: %s", resp.StatusCode, clip(string(raw), 240))
	}
	return raw, nil
}
