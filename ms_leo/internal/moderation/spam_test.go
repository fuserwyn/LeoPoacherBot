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
		// Ослабление: одиночное слово, естественно повторённое 3 раза в разнообразном
		// тексте, больше НЕ блокируется (раньше это было ложным срабатыванием).
		{"single word 3x in varied note", "Сегодня бегал утром, потом бегал в обед и вечером снова бегал, доволен", false},
		{"single word 3x non-consecutive", "качал спину, качал ноги, качал плечи на тренировке", false},
		// Но одно доминирующее слово (>=4 и >=50%) — всё ещё спам.
		{"single word dominates", "скидка тут скидка там скидка везде скидка всем скидка", true},
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
