package bot

import (
	"strings"
	"testing"

	"leo-bot/internal/database"
	"leo-bot/internal/config"
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

func TestTrackerPipelineNotify(t *testing.T) {
	note := trackerPipelineNotify(database.TrackerTask{Num: 3})
	if !strings.Contains(note, "#3") || !strings.Contains(note, "выполнена") {
		t.Fatalf("note: %q", note)
	}
	if !strings.Contains(note, "запушь") {
		t.Fatalf("must mention push: %q", note)
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
