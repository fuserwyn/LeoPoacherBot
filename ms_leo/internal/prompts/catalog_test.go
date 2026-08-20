package prompts

import "testing"

func TestCatalogHasEmbedFiles(t *testing.T) {
	if len(Catalog()) != 12 {
		t.Fatalf("slots: %d", len(Catalog()))
	}
	for _, s := range Catalog() {
		if s.embedded == "" || s.File == "" {
			t.Fatalf("empty %s", s.Key)
		}
	}
}

func TestApplyOverrides(t *testing.T) {
	base := DefaultBundle()
	got := ApplyOverrides(base, map[string]string{"answer_user_question": "новый характер"})
	if got.AnswerUserQuestion != "новый характер" {
		t.Fatal(got.AnswerUserQuestion)
	}
	if got.DailySummary != base.DailySummary {
		t.Fatal("other fields stay")
	}
}
