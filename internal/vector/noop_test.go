package vector

import "testing"

func TestNoopStore(t *testing.T) {
	s := NewNoopStore()
	if s.Enabled() {
		t.Fatal("noop store must be disabled")
	}
	if err := s.EnsureCollection(); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	if err := s.UpsertMessage(ChatPoint{MessageID: 1, MessageText: "hi"}); err != nil {
		t.Fatalf("UpsertMessage: %v", err)
	}
	hits, err := s.SearchChat(42, "test", 10)
	if err != nil {
		t.Fatalf("SearchChat: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected no hits, got %d", len(hits))
	}
}
