package bot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"leo-bot/internal/config"
)

func TestPingTrackerWireRequiresNotify(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"service":"ms_tracker","wire":{"notify":false}}`))
	}))
	defer srv.Close()
	b := &Bot{config: &config.Config{BoardURL: srv.URL, BoardSecret: "sec"}}
	err := b.pingTrackerWire()
	if err == nil || !strings.Contains(err.Error(), "LEO_NOTIFY_URL") {
		t.Fatalf("must refuse silent tracker: %v", err)
	}
}

func TestPingTrackerWireOldHealthOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"service":"ms_tracker"}`))
	}))
	defer srv.Close()
	b := &Bot{config: &config.Config{BoardURL: srv.URL, BoardSecret: "sec"}}
	if err := b.pingTrackerWire(); err != nil {
		t.Fatal(err)
	}
}

func TestPingTrackerWireOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"service":"ms_tracker","wire":{"notify":true,"railway":true}}`))
	}))
	defer srv.Close()
	b := &Bot{config: &config.Config{BoardURL: srv.URL, BoardSecret: "sec"}}
	if err := b.pingTrackerWire(); err != nil {
		t.Fatal(err)
	}
}

func TestWaitStandBuildRequiresPinned(t *testing.T) {
	b := &Bot{config: &config.Config{RailwayToken: "t", RailwayProjectID: "p"}}
	if err := b.waitStandBuild(time.Now(), nil); err == nil {
		t.Fatal("без заказанной сборки карточку закрывать нельзя")
	}
}
