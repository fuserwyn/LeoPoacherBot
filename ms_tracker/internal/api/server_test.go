package api

import "testing"

func TestUnwrapScheduledBody(t *testing.T) {
	direct := map[string]any{"when": "сейчас", "prompt": "починить клавиатуру"}
	if got := unwrapScheduledBody(direct); got["prompt"] != "починить клавиатуру" {
		t.Fatalf("direct: %#v", got)
	}

	legacy := map[string]any{
		"session": "x.y",
		"op":      "create",
		"task_id": 7,
		"payload": map[string]any{
			"when":   "сейчас",
			"prompt": "починить клавиатуру",
		},
	}
	got := unwrapScheduledBody(legacy)
	if got["prompt"] != "починить клавиатуру" || got["when"] != "сейчас" {
		t.Fatalf("legacy: %#v", got)
	}
	if _, ok := got["session"]; ok {
		t.Fatal("session must stay in the envelope, not the job")
	}
}
