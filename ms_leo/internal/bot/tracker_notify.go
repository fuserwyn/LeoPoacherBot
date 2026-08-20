package bot

import (
	"regexp"
	"strconv"
	"strings"

	"leo-bot/internal/database"
)

var trackerNotifyNumRe = regexp.MustCompile(`(?i)задач[аиеу]?\s*#\s*(\d+)`)

const trackerNotifyResultMax = 4000

// ApplyBoardNotify — входящее «задача выполнена» должно сдвинуть карточку
// на доске. Раньше писали только в личку, а колонка так и оставалась «В работе».
func (b *Bot) ApplyBoardNotify(taskID int64, text string) (localID int64, ship bool, err error) {
	text = strings.TrimSpace(text)
	if b == nil || b.db == nil || text == "" {
		return 0, false, nil
	}
	kind := trackerNotifyKind(text)
	t, err := b.findTrackerTaskForNotify(taskID, trackerNotifyTaskNum(text))
	if err != nil {
		return 0, false, err
	}
	if t.ID <= 0 {
		return 0, false, nil
	}
	applyTrackerNotify(&t, kind, text)
	if err := b.db.SaveTrackerTask(t); err != nil {
		return t.ID, false, err
	}
	return t.ID, trackerShouldShipAfterNotify(t), nil
}

func (b *Bot) findTrackerTaskForNotify(id int64, num int) (database.TrackerTask, error) {
	var empty database.TrackerTask
	if b == nil || b.db == nil {
		return empty, nil
	}
	if id > 0 {
		if t, ok := trackerTaskIfFound(b.db.GetTrackerTask(id)); ok {
			return t, nil
		}
		if t, ok := trackerTaskIfFound(b.db.GetTrackerTaskByNum(int(id))); ok {
			return t, nil
		}
	}
	if num > 0 && int64(num) != id {
		if t, ok := trackerTaskIfFound(b.db.GetTrackerTaskByNum(num)); ok {
			return t, nil
		}
	}
	return b.db.FindOpenTrackerTask()
}

func trackerTaskIfFound(t database.TrackerTask, err error) (database.TrackerTask, bool) {
	if err != nil || t.ID <= 0 {
		return database.TrackerTask{}, false
	}
	return t, true
}

func trackerNotifyKind(text string) string {
	low := strings.ToLower(text)
	switch {
	case strings.Contains(low, "отменен") || strings.Contains(low, "cancelled") || strings.Contains(low, "canceled"):
		return "canceled"
	case strings.Contains(low, "выполнен") || strings.Contains(low, "completed") ||
		strings.Contains(low, "готово") || strings.Contains(text, "✅"):
		return "done"
	case strings.Contains(low, "ошибк") || strings.Contains(low, "не удалось") || strings.Contains(low, "срыв"):
		return "error"
	case strings.Contains(low, "началась") || strings.Contains(low, "взяли в работу"):
		return "started"
	default:
		return ""
	}
}

func trackerNotifyTaskNum(text string) int {
	m := trackerNotifyNumRe.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func clipNotifyText(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > trackerNotifyResultMax {
		return string(r[:trackerNotifyResultMax])
	}
	return s
}

func trackerNotifyDoneColumn(t database.TrackerTask) string {
	col := strings.ToLower(strings.TrimSpace(t.DevColumn))
	switch col {
	case trackerColReview, trackerColTest, trackerColDeploy, trackerColDone:
		return col
	}
	if t.FastTrack {
		return trackerColDeploy
	}
	if t.AutoReview {
		return trackerColTest
	}
	return trackerColReview
}

func trackerShouldShipAfterNotify(t database.TrackerTask) bool {
	if t.FastTrack {
		return true
	}
	col := strings.ToLower(strings.TrimSpace(t.DevColumn))
	return col == trackerColDeploy || col == trackerColDone
}

func applyTrackerNotify(t *database.TrackerTask, kind, text string) {
	if t == nil {
		return
	}
	text = clipNotifyText(text)
	if text != "" {
		t.Result = text
	}
	switch kind {
	case "canceled":
		_ = applyTrackerColumn(t, trackerColCanceled)
		appendTrackerStep(t, "Отменена по уведомлению")
	case "done":
		col := trackerNotifyDoneColumn(*t)
		if col != strings.ToLower(strings.TrimSpace(t.DevColumn)) {
			_ = applyTrackerColumn(t, col)
		}
		t.Error = ""
		appendTrackerStep(t, "Агент сдал результат")
	case "error":
		t.Status = "error"
		if text != "" {
			t.Error = text
		}
		appendTrackerStep(t, "Ошибка")
	case "started":
		status := strings.ToLower(strings.TrimSpace(t.Status))
		if t.DevColumn == trackerColTodo || status == "pending" || status == "scheduled" {
			_ = applyTrackerColumn(t, trackerColDoing)
			appendTrackerStep(t, "Взяли в работу по уведомлению")
		}
	default:
		if text != "" {
			appendTrackerStep(t, "Обновили результат")
		}
	}
}
