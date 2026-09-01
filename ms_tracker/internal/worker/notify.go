package worker

import (
	"fmt"
	"regexp"
	"strings"

	"leo-tracker/internal/store"
)

const noCodeMark = "TRACKER_NO_CODE"

var (
	trackerSprintPrefixRe = regexp.MustCompile(`(?i)^\[Спринт\s+\d+\]\s*`)
	trackerTaskNumLineRe  = regexp.MustCompile(`(?i)^Задача\s*#\s*\d+\.?$`)
)

func jobNotifyLabel(job store.Job) string {
	n := job.SourceNum
	if n <= 0 {
		n = int(job.ID)
	}
	return notifyTaskLabel(n, taskTitleFromPrompt(job.Prompt))
}

func notifyTaskLabel(n int, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Sprintf("Задача #%d", n)
	}
	return fmt.Sprintf("Задача #%d: %s", n, title)
}

func taskTitleFromPrompt(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ""
	}
	if idx := strings.Index(prompt, "Формулировка:\n"); idx >= 0 {
		rest := strings.TrimSpace(prompt[idx+len("Формулировка:\n"):])
		if end := strings.Index(rest, "\n\n"); end >= 0 {
			return clipTitle(rest[:end])
		}
		return clipTitle(rest)
	}
	if m := trackerSprintPrefixRe.FindString(prompt); m != "" {
		prompt = strings.TrimSpace(prompt[len(m):])
	}
	for _, line := range strings.Split(prompt, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if trackerTaskNumLineRe.MatchString(line) {
			continue
		}
		return clipTitle(line)
	}
	return clipTitle(prompt)
}

func clipTitle(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= 80 {
		return string(r)
	}
	return string(r[:80]) + "…"
}

func noCodeVerdict(phase string, hasCode bool) string {
	if hasCode {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "review":
		return "ревью не принято: в репозитории нет коммита этой задачи. Агент сдал только текст."
	case "test":
		return "тест не прошёл: нечего прогонять, кода в репозитории нет."
	default:
		return ""
	}
}

func notifyText(job store.Job, note, branch, commit string, hasCode bool) string {
	head := jobNotifyLabel(job)
	note = clip(note, 1200)
	phase := strings.ToLower(strings.TrimSpace(job.Phase))
	if phase == "" {
		phase = "doing"
	}
	if phase == "review" || phase == "test" {
		if commit != "" {
			label := "ревью"
			if phase == "test" {
				label = "тест"
			}
			return strings.TrimSpace(note + "\n\nкоммит " + commit + " " + label)
		}
		return note
	}
	if !hasCode {
		if gitFailed(note) {
			return fmt.Sprintf(
				"%s: коммит выполнения не попал в GitHub.\n\n%s\n\nВетки нет — review, тест и пуш на стенд не запускались.\n%s",
				head, note, noCodeMark,
			)
		}
		return fmt.Sprintf(
			"%s: агент сдал план, репозиторий не менялся.\n\n%s\n\nКода нет — review, тест и пуш на стенд не запускались.\n%s",
			head, note, noCodeMark,
		)
	}
	text := fmt.Sprintf("%s: коммит выполнения %s на ветке %s.\n\n%s", head, commit, branch, note)
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
