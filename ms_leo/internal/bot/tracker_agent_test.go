package bot

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"leo-bot/internal/config"
	"leo-bot/internal/database"
)

func TestTrackerAgentPromptPhases(t *testing.T) {
	task := database.TrackerTask{Num: 7, Prompt: "починить стрик", Result: "подпись короче"}
	doing := trackerAgentPrompt(task, "doing")
	if !strings.Contains(doing, "#7") || !strings.Contains(doing, "починить стрик") {
		t.Fatalf("doing: %q", doing)
	}
	review := trackerAgentPrompt(task, "review")
	if !strings.Contains(review, "Cursor Composer") || !strings.Contains(review, "можно на тест") {
		t.Fatalf("review: %q", review)
	}
	test := trackerAgentPrompt(task, "test")
	if !strings.Contains(test, "тест пройден") || !strings.Contains(test, "подпись короче") {
		t.Fatalf("test: %q", test)
	}
}

func TestTrackerComposerPassed(t *testing.T) {
	if !trackerComposerPassed("review", "Глянул diff. Можно на тест.") {
		t.Fatal("review pass")
	}
	if trackerComposerPassed("review", "Ревью не принято: нет тестов.") {
		t.Fatal("review fail")
	}
	if !trackerComposerPassed("test", "Покрыл сценарий. Тест пройден.") {
		t.Fatal("test pass")
	}
	if trackerComposerPassed("test", "Тест не прошёл: кнопка не жмётся.") {
		t.Fatal("test fail")
	}
	if !trackerComposerPassed("doing", "⏰ Задача #1 выполнена.\n\nГотово.") {
		t.Fatal("impl done is pass")
	}
}

func TestTrackerStepRemoteID(t *testing.T) {
	if got := trackerStepRemoteID([]string{"Взяли", "агент:#512", "Composer: запустили"}); got != 512 {
		t.Fatalf("got %d", got)
	}
	if got := trackerStepRemoteID(nil); got != 0 {
		t.Fatalf("empty %d", got)
	}
}

func TestTrackerComposerModel(t *testing.T) {
	if got := trackerComposerModel(nil); got != trackerComposerModelKey {
		t.Fatalf("nil: %s", got)
	}
	b := &Bot{config: &config.Config{BoardReviewModel: "cursor-composer"}}
	if got := trackerComposerModel(b); got != "cursor-composer" {
		t.Fatalf("cfg: %s", got)
	}
}

func TestTrackerAgentBoardUserIDUsesOwnerNotAuthor(t *testing.T) {
	b := &Bot{config: &config.Config{OwnerID: 99, AdminIDs: []int64{7}}}
	if got := b.trackerAgentBoardUserID(); got != 99 {
		t.Fatalf("owner: %d", got)
	}
	if got := (&Bot{}).trackerAgentBoardUserID(); got != 0 {
		t.Fatalf("empty: %d", got)
	}
}

func TestTrackerPipelineNotify(t *testing.T) {
	note := trackerPipelineNotify(database.TrackerTask{Num: 3})
	if !strings.Contains(note, "#3") || !strings.Contains(note, "выполнена") {
		t.Fatalf("note: %q", note)
	}
	if !strings.Contains(note, "запушь") {
		t.Fatalf("must mention push: %q", note)
	}
}

func TestTrackerNeedsAgentKick(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 50, 0, 0, time.UTC)
	failed := database.TrackerTask{
		Status:     "running",
		DevColumn:  "doing",
		Error:      "Агент не стартовал: unauthorized",
		Steps:      []string{"Взяли в работу по расписанию", "Агент не стартовал"},
		HasLastRun: true,
		LastRunAt:  now.Add(-time.Minute),
	}
	if !trackerNeedsAgentKick(failed, now, true) {
		t.Fatal("failed agent must retry on refresh")
	}
	freshClaim := database.TrackerTask{
		Status:     "running",
		DevColumn:  "doing",
		Steps:      []string{"Взяли в работу по расписанию"},
		HasLastRun: true,
		LastRunAt:  now.Add(-5 * time.Second),
	}
	if trackerNeedsAgentKick(freshClaim, now, true) {
		t.Fatal("fresh claim must not double-dispatch")
	}
	staleClaim := freshClaim
	staleClaim.LastRunAt = now.Add(-time.Hour)
	if !trackerNeedsAgentKick(staleClaim, now, false) {
		t.Fatal("stale claim without remote id must retry")
	}
	hasRemote := failed
	hasRemote.Steps = append(hasRemote.Steps, "агент:#88")
	if trackerNeedsAgentKick(hasRemote, now, true) {
		t.Fatal("already on remote board must not retry")
	}
}

func TestRemoteTrackerCreateUsesOwnTracker(t *testing.T) {
	var gotPath, gotSecret, gotWhen string
	var sourceID float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSecret = r.Header.Get("X-Tracker-Secret")
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		gotWhen, _ = body["when"].(string)
		sourceID, _ = body["source_task_id"].(float64)
		if _, ok := body["session"]; ok {
			t.Error("own tracker must not send MyVibeLab session")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"id":512,"when":"20.08 10:54"}`))
	}))
	defer srv.Close()

	b := &Bot{config: &config.Config{
		OwnerID:     99,
		BoardSecret: "secret",
		BoardRepo:   "fuserwyn/Fat-Leopard",
		BoardURL:    srv.URL,
	}}
	id, when, err := b.remoteTrackerCreate(database.TrackerTask{ID: 2, Num: 2, Prompt: "убрать огонёк"}, "doing", "")
	if err != nil {
		t.Fatal(err)
	}
	if id != 512 || when != "20.08 10:54" {
		t.Fatalf("id=%d when=%q", id, when)
	}
	if gotPath != trackerBoardAPI {
		t.Fatalf("path: %s", gotPath)
	}
	if gotSecret != "secret" {
		t.Fatalf("secret: %q", gotSecret)
	}
	if gotWhen != trackerRemoteWhen || sourceID != 2 {
		t.Fatalf("when=%q source=%v", gotWhen, sourceID)
	}
}

func TestDispatchTrackerAgentNilSafe(t *testing.T) {
	var b *Bot
	b.dispatchTrackerAgent(database.TrackerTask{ID: 1}, "doing")
	(&Bot{}).dispatchTrackerAgent(database.TrackerTask{ID: 1}, "review")
	(&Bot{}).kickTrackerPipeline(database.TrackerTask{ID: 1, DevColumn: trackerColReview})
}

func TestKickTrackerPipelineSkipsManualQa(t *testing.T) {
	b := &Bot{config: &config.Config{BoardURL: "https://example.test"}}
	// Без секрета dispatch сразу выйдет из горутины на session; главное —
	// ручное QA не должно запускать Composer-тест.
	b.kickTrackerPipeline(database.TrackerTask{
		ID: 1, DevColumn: trackerColTest, ManualQa: true,
	})
}
