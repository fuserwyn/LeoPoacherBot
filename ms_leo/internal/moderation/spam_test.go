package moderation

import "testing"

func TestExcessiveWordRepetition(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		hasSpam  bool
	}{
		{"normal text", "Хороший воркаут, молодец!", false},
		{"two repetitions", "Хищник воркаут хищник", false},
		{"three same word", "Держи ритм, хищник. Держи ритм, хищник. Держи ритм!", true},
		{"short text with repeat", "хищник хищник хищник", true},
		{"repeated word 3 times lowercase", "test test test hello", true},
		{"repeated word 4 times", "wow wow wow wow", true},
		{"mixed case repetition", "Hello HELLO hello", true},
		{"less than 4 words", "a a a", false},
		{"no significant repetition", "one two three four five six", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasExcessiveWordRepetition(tt.text)
			if result != tt.hasSpam {
				t.Errorf("hasExcessiveWordRepetition(%q) = %v, want %v", tt.text, result, tt.hasSpam)
			}
		})
	}
}
