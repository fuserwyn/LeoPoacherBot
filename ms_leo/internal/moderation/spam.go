package moderation

import (
	"regexp"
	"strings"
)

var wordRe = regexp.MustCompile(`\p{L}+`)

// hasExcessiveWordRepetition ловит спам/мусор по повторам слов, но НЕ душит
// естественные повторы в осмысленных заметках. Раньше правило было слишком строгим:
// любое слово, встретившееся 3 раза где угодно в тексте, блокировало публикацию
// (ложные срабатывания на нормальных комментариях вроде «бегал… бегал… снова бегал»).
//
// Теперь блокируем только явный спам:
//   - 3+ одинаковых слова ПОДРЯД («хищник хищник хищник», «wow wow wow wow»);
//   - повтор целой ФРАЗЫ — два и более разных слова, каждое встретилось 3+ раза
//     («Держи ритм, хищник. Держи ритм, хищник…»);
//   - одно слово ДОМИНИРУЕТ над текстом (>=4 раз и это >=50% значимых слов).
//
// Одиночное слово, естественно повторённое 3 раза в разнообразном тексте, проходит.
func hasExcessiveWordRepetition(text string) bool {
	all := wordRe.FindAllString(strings.ToLower(text), -1)
	words := make([]string, 0, len(all))
	for _, w := range all {
		if len(w) < 2 { // пропускаем односимвольные ascii-слова, как и раньше
			continue
		}
		words = append(words, w)
	}
	if len(words) == 0 {
		return false
	}

	counts := make(map[string]int, len(words))
	maxCount := 0
	maxRun := 1 // максимальная серия одинаковых слов подряд
	run := 1
	for i, w := range words {
		counts[w]++
		if counts[w] > maxCount {
			maxCount = counts[w]
		}
		if i > 0 && words[i] == words[i-1] {
			run++
			if run > maxRun {
				maxRun = run
			}
		} else {
			run = 1
		}
	}

	// 3+ одинаковых слова подряд — явный спам.
	if maxRun >= 3 {
		return true
	}

	// Повтор целой фразы: два и более разных слова по 3+ повтора.
	wordsRepeated3 := 0
	for _, c := range counts {
		if c >= 3 {
			wordsRepeated3++
		}
	}
	if wordsRepeated3 >= 2 {
		return true
	}

	// Одно слово доминирует над всем текстом (мусор/спам).
	if maxCount >= 4 && maxCount*2 >= len(words) {
		return true
	}

	return false
}
