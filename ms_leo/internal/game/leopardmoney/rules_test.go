package leopardmoney

import "testing"

func TestStreakAchievementIndex(t *testing.T) {
	cases := []struct {
		streak int
		want   int
	}{
		{7, 0},
		{14, 1},
		{30, 2},
		{60, 3},
		{0, -1},
		{6, -1},
		{8, -1},
		{21, -1}, // старый порог, больше не действует
		{28, -1}, // старый порог, больше не действует
		{100, -1},
	}
	for _, c := range cases {
		if got := StreakAchievementIndex(c.streak); got != c.want {
			t.Errorf("StreakAchievementIndex(%d) = %d, want %d", c.streak, got, c.want)
		}
	}
}

func TestLevelNames(t *testing.T) {
	expected := []string{"", "Сурикат", "Газель", "Зебра", "Гепард", "Лев", "Слон"}
	if len(LevelNames) != len(expected) {
		t.Fatalf("len(LevelNames) = %d, want %d", len(LevelNames), len(expected))
	}
	for i, name := range expected {
		if LevelNames[i] != name {
			t.Errorf("LevelNames[%d] = %q, want %q", i, LevelNames[i], name)
		}
	}
}

func TestLevelName(t *testing.T) {
	cases := []struct {
		level int
		want  string
	}{
		{1, "Сурикат"},
		{2, "Газель"},
		{3, "Зебра"},
		{4, "Гепард"},
		{5, "Лев"},
		{6, "Слон"},
		{0, ""},
		{7, ""},
		{-1, ""},
	}
	for _, c := range cases {
		if got := LevelName(c.level); got != c.want {
			t.Errorf("LevelName(%d) = %q, want %q", c.level, got, c.want)
		}
	}
}

func TestTrainingCupsFromParts_extra(t *testing.T) {
	cases := []struct {
		dur   int
		inten int
		cat   string
		want  int
	}{
		{10, 3, "run", 7},       // 10*3*1.2/5 = 7.2 → 7
		{1, 1, "yoga", 1},       // 1*1*0.8/5 = 0.16 → min 1
		{0, 1, "run", 1},        // d clamped to 1; 1*1*1.2/5 = 0.24 → min 1
		{500, 1, "run", 115},    // d clamped to 480; 480*1.2/5 = 115.2 → 115
		{25, 5, "hiit", 38},     // 25*5*1.5/5 = 37.5 → 38
		{20, 2, "walk", 6},      // 20*2*0.8/5 = 6.4 → 6
		{60, 5, "crossfit", 90}, // 60*5*1.5/5 = 90
	}
	for _, c := range cases {
		if got := TrainingCupsFromParts(c.dur, c.inten, c.cat); got != c.want {
			t.Errorf("TrainingCupsFromParts(%d, %d, %q) = %d, want %d", c.dur, c.inten, c.cat, got, c.want)
		}
	}
}
