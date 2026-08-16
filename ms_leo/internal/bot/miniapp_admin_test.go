package bot

import "testing"

func TestNormalizeAdminPostAuthor(t *testing.T) {
	if got := normalizeAdminPostAuthor("leo"); got != adminPostAuthorLeo {
		t.Fatalf("leo: got %q", got)
	}
	if got := normalizeAdminPostAuthor(" admin "); got != adminPostAuthorAdmin {
		t.Fatalf("admin: got %q", got)
	}
	if got := normalizeAdminPostAuthor(""); got != adminPostAuthorAdmin {
		t.Fatalf("empty should default to admin, got %q", got)
	}
	if got := normalizeAdminPostAuthor("Лео"); got != adminPostAuthorAdmin {
		t.Fatalf("unknown should default to admin, got %q", got)
	}
}

func TestIsAllowedMiniappAdminUserAction(t *testing.T) {
	allowed := []string{"sick_set", "sick_cancel", "restore_full", "restore_scratch", "mute", "unmute", "grant_save", "kick"}
	for _, a := range allowed {
		if !isAllowedMiniappAdminUserAction(a) {
			t.Fatalf("%s should be allowed", a)
		}
	}
	for _, a := range []string{"", "purge", "cups_add", "DELETE"} {
		if isAllowedMiniappAdminUserAction(a) {
			t.Fatalf("%q must be rejected", a)
		}
	}
}
