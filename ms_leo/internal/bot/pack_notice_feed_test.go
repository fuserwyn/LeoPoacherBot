package bot

import (
	"strings"
	"testing"

	"leo-bot/internal/prompts"
)

func TestPackRemovedFeedNotice_ForPeers(t *testing.T) {
	p := prompts.DefaultBundle()
	s := PackRemovedFeedNotice(p, "@tester")
	t.Log(s)
	if strings.Contains(s, "возвращайся, когда") {
		t.Fatal("avoid direct 'ты-возвращайся' leftover")
	}
	if !strings.Contains(s, "tester") {
		t.Fatal("missing display name fragment")
	}
}
