package bot

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLeopardOnboardingBody_KeySections(t *testing.T) {
	s := leopardOnboardingBody()
	if s != leopardOnboardingBodyText {
		t.Fatal("leopardOnboardingBody() must return leopardOnboardingBodyText")
	}
	if strings.TrimSpace(s) != s {
		t.Fatal("onboarding body must not have leading/trailing whitespace")
	}
	if utf8.RuneCountInString(s) < 200 {
		t.Fatalf("onboarding body unexpectedly short (%d runes)", utf8.RuneCountInString(s))
	}

	checks := []string{
		"Добро пожаловать в стаю, Fat Leopard",
		"⚡️ КАК ОТМЕТИТЬ ТРЕНИРОВКУ",
		"мини-апп",
		"🏆 КУБКИ И СТРИК",
		"начисляются кубки по формуле",
		"⏰ ЧТО БУДЕТ, ЕСЛИ ПРОПУСКАТЬ",
		"День 5 без тренировки",
		"День 6",
		"День 7 — кубки обнуляются",
		"День 8 — удаление из стаи",
		"/start в личке",
		"🏆 АЧИВКИ",
		"100 дней",
		"Заморозку",
		"❄️ ПЛАТНАЯ ЗАМОРОЗКА",
		"42 ₽ за 7 дней",
		"🔄 ВЕРНУТЬСЯ В СТАЮ",
		"210 ₽",
		"🎯 Начни прямо сейчас — отметь тренировку в мини-аппе",
	}
	for _, c := range checks {
		if !strings.Contains(s, c) {
			t.Fatalf("onboarding body missing %q\n---\n%s\n---", c, s)
		}
	}
	if strings.Contains(s, "#training_done") {
		t.Fatal("onboarding must not mention #training_done")
	}
}

func TestWelcomeStartTextUsesLeopardOnboarding(t *testing.T) {
	if welcomeStartText() != leopardOnboardingBody() {
		t.Fatal("welcomeStartText must equal leopardOnboardingBody()")
	}
}

func TestSendWelcomeMessage_OnboardingFormat(t *testing.T) {
	const username = "testuser"
	got := username + "\n\n" + leopardOnboardingBody()
	wantPrefix := username + "\n\n"
	if !strings.HasPrefix(got, wantPrefix) {
		preview := got
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		t.Fatalf("expected prefix %q, got %q", wantPrefix, preview)
	}
	if !strings.Contains(got, "Fat Leopard") {
		t.Fatal("welcome DM must include onboarding body")
	}
}
