package bot

import (
	"testing"

	"leo-bot/internal/database"
)

func TestTrainingThreadParentIsLeo(t *testing.T) {
	t.Parallel()
	if !trainingThreadParentIsLeo(database.TrainingFeedThreadRow{FromUserID: 0}) {
		t.Fatal("system Leo (from_user_id=0) must count")
	}
	if !trainingThreadParentIsLeo(database.TrainingFeedThreadRow{FromUserID: 42, PostedAs: "leo"}) {
		t.Fatal("admin voice «leo» must count")
	}
	if trainingThreadParentIsLeo(database.TrainingFeedThreadRow{FromUserID: 42, PostedAs: "self"}) {
		t.Fatal("regular participant must not count as Leo")
	}
	if trainingThreadParentIsLeo(database.TrainingFeedThreadRow{FromUserID: 42, PostedAs: "admin"}) {
		t.Fatal("admin voice must not count as Leo")
	}
}

func TestShouldLeoReplyInFeedThread(t *testing.T) {
	t.Parallel()
	if !shouldLeoReplyInFeedThread("training_done", false, true, false) {
		t.Fatal("reply to Leo under a report")
	}
	if !shouldLeoReplyInFeedThread("sick_leave", false, true, false) {
		t.Fatal("reply to Leo in a non-report thread")
	}
	if !shouldLeoReplyInFeedThread("admin_post", false, false, true) {
		t.Fatal("@leo mention in an announcement thread")
	}
	if shouldLeoReplyInFeedThread("training_done", true, true, true) {
		t.Fatal("official voice must not trigger Leo AI")
	}
	if shouldLeoReplyInFeedThread("pack_join", false, true, true) {
		t.Fatal("cards without a thread must not trigger Leo")
	}
	if shouldLeoReplyInFeedThread("training_done", false, false, false) {
		t.Fatal("plain comment without @leo / reply-to-Leo")
	}
}

func TestFeedThreadLeoPromptKind(t *testing.T) {
	t.Parallel()
	if got := feedThreadLeoPromptKind("sick_leave"); got != "заявкой на больничный" {
		t.Fatalf("sick_leave: %q", got)
	}
	if got := feedThreadLeoPromptKind("training_done"); got != "отчётом о тренировке" {
		t.Fatalf("training_done: %q", got)
	}
}

func TestFeedThreadAuthorLabel(t *testing.T) {
	t.Parallel()
	if got := feedThreadAuthorLabel(database.TrainingFeedThreadRow{FromUserID: 0, Username: "x"}); got != "Лео" {
		t.Fatalf("system Leo: %q", got)
	}
	if got := feedThreadAuthorLabel(database.TrainingFeedThreadRow{FromUserID: 9, PostedAs: "leo", Username: "админ"}); got != "Лео" {
		t.Fatalf("posted_as leo: %q", got)
	}
	if got := feedThreadAuthorLabel(database.TrainingFeedThreadRow{FromUserID: 9, PostedAs: "admin", Username: "админ"}); got != "Админ" {
		t.Fatalf("posted_as admin: %q", got)
	}
	if got := feedThreadAuthorLabel(database.TrainingFeedThreadRow{FromUserID: 9, Username: "Аня"}); got != "Аня" {
		t.Fatalf("participant: %q", got)
	}
}
