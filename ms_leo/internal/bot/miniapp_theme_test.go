package bot

import "testing"

func TestNormalizeMiniappTheme(t *testing.T) {
	got, ok := NormalizeMiniappTheme("leopard", 5)
	if !ok || got != "leopard" {
		t.Fatalf("leo: %q %v", got, ok)
	}
	got, ok = NormalizeMiniappTheme("leopard", 4)
	if !ok || got != "dark" {
		t.Fatalf("lock: %q %v", got, ok)
	}
	got, ok = NormalizeMiniappTheme("light", 1)
	if !ok || got != "light" {
		t.Fatalf("light: %q %v", got, ok)
	}
	if _, ok = NormalizeMiniappTheme("neon", 6); ok {
		t.Fatal("invalid must fail")
	}
}
