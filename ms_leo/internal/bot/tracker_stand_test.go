package bot

import (
	"testing"
	"time"

	"leo-bot/internal/database"
)

func TestStandWaitDecision(t *testing.T) {
	started := time.Date(2026, 8, 20, 21, 0, 0, 0, time.UTC)
	since := started.Add(-45 * time.Second)

	done, fail := standWaitDecision([]standDeploy{
		{Status: "SUCCESS", CreatedAt: started.Add(20 * time.Second)},
	}, since, started, started.Add(20*time.Second))
	if fail != nil || !done {
		t.Fatalf("fresh success: %v %v", done, fail)
	}

	done, fail = standWaitDecision([]standDeploy{
		{Status: "FAILED", CreatedAt: started.Add(10 * time.Second)},
	}, since, started, started.Add(10*time.Second))
	if fail == nil || done {
		t.Fatal("fresh fail must error")
	}

	done, fail = standWaitDecision([]standDeploy{
		{Status: "DEPLOYING", CreatedAt: started.Add(5 * time.Second)},
		{Status: "SUCCESS", CreatedAt: started.Add(-2 * time.Minute)},
	}, since, started, started.Add(2*time.Minute))
	if fail != nil || done {
		t.Fatalf("in-flight must wait: %v %v", done, fail)
	}

	old := started.Add(-3 * time.Minute)
	done, fail = standWaitDecision([]standDeploy{
		{Status: "SUCCESS", CreatedAt: old},
		{Status: "SKIPPED", CreatedAt: started.Add(10 * time.Second)},
	}, since, started, started.Add(10*time.Second))
	if fail != nil || done {
		t.Fatalf("grace: %v %v", done, fail)
	}
	done, fail = standWaitDecision([]standDeploy{
		{Status: "SUCCESS", CreatedAt: old},
		{Status: "SKIPPED", CreatedAt: started.Add(10 * time.Second)},
	}, since, started, started.Add(trackerStandSkipGrace+time.Second))
	if fail != nil || !done {
		t.Fatalf("already live: %v %v", done, fail)
	}
}

func TestTrackerTaskShippedToStand(t *testing.T) {
	if trackerTaskShippedToStand(database.TrackerTask{Steps: []string{"тест пройден"}}) {
		t.Fatal("test is not ship")
	}
	if !trackerTaskShippedToStand(database.TrackerTask{Steps: []string{"пуш в main", "ждём сборку на стенде"}}) {
		t.Fatal("shipped")
	}
}

func TestTryBeginTrackerStandOnce(t *testing.T) {
	if !tryBeginTrackerStand(42) {
		t.Fatal("first")
	}
	if tryBeginTrackerStand(42) {
		t.Fatal("second must skip")
	}
	endTrackerStand(42)
	if !tryBeginTrackerStand(42) {
		t.Fatal("after end")
	}
	endTrackerStand(42)
}
