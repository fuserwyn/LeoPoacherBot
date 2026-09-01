package railway

import (
	"testing"
	"time"
)

func TestPickInFlight(t *testing.T) {
	since := time.Date(2026, 9, 1, 6, 20, 0, 0, time.UTC)
	got := PickInFlight([]Deploy{
		{ID: "old", Status: "BUILDING", CreatedAt: since.Add(-5 * time.Minute)},
		{ID: "hook", Status: "DEPLOYING", CreatedAt: since.Add(10 * time.Second)},
		{ID: "done", Status: "SUCCESS", CreatedAt: since.Add(20 * time.Second)},
	}, since)
	if got != "hook" {
		t.Fatalf("reuse webhook: %q", got)
	}
	if PickInFlight(nil, since) != "" {
		t.Fatal("empty")
	}
}

func TestPinStartedReusesWebhookNotSecondAPI(t *testing.T) {
	since := time.Date(2026, 9, 1, 6, 20, 0, 0, time.UTC)
	svcs := []Service{{Name: "MiniApp"}, {Name: "ms_leo"}}
	pinned := map[string]string{}
	deploys := map[string][]Deploy{
		"ms_leo": {
			{ID: "hook-leo", Status: "DEPLOYING", CreatedAt: since.Add(time.Second)},
		},
		"MiniApp": {
			{ID: "hook-app", Status: "SKIPPED", CreatedAt: since.Add(2 * time.Second)},
		},
	}
	if !PinStarted(svcs, pinned, deploys, since) {
		t.Fatalf("оба вебхука уже есть: %+v", pinned)
	}
	if pinned["ms_leo"] != "hook-leo" || pinned["MiniApp"] != "hook-app" {
		t.Fatalf("пин: %+v", pinned)
	}
	if PickStarted(nil, since) != "" {
		t.Fatal("пустой список — API ещё можно заказать")
	}
}

func TestIsAppService(t *testing.T) {
	for _, name := range []string{"MiniApp", "miniapp-main", "ms_leo", "ms-leo-main", "leo", "fat-leopard-main"} {
		if !IsAppService(name) {
			t.Fatalf("watch %q", name)
		}
	}
	for _, name := range []string{"ms_tracker", "ms-tracker-main", "Postgres", "Redis", "ms_payments"} {
		if IsAppService(name) {
			t.Fatalf("skip %q", name)
		}
	}
}

func TestIsSelfService(t *testing.T) {
	if !IsSelfService("ms-leo-main") || !IsSelfService("fat-leopard-main") || IsSelfService("MiniApp") {
		t.Fatal("self")
	}
}

func TestPickEnvironmentID(t *testing.T) {
	envs := []Env{{ID: "e1", Name: "staging"}, {ID: "e2", Name: "production"}}
	if got := PickEnvironmentID("main", "", envs); got != "e2" {
		t.Fatalf("production over first: %s", got)
	}
	if got := PickEnvironmentID("", "e1", envs); got != "e1" {
		t.Fatalf("explicit id: %s", got)
	}
}

func TestQueueSelfLast(t *testing.T) {
	got := QueueSelfLast([]Service{{ID: "1", Name: "ms-leo-main"}, {ID: "2", Name: "MiniApp"}})
	if len(got) != 2 || got[0].Name != "MiniApp" || got[1].Name != "ms-leo-main" {
		t.Fatalf("%+v", got)
	}
}

func TestParseDeployID(t *testing.T) {
	if got := parseDeployID([]byte(`{"serviceInstanceDeploy":"dep-1"}`)); got != "dep-1" {
		t.Fatalf("string: %s", got)
	}
	if got := parseDeployID([]byte(`{"serviceInstanceDeployV2":{"id":"dep-2"}}`)); got != "dep-2" {
		t.Fatalf("obj: %s", got)
	}
	if got := parseDeployID([]byte(`{"serviceInstanceDeploy":true}`)); got != "" {
		t.Fatalf("bool: %s", got)
	}
}
