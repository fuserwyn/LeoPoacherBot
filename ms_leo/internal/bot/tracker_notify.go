package bot

import (
	"fmt"
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
	from := strings.ToLower(strings.TrimSpace(t.DevColumn))
	applyTrackerNotify(&t, kind, text)
	if kind == "done" && (from == trackerColReview || from == trackerColTest) &&
		!trackerComposerPassed(from, text) {
		// Composer вернул провал: не двигаем дальше, возвращаем в работу.
		t.Error = clipNotifyText(text)
		_ = applyTrackerColumn(&t, trackerColDoing)
		appendTrackerStep(&t, trackerAgentName(from)+" не принято")
	}
	if err := b.db.SaveTrackerTask(t); err != nil {
		return t.ID, false, err
	}
	if kind == "done" {
		b.kickTrackerPipeline(t)
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
		if t, ok := trackerTaskIfFound(b.findTrackerTaskByRemoteID(id)); ok {
			return t, nil
		}
	}
	if num > 0 && int64(num) != id {
		if t, ok := trackerTaskIfFound(b.db.GetTrackerTaskByNum(num)); ok {
			return t, nil
		}
		if t, ok := trackerTaskIfFound(b.findTrackerTaskByRemoteID(int64(num))); ok {
			return t, nil
		}
	}
	return b.db.FindOpenTrackerTask()
}

func (b *Bot) findTrackerTaskByRemoteID(remoteID int64) (database.TrackerTask, error) {
	var empty database.TrackerTask
	if b == nil || b.db == nil || remoteID <= 0 {
		return empty, fmt.Errorf("задача не найдена")
	}
	list, err := b.db.ListTrackerTasks()
	if err != nil {
		return empty, err
	}
	for _, t := range list {
		if trackerStepRemoteID(t.Steps) == remoteID {
			return t, nil
		}
	}
	return empty, fmt.Errorf("задача не найдена")
}

func trackerTaskIfFound(t database.TrackerTask, err error) (database.TrackerTask, bool) {
	if err != nil || t.ID <= 0 {
		return database.TrackerTask{}, false
	}
	return t, true
}

// TrackerNotifyIsFullyShipped — в личку только финал: задача на Railway в main.
func TrackerNotifyIsFullyShipped(text string) bool {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return false
	}
	if strings.Contains(low, "запушь") || strings.Contains(text, "TRACKER_NO_CODE") ||
		strings.Contains(low, "началась") || strings.Contains(low, "можно на тест") ||
		strings.Contains(low, "ревью не принято") || strings.Contains(low, "тест не прошёл") ||
		strings.Contains(low, "тест не прошел") || strings.Contains(low, "кода нет") ||
		strings.Contains(low, "не попал в github") {
		return false
	}
	hasRailway := strings.Contains(low, "railway")
	hasMain := strings.Contains(low, "ветке main") || strings.Contains(low, "ветки main") ||
		strings.Contains(low, "ветка main") || strings.Contains(low, "в main") ||
		strings.Contains(low, "(main)") || strings.Contains(low, "railway main")
	hasDeployed := strings.Contains(low, "задепло") || strings.Contains(low, "выехал") ||
		strings.Contains(low, "на railway") && (strings.Contains(low, "выполнен") || strings.Contains(low, "готово"))
	return hasRailway && hasMain && hasDeployed
}

func trackerFullyDoneNote(t database.TrackerTask) string {
	return fmt.Sprintf("✅ Задача #%d выполнена.\nВыехала на Railway (ветка main).", trackerDueNum(t))
}

const trackerShipNotifiedStep = "уведомили о выкате"

func trackerAlreadyNotifiedShip(t database.TrackerTask) bool {
	for _, step := range t.Steps {
		if strings.TrimSpace(step) == trackerShipNotifiedStep {
			return true
		}
	}
	return false
}

func trackerNotifyAuthor(t database.TrackerTask) int64 {
	if t.HasAuthor {
		return t.AuthorID
	}
	return 0
}

