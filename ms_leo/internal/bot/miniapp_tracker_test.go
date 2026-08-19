package bot

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"leo-bot/internal/config"
)

func decodeTrackerSessionPayload(t *testing.T, sess string) map[string]any {
	t.Helper()
	parts := strings.SplitN(sess, ".", 2)
	if len(parts) != 2 {
		t.Fatalf("session must be payload.sig, got %q", sess)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	return out
}

func TestTrackerSessionOmitsEmptyBranch(t *testing.T) {
	b := &Bot{config: &config.Config{
		BoardSecret: "secret",
		BoardRepo:   "fuserwyn/Fat-Leopard",
		BoardURL:    "https://example.test",
	}}
	sess, err := b.trackerSession(42, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	payload := decodeTrackerSessionPayload(t, sess)
	if _, ok := payload["b"]; ok {
		t.Fatalf("empty BOARD_BRANCH must not be in session, got %#v", payload["b"])
	}
	if payload["r"] != "fuserwyn/Fat-Leopard" {
		t.Fatalf("repo: %#v", payload["r"])
	}
}

func TestTrackerSessionKeepsLabBranch(t *testing.T) {
	b := &Bot{config: &config.Config{
		BoardSecret: "secret",
		BoardRepo:   "fuserwyn/Fat-Leopard",
		BoardURL:    "https://example.test",
		BoardBranch: "leo-lab",
	}}
	sess, err := b.trackerSession(1, "Leo")
	if err != nil {
		t.Fatal(err)
	}
	payload := decodeTrackerSessionPayload(t, sess)
	if payload["b"] != "leo-lab" {
		t.Fatalf("lab branch: %#v", payload["b"])
	}
}

func TestParseTrackerTaskSnapshotWrapAndFlat(t *testing.T) {
	wrap, err := parseTrackerTaskSnapshot([]byte(`{"task":{"status":"done","commit":"abc","branch":"feat"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if wrap.Status != "done" || wrap.Commit != "abc" || wrap.Branch != "feat" {
		t.Fatalf("wrap: %+v", wrap)
	}
	flat, err := parseTrackerTaskSnapshot([]byte(`{"status":"done","commit":"def"}`))
	if err != nil {
		t.Fatal(err)
	}
	if flat.Commit != "def" {
		t.Fatalf("flat: %+v", flat)
	}
}

func TestNormalizeTrackerRescheduleNow(t *testing.T) {
	p := map[string]any{"id": float64(7), "mode": "now"}
	normalizeTrackerReschedule("reschedule", p)
	if p["when"] != "через 1 мин" {
		t.Fatalf("when: %#v", p["when"])
	}
	if _, ok := p["mode"]; ok {
		t.Fatalf("mode must be dropped, got %#v", p["mode"])
	}
	keep := map[string]any{"id": float64(1), "when": "завтра 4:20"}
	normalizeTrackerReschedule("reschedule", keep)
	if keep["when"] != "завтра 4:20" {
		t.Fatalf("explicit when must stay, got %#v", keep["when"])
	}
	normalizeTrackerReschedule("cancel", p)
}

func TestTrackerPayloadTaskID(t *testing.T) {
	if got := trackerPayloadTaskID(9, map[string]any{"id": float64(3)}); got != 9 {
		t.Fatalf("explicit id wins, got %d", got)
	}
	if got := trackerPayloadTaskID(0, map[string]any{"id": float64(7)}); got != 7 {
		t.Fatalf("payload id, got %d", got)
	}
	if got := trackerPayloadTaskID(0, nil); got != 0 {
		t.Fatalf("empty, got %d", got)
	}
}
