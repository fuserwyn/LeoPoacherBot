package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"leo-tracker/internal/config"
)

func TestWritesGitOnlyDoing(t *testing.T) {
	if !writesGit("doing") || !writesGit("") {
		t.Fatal("doing commits implementation")
	}
	if writesGit("review") || writesGit("test") || writesGit("ship") {
		t.Fatal("review/test stamp after pass via Stamp, not writesGit")
	}
}

func TestVerdictPassed(t *testing.T) {
	if !verdictPassed("review", "глянул diff. можно на тест") {
		t.Fatal("review pass")
	}
	if verdictPassed("review", "ревью не принято: только план") {
		t.Fatal("review fail")
	}
	if !verdictPassed("review", "глянул поверхностно") {
		t.Fatal("lenient review passes")
	}
	if !verdictPassed("test", "дымовая ок") {
		t.Fatal("minimal test passes")
	}
}

func TestCommitLabel(t *testing.T) {
	if commitLabel("doing") != "выполнение" || commitLabel("review") != "ревью" || commitLabel("test") != "тест" {
		t.Fatal(commitLabel("doing") + " " + commitLabel("review") + " " + commitLabel("test"))
	}
}

func TestMergeToMainSuccess(t *testing.T) {
	var got struct {
		Base string `json:"base"`
		Head string `json:"head"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/fuserwyn/Fat-Leopard/merges" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatal("auth")
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sha":"abc"}`))
	}))
	defer srv.Close()

	base, err := MergeToMain(config.Config{
		GithubToken: "tok",
		GithubAPI:   srv.URL,
		Repo:        "fuserwyn/Fat-Leopard",
		Branch:      "main",
	}, "tracker/4-43", 4)
	if err != nil {
		t.Fatal(err)
	}
	if base != "main" || got.Base != "main" || got.Head != "tracker/4-43" {
		t.Fatalf("merge %+v base=%s", got, base)
	}
}

func TestMergeToMainConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()
	_, err := MergeToMain(config.Config{
		GithubToken: "tok",
		GithubAPI:   srv.URL,
		Repo:        "fuserwyn/Fat-Leopard",
	}, "tracker/4-1", 4)
	if err == nil || !strings.Contains(err.Error(), "конфликт") {
		t.Fatalf("conflict: %v", err)
	}
}

func TestMergeToMainAlreadyOnBase(t *testing.T) {
	base, err := MergeToMain(config.Config{GithubToken: "tok", Repo: "o/r", Branch: "main"}, "main", 1)
	if err != nil || base != "main" {
		t.Fatalf("noop: %s %v", base, err)
	}
}
