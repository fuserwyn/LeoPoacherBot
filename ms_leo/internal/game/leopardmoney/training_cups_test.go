package leopardmoney

import "testing"

func TestTrainingCupsFromParts_specExamples(t *testing.T) {
	cases := []struct {
		name     string
		d, in    int
		cat      string
		wantCups int
	}{
		{"1min int1 strength", 1, 1, "strength", 1},
		{"5min int1 yoga", 5, 1, "yoga", 1},
		{"30min int3 yoga", 30, 3, "yoga", 14},
		{"60min int3 strength", 60, 3, "strength", 36},
		{"45min int4 strength", 45, 4, "strength", 36},
		{"60min int4 run", 60, 4, "run", 58},
		{"30min int5 hiit", 30, 5, "hiit", 45},
		{"90min int5 hiit", 90, 5, "hiit", 135},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TrainingCupsFromParts(tc.d, tc.in, tc.cat)
			if got != tc.wantCups {
				t.Fatalf("got %d want %d", got, tc.wantCups)
			}
		})
	}
}

func TestParseTrainingDoneReport(t *testing.T) {
	text := "силовая, 60 мин, инт. 3/5\nnotes"
	d, in, cat, ok := ParseTrainingDoneReport(text)
	if !ok || d != 60 || in != 3 || cat != "strength" {
		t.Fatalf("got ok=%v d=%d in=%d cat=%q", ok, d, in, cat)
	}
	if TrainingCupsFromParts(d, in, cat) != 36 {
		t.Fatal("cups for 60/3/strength")
	}
	legacy := "#training_done — бег, 15 мин, инт. 2/5"
	d2, in2, cat2, ok2 := ParseTrainingDoneReport(legacy)
	if !ok2 || d2 != 15 || in2 != 2 || cat2 != "run" {
		t.Fatalf("legacy parse: ok=%v d=%d in=%d cat=%q", ok2, d2, in2, cat2)
	}
}

func TestLevelFromTotalCups(t *testing.T) {
	if LevelFromTotalCups(0) != 1 || LevelFromTotalCups(419) != 1 {
		t.Fatal("L1")
	}
	if LevelFromTotalCups(420) != 2 || LevelFromTotalCups(1259) != 2 {
		t.Fatal("L2")
	}
	if LevelFromTotalCups(26460) != 7 {
		t.Fatalf("L7 boundary got %d", LevelFromTotalCups(26460))
	}
}
