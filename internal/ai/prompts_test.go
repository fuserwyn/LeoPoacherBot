package ai

import (
	"strings"
	"testing"
)

func TestBuildUserPrompt(t *testing.T) {
	out := BuildUserPrompt("сколько у меня кубков?", "👤 ivan | 🏆 кубки: 15")
	for _, want := range []string{
		"Вопрос пользователя: сколько у меня кубков?",
		"=== ПОЛНЫЙ КОНТЕКСТ ПОЛЬЗОВАТЕЛЯ ===",
		"👤 ivan | 🏆 кубки: 15",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in prompt:\n%s", want, out)
		}
	}
}

func TestFatLeopardSystemPrompt(t *testing.T) {
	p := FatLeopardSystemPrompt()
	for _, want := range []string{
		"ИСТОЧНИКИ ПРАВДЫ",
		"БАНТЕР",
		"#training_done",
		"тренер-леопард",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("missing %q in system prompt", want)
		}
	}
	if strings.Contains(p, "### Пользователи") {
		t.Fatal("system prompt should not use sectioned markdown headers from new template")
	}
}
