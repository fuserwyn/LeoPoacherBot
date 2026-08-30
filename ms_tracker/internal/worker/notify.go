package worker

import (
	"fmt"
	"strings"

	"leo-tracker/internal/store"
)

const noCodeMark = "TRACKER_NO_CODE"

func noCodeVerdict(phase string, hasCode bool) string {
	if hasCode {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "review":
		return "ревью не принято: в репозитории нет коммита этой задачи. Агент сдал только текст."
	case "test":
		return "тест не прошёл: нечего прогонять, кода в репозитории нет."
	case "donate":
		return "донат не сделан: в репозитории нет коммита этой задачи."
	case "donate100":
		return "Донат 100 сделан: спасибо за поддержку!"
	default:
		return ""
	}
}

func notifyText(job store.Job, note, branch, commit string, hasCode bool) string {
	n := job.SourceNum
	if n <= 0 {
		n = int(job.ID)
	}
	note = clip(note, 1200)
	phase := strings.ToLower(strings.TrimSpace(job.Phase))
	if phase == "" {
		phase = "doing"
	}
	if job.Status == "donate100" {
		return "Донат 100 сделан: спасибо за поддержку!"
	}
	if phase == "review" || phase == "test" || phase == "donate" {
		if commit != "" {
			label := "ревью"
			switch phase {
			case "test":
				label = "тест"
			case "donate":
				label = "донейт"
			}
			return strings.TrimSpace(note + "\n\nкоммит " + commit + " " + label)
		}
		return note
	}
	if !hasCode {
		if gitFailed(note) {
			return fmt.Sprintf(
				"Задача #%d: коммит выполнения не попал в GitHub.\n\n%s\n\nВетки нет — review, тест и пуш на стенд не запускались.\n%s",
				n, note, noCodeMark,
			)
		}
		return fmt.Sprintf(
			"Задача #%d: агент сдал план, репозиторий не менялся.\n\n%s\n\nКода нет — review, тест и пуш на стенд не запускались.\n%s",
			n, note, noCodeMark,
		)
	}
	text := fmt.Sprintf("Задача #%d: коммит выполнения %s на ветке %s.\n\n%s", n, commit, branch, note)
	text += "\nСледующий шаг — review. Пуш на стенд только после теста."
	return text
}

func gitFailed(note string) bool {
	low := strings.ToLower(note)
	return strings.Contains(low, "git:") ||
		strings.Contains(low, "authentication failed") ||
		strings.Contains(low, "invalid username or token")
}

func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