func (b *Bot) notifyTrackerShippedOnce(t database.TrackerTask) {
	if b == nil || t.ID <= 0 {
		return
	}
	if b.db != nil {
		if fresh, err := b.db.GetTrackerTask(t.ID); err == nil && fresh.ID > 0 {
			t = fresh
		}
	}
	if trackerAlreadyNotifiedShip(t) {
		return
	}
	note := trackerFullyDoneNote(t)
	appendTrackerStep(&t, trackerShipNotifiedStep)
	if !strings.Contains(t.Result, note) {
		t.Result = strings.TrimSpace(strings.TrimSpace(t.Result) + "\n\n" + note)
	}
	if b.db != nil {
		if err := b.db.SaveTrackerTask(t); err != nil && b.logger != nil {
			b.logger.Warnf("трекер: не сохранить уведомление о выкате #%d: %v", trackerDueNum(t), err)
			return
		}
	}
	if err := b.NotifyTrackerResult(trackerNotifyAuthor(t), note); err != nil && b.logger != nil {
		b.logger.Warnf("трекер: не сообщить о выкате #%d: %v", trackerDueNum(t), err)
	}
}

func (b *Bot) NotifyTrackerShippedIfNeeded(taskID int64, text string) {
	if b == nil || !TrackerNotifyIsFullyShipped(text) {
		return
	}
	t, err := b.findTrackerTaskForNotify(taskID, trackerNotifyTaskNum(text))
	if err != nil || t.ID <= 0 {
		return
	}
	b.notifyTrackerShippedOnce(t)
}

func trackerNotifyKind(text string) string {
	low := strings.ToLower(text)
	switch {
	case strings.Contains(low, "отменен") || strings.Contains(low, "cancelled") || strings.Contains(low, "canceled"):
		return "canceled"
	case strings.Contains(low, "выполнен") || strings.Contains(low, "completed") ||
		strings.Contains(low, "готово") || strings.Contains(text, "✅") ||
		strings.Contains(low, "можно на тест") || strings.Contains(low, "тест пройден") ||
		strings.Contains(low, "ревью не принято") || strings.Contains(low, "тест не прошёл") ||
		strings.Contains(low, "тест не прошел"):
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
	// Сдача на review/тесте — это вердикт Composer, двигаем дальше.
	// Ручное QA на тесте не перескакиваем: человек ещё смотрит.
	if t.ManualQa && col == trackerColTest {
		return trackerColTest
	}
	switch col {
	case trackerColReview:
		return trackerColTest
	case trackerColTest:
		return trackerColDeploy
	case trackerColDeploy, trackerColDone:
		return col
	}
	if t.FastTrack {
		return trackerColDeploy
	}
	return trackerColReview
}

func trackerShouldShipAfterNotify(t database.TrackerTask) bool {
	// Обычный путь: kickTrackerPipeline сам закрывает сборку.
	// Fast-track сразу к публикации — пусть старый ship тоже сработает.
	if !t.FastTrack {
		return false
	}
	col := strings.ToLower(strings.TrimSpace(t.DevColumn))
	return col == trackerColDeploy || col == trackerColDone
}

// healTrackerCardsFromStoredResult — «выполнена» уже лежит в result, а колонка
// всё ещё «Ожидает» / «В работе». Так бывает, если уведомление пришло, пока
// карточка ждала срок, или claim из-за TZ её не снял. Двигаем по сохранённому
// тексту, не дожидаясь второго вебхука.
func (b *Bot) healTrackerCardsFromStoredResult() (int, error) {
	if b == nil || b.db == nil {
		return 0, nil
	}
	list, err := b.db.ListTrackerTasks()
	if err != nil {
		return 0, err
	}
	moved := 0
	for _, t := range list {
		if !trackerShouldAdvanceFromResult(t) {
			continue
		}
		applyTrackerNotify(&t, trackerNotifyKind(t.Result), t.Result)
		if err := b.db.SaveTrackerTask(t); err != nil {
			return moved, err
		}
		if trackerNotifyKind(t.Result) == "done" {
			b.kickTrackerPipeline(t)
		}
		moved++
	}
	return moved, nil
}

// trackerShouldAdvanceFromResult — в result уже «готово», а колонка не там.
func trackerShouldAdvanceFromResult(t database.TrackerTask) bool {
	kind := trackerNotifyKind(t.Result)
	if kind == "" || kind == "started" {
		return false
	}
	col := strings.ToLower(strings.TrimSpace(t.DevColumn))
	status := strings.ToLower(strings.TrimSpace(t.Status))
	if status == "done" || status == "canceled" || status == "cancelled" ||
		col == trackerColDone || col == trackerColCanceled {
		return false
	}
	switch kind {
	case "done":
		want := trackerNotifyDoneColumn(t)
		if col == want && status != "pending" && status != "scheduled" && status != "running" {
			return false
		}
		return col == "" || col == trackerColTodo || col == trackerColDoing ||
			status == "pending" || status == "scheduled" || status == "running"
	case "error":
		return status != "error"
	case "canceled":
		return col != trackerColCanceled
	default:
		return false
	}
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
