package miniappapi

import (
	"encoding/json"
	"testing"
)

func TestParseJSONInt64(t *testing.T) {
	if got := parseJSONInt64(json.RawMessage(`235`)); got != 235 {
		t.Fatalf("number: got %d", got)
	}
	if got := parseJSONInt64(json.RawMessage(`"235"`)); got != 235 {
		t.Fatalf("string: got %d", got)
	}
	if got := parseJSONInt64(nil); got != 0 {
		t.Fatalf("empty: got %d", got)
	}
}
