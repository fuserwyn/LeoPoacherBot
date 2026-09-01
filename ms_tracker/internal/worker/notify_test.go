package worker

import (
	"strings"
	"testing"

	"leo-tracker/internal/store"
)

func TestTaskTitleFromPrompt(t *testing.T) {
	if got := taskTitleFromPrompt("Задача #40.\n\nНазвание в уведомлении"); got != "Название в уведомлении" {
		t.Fatalf("doing prompt: %q", got)
	}
	if got := taskTitleFromPrompt("Ревью задачи #5: ...\n\nФормулировка:\nСделай донат\n\nЧто сдал"); got != "Сделай донат" {
		t.Fatalf("review prompt: %q", got)
	}
}

func TestNotifyTextNoCodeDoesNotLookDone(t *testing.T) {
	text := notifyText(store.Job{SourceNum: 4, Phase: "doing"}, "Создам папку pink-leopard", "", "", false)
	if strings.Contains(text, "✅") || strings.Contains(text, "выполнена") {
		t.Fatalf("plan must not look completed: %q", text)
	}
	if !strings.Contains(text, noCodeMark) || !strings.Contains(text, "стенд") {
		t.Fatalf("must say no code / no stand: %q", text)
	}
}

func TestNotifyTextReviewFailWithoutCode(t *testing.T) {
	if got := noCodeVerdict("review", false); !strings.Contains(got, "ревью не принято") {
		t.Fatalf("review: %q", got)
	}
	if got := noCodeVerdict("test", false); !strings.Contains(got, "тест не прошёл") {
		t.Fatalf("test: %q", got)
	}
	if got := noCodeVerdict("doing", false); got != "" {
		t.Fatalf("doing stays empty: %q", got)
	}
}

func TestNotifyTextWithBranchIsImplementationCommit(t *testing.T) {
	text := notifyText(store.Job{SourceNum: 4, Phase: "doing", Prompt: "Задача #4.\n\nправка"}, "правка", "tracker/4-1", "abc1234", true)
	if !strings.Contains(text, "tracker/4-1") || !strings.Contains(text, "коммит выполнения") {
		t.Fatalf("branch: %q", text)
	}
	if !strings.Contains(text, "Задача #4: правка") {
		t.Fatalf("title: %q", text)
	}
	if strings.Contains(text, "запушь") || strings.Contains(text, "Автопуш") {
		t.Fatalf("push to stand is after test: %q", text)
	}
}

func TestNotifyTextFailedPushIsNotOnGithub(t *testing.T) {
	note := "Git: push: exit status 128: remote: Invalid username or token.\nfatal: Authentication failed for 'https://github.com/fuserwyn/Fat-Leopard.git/'"
	text := notifyText(store.Job{SourceNum: 4, Phase: "doing", AutoPush: true}, note, "tracker/4-14", "", false)
	if strings.Contains(text, "ветка ушла") || strings.Contains(text, "код в ветке") {
		t.Fatalf("failed push must not claim github: %q", text)
	}
	if !strings.Contains(text, "не попал в GitHub") || !strings.Contains(text, noCodeMark) {
		t.Fatalf("must say push failed: %q", text)
	}
}

func TestNoCodeWhenOnlyTrackerNoteCommitted(t *testing.T) {
	if got := noCodeVerdict("review", false); !strings.Contains(got, "ревью не принято") {
		t.Fatalf("note-only must fail review: %q", got)
	}
	text := notifyText(store.Job{SourceNum: 26, Phase: "doing"}, "Добавил 1 звезду", "tracker/26-76", "e5e0cf0", false)
	if !strings.Contains(text, noCodeMark) {
		t.Fatalf("note-only doing must not look like code: %q", text)
	}
}

func TestNotifyTextDoingCommitDoesNotShip(t *testing.T) {
	text := notifyText(store.Job{SourceNum: 4, Phase: "doing"}, "правка", "tracker/4-1", "def5678", true)
	if !strings.Contains(text, "def5678") || !strings.Contains(text, "после теста") {
		t.Fatalf("status commit: %q", text)
	}
}

func TestNotifyTextTestCommit(t *testing.T) {
	text := notifyText(store.Job{SourceNum: 6, Phase: "test"}, "Тест пройден.", "tracker/6-1", "abc1234", true)
	if !strings.Contains(text, "коммит abc1234 тест") {
		t.Fatalf("test stamp: %q", text)
	}
	if strings.Contains(text, "выполнена") || strings.Contains(text, "Railway") {
		t.Fatalf("test is not ship: %q", text)
	}
}
