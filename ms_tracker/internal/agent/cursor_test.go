package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"leo-tracker/internal/config"
	"leo-tracker/internal/store"
)

func TestCursorModelID(t *testing.T) {
	if got := cursorModelID(config.Config{}, store.Job{}); got != "composer-2.5" {
		t.Fatalf("default: %s", got)
	}
	if got := cursorModelID(config.Config{CursorModel: "cursor-composer"}, store.Job{Model: "cursor-composer"}); got != "composer-2.5" {
		t.Fatalf("alias: %s", got)
	}
	if got := cursorModelID(config.Config{CursorModel: "composer-2"}, store.Job{}); got != "composer-2" {
		t.Fatalf("cfg: %s", got)
	}
	if got := cursorModelID(config.Config{CursorModel: "composer-2"}, store.Job{Model: "composer-2.5"}); got != "composer-2.5" {
		t.Fatalf("job wins: %s", got)
	}
}

func TestCursorRepoURL(t *testing.T) {
	if got := cursorRepoURL("fuserwyn/Fat-Leopard"); got != "https://github.com/fuserwyn/Fat-Leopard" {
		t.Fatal(got)
	}
	if got := cursorRepoURL("https://github.com/fuserwyn/Fat-Leopard"); got != "https://github.com/fuserwyn/Fat-Leopard" {
		t.Fatal(got)
	}
}

func TestIsCursorRunTerminal(t *testing.T) {
	if !isCursorRunTerminal("FINISHED") || !isCursorRunTerminal("error") {
		t.Fatal("terminal")
	}
	if isCursorRunTerminal("RUNNING") || isCursorRunTerminal("CREATING") {
		t.Fatal("active")
	}
}

func TestCursorPickedBranch(t *testing.T) {
	var run cursorRunResp
	run.Git.Branches = append(run.Git.Branches,
		struct {
			RepoURL string `json:"repoUrl"`
			Branch  string `json:"branch"`
			PRURL   string `json:"prUrl"`
		}{Branch: "cursor/other"},
		struct {
			RepoURL string `json:"repoUrl"`
			Branch  string `json:"branch"`
			PRURL   string `json:"prUrl"`
		}{Branch: "tracker/24-71"},
	)
	if got := cursorPickedBranch(run, "tracker/24-71"); got != "tracker/24-71" {
		t.Fatal(got)
	}
}

func TestRunCursorDoing(t *testing.T) {
	var sawAuth bool
	var createBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := r.BasicAuth()
		if ok && user == "cursor_key" {
			sawAuth = true
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/agents":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &createBody)
			_, _ = w.Write([]byte(`{"agent":{"id":"bc-1","latestRunId":"run-1"},"run":{"id":"run-1","status":"CREATING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents/bc-1/runs/run-1":
			_, _ = w.Write([]byte(`{"id":"run-1","status":"FINISHED","result":"добавил донат","git":{"branches":[{"branch":"tracker/24-71"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	got, err := runCursorDoing(config.Config{
		CursorAPIKey: "cursor_key",
		CursorAPI:    srv.URL,
		Repo:         "fuserwyn/Fat-Leopard",
	}, store.Job{ID: 71, SourceNum: 24, Prompt: "Сделай донат 1 и 5 звезд"}, "tracker/24-71")
	if err != nil {
		t.Fatal(err)
	}
	if !sawAuth || got.Result != "добавил донат" {
		t.Fatalf("auth=%v result=%q", sawAuth, got.Result)
	}
	if cursorPickedBranch(got, "tracker/24-71") != "tracker/24-71" {
		t.Fatal(got.Git)
	}
	repos, _ := createBody["repos"].([]any)
	if len(repos) != 1 {
		t.Fatalf("repos: %#v", createBody["repos"])
	}
	if createBody["workOnCurrentBranch"] != true {
		t.Fatalf("branch: %#v", createBody["workOnCurrentBranch"])
	}
	prompt := createBody["prompt"].(map[string]any)["text"].(string)
	if !strings.Contains(prompt, "Сделай донат") {
		t.Fatal(prompt)
	}
}

func TestApplyDoingRequiresCursorKey(t *testing.T) {
	_, err := applyDoing(config.Config{}, store.Job{Prompt: "x"})
	if err == nil || !strings.Contains(err.Error(), "CURSOR_API_KEY") {
		t.Fatalf("%v", err)
	}
}
