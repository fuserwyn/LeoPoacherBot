package bot

import "testing"

func TestClassifyAIQuery(t *testing.T) {
	cases := []struct {
		q       string
		visual  bool
		want    QueryKind
		skipSem bool
		minimal bool
	}{
		{"", false, QueryBanter, true, true},
		{"привет", false, QueryBanter, true, true},
		{"кто ты", false, QueryIdentity, true, true},
		{"сколько у меня кубков?", false, QueryFactual, false, false},
		{"придумай шутку про леопарда", false, QueryCreative, true, false},
		{"", true, QueryVision, false, false},
	}
	for _, tc := range cases {
		r := classifyAIQuery(tc.q, tc.visual)
		if r.Kind != tc.want || r.SkipSemantic != tc.skipSem || r.MinimalContext != tc.minimal {
			t.Errorf("q=%q visual=%v: got kind=%s skip=%v min=%v, want kind=%s skip=%v min=%v",
				tc.q, tc.visual, r.Kind, r.SkipSemantic, r.MinimalContext,
				tc.want, tc.skipSem, tc.minimal)
		}
	}
}

func TestFilterContextChunksDropsBot(t *testing.T) {
	chunks := []contextChunk{
		{Source: "thread", Text: "user said hi", MessageID: 1, UserID: 10},
		{Source: "thread", Text: "bot reply", MessageID: 2, UserID: 99, MessageType: "ai_reply"},
		{Source: "semantic", Text: "user said hi", MessageID: 1, UserID: 10},
	}
	got := filterContextChunks(chunks)
	if len(got) != 1 {
		t.Fatalf("expected 1 chunk after filter, got %d", len(got))
	}
	if got[0].UserID != 10 {
		t.Fatalf("expected user chunk, got userID=%d", got[0].UserID)
	}
}

func TestSanitizeAIReplyDropsWisdom(t *testing.T) {
	wisdom := "Сегодняшний день — это возможность проявить терпение."
	if got := sanitizeAIReply(wisdom); got != "" {
		t.Fatalf("expected empty wisdom reply, got %q", got)
	}
	if !shouldPersistAIReply(wisdom) {
		return
	}
	t.Fatal("wisdom should not be persisted")
}

func TestShouldPersistAIReplyRejectsLoop(t *testing.T) {
	loop := "Продолжай в том же духе и не сбавляй темп. Продолжай в том же духе и не сбавляй темп."
	if shouldPersistAIReply(loop) {
		t.Fatal("expected loop reply not to persist")
	}
}
