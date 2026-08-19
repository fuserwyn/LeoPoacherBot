package trainingmap

import "testing"

func TestSnapshotForEmpty(t *testing.T) {
	s := SnapshotFor(0)
	if s.Completed != 0 || s.Remaining != 8 || s.NextIndex != 0 || s.Lap != 1 {
		t.Fatalf("empty map: %+v", s)
	}
	if s.Nodes[0].Status != "next" || s.Nodes[1].Status != "remaining" {
		t.Fatalf("first node should be next, got %s / %s", s.Nodes[0].Status, s.Nodes[1].Status)
	}
}

func TestSnapshotForProgressAndLap(t *testing.T) {
	s := SnapshotFor(3)
	if s.Completed != 3 || s.Remaining != 5 || s.NextIndex != 3 {
		t.Fatalf("progress: %+v", s)
	}
	if s.Nodes[0].Status != "done" || s.Nodes[2].Status != "done" || s.Nodes[3].Status != "next" {
		t.Fatalf("statuses: %+v", []string{s.Nodes[0].Status, s.Nodes[2].Status, s.Nodes[3].Status})
	}
	if s.Nodes[3].ID != "walk" {
		t.Fatalf("next id = %s", s.Nodes[3].ID)
	}

	full := SnapshotFor(8)
	if full.Completed != 0 || full.Lap != 2 || full.Nodes[0].Status != "next" {
		t.Fatalf("new lap after 8: %+v", full)
	}
}

func TestSnapshotForNegative(t *testing.T) {
	s := SnapshotFor(-4)
	if s.WorkoutsTotal != 0 || s.NextIndex != 0 {
		t.Fatalf("neg: %+v", s)
	}
}
