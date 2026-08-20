package bot

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"leo-bot/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestTrackerNotifyKind(t *testing.T) {
	if got := trackerNotifyKind("⏰ Задача #236 выполнена.\n\nГотово."); got != "done" {
		t.Fatalf("done: %q", got)
	}
	if got := trackerNotifyKind("🔧 Задача #1 началась."); got != "started" {
		t.Fatalf("started: %q", got)
	}
	if got := trackerNotifyKind("Задача отменена админом"); got != "canceled" {
		t.Fatalf("canceled: %q", got)
	}
	if got := trackerNotifyKind("не удалось собрать"); got != "error" {
		t.Fatalf("error: %q", got)
	}
	if got := trackerNotifyKind("⏰ Задача #1 выполнена.\n\nГотово.\n📤 Git push недоступен"); got != "done" {
		t.Fatalf("git push note is still done: %q", got)
	}
	if got := trackerNotifyKind("можно на тест"); got != "done" {
		t.Fatalf("composer review pass: %q", got)
	}
	if got := trackerNotifyKind("ревью не принято: нет тестов"); got != "done" {
		t.Fatalf("composer review fail is still a phase result: %q", got)
	}
}

func TestTrackerNotifyTaskNum(t *testing.T) {
	if got := trackerNotifyTaskNum("⏰ Задача #236 выполнена."); got != 236 {
		t.Fatalf("num: %d", got)
	}
	if got := trackerNotifyTaskNum("🔧 Задача #1 началась."); got != 1 {
		t.Fatalf("num: %d", got)
	}
	if got := trackerNotifyTaskNum("без номера"); got != 0 {
		t.Fatalf("empty: %d", got)
	}
}

func TestApplyTrackerNotifyReviewGoesToTest(t *testing.T) {
	task := database.TrackerTask{Status: "reviewing", DevColumn: trackerColReview}
	applyTrackerNotify(&task, "done", "можно на тест")
	if task.DevColumn != trackerColTest || task.Status != "holding" {
		t.Fatalf("review → test: %+v", task)
	}
}

func TestApplyTrackerNotifyMovesDoingToReview(t *testing.T) {
	task := database.TrackerTask{
		ID:        11,
		Num:       1,
		Status:    "running",
		DevColumn: trackerColDoing,
		Steps:     []string{"Взяли в работу по расписанию"},
	}
	text := "⏰ Задача #1 выполнена.\n\nГотово.\n\n- Подпись теперь только «сгорит через …».\n📤 Git push недоступен"
	applyTrackerNotify(&task, trackerNotifyKind(text), text)
	if task.Status != "reviewing" || task.DevColumn != trackerColReview {
		t.Fatalf("after done: status=%s col=%s", task.Status, task.DevColumn)
	}
	if !strings.Contains(task.Result, "выполнена") {
		t.Fatalf("result not stored: %q", task.Result)
	}
	if task.Error != "" {
		t.Fatalf("git push must not be an error: %q", task.Error)
	}
	if n := len(task.Steps); n == 0 || task.Steps[n-1] != "Агент сдал результат" {
		t.Fatalf("steps: %#v", task.Steps)
	}
}

func TestApplyTrackerNotifyFastTrackGoesToDeploy(t *testing.T) {
	task := database.TrackerTask{Status: "running", DevColumn: trackerColDoing, FastTrack: true}
	applyTrackerNotify(&task, "done", "Готово.")
	if task.DevColumn != trackerColDeploy || task.Status != "holding" {
		t.Fatalf("fast track: %+v", task)
	}
	if !trackerShouldShipAfterNotify(task) {
		t.Fatal("fast track should ship")
	}
}

func TestApplyTrackerNotifyDoesNotPullBackFromTest(t *testing.T) {
	task := database.TrackerTask{Status: "holding", DevColumn: trackerColTest, HandedToQa: true}
	applyTrackerNotify(&task, "done", "Задача #8 выполнена.\n\nтест пройден")
	if task.DevColumn != trackerColDeploy {
		t.Fatalf("composer test pass goes to build: %s", task.DevColumn)
	}
	if trackerShouldShipAfterNotify(task) {
		t.Fatal("ordinary pipeline ships via finishTrackerBuild, not this flag")
	}
	manual := database.TrackerTask{Status: "holding", DevColumn: trackerColTest, HandedToQa: true, ManualQa: true}
	applyTrackerNotify(&manual, "done", "Задача #8 выполнена.")
	if manual.DevColumn != trackerColTest {
		t.Fatalf("manual QA stays on test: %s", manual.DevColumn)
	}
}

func TestApplyTrackerNotifyUnknownTextKeepsDoing(t *testing.T) {
	task := database.TrackerTask{Status: "running", DevColumn: trackerColDoing}
	applyTrackerNotify(&task, "", "агент ещё пишет код")
	if task.DevColumn != trackerColDoing || task.Status != "running" {
		t.Fatalf("progress must stay in work: %+v", task)
	}
	if task.Result != "агент ещё пишет код" {
		t.Fatalf("result: %q", task.Result)
	}
}

func TestApplyTrackerNotifyStartedOnlyFromTodo(t *testing.T) {
	todo := database.TrackerTask{Status: "pending", DevColumn: trackerColTodo}
	applyTrackerNotify(&todo, "started", "🔧 Задача #1 началась.")
	if todo.DevColumn != trackerColDoing || todo.Status != "running" {
		t.Fatalf("todo→doing: %+v", todo)
	}
	doing := database.TrackerTask{Status: "running", DevColumn: trackerColDoing}
	applyTrackerNotify(&doing, "started", "🔧 Задача #1 началась.")
	if doing.DevColumn != trackerColDoing {
		t.Fatalf("already doing: %s", doing.DevColumn)
	}
}

func TestTrackerNotifyDoneColumn(t *testing.T) {
	if got := trackerNotifyDoneColumn(database.TrackerTask{DevColumn: trackerColDoing}); got != trackerColReview {
		t.Fatalf("default: %s", got)
	}
	if got := trackerNotifyDoneColumn(database.TrackerTask{DevColumn: trackerColDoing, AutoReview: true}); got != trackerColReview {
		t.Fatalf("auto review still goes through review: %s", got)
	}
	if got := trackerNotifyDoneColumn(database.TrackerTask{DevColumn: trackerColReview}); got != trackerColTest {
		t.Fatalf("review → test: %s", got)
	}
	if got := trackerNotifyDoneColumn(database.TrackerTask{DevColumn: trackerColTest}); got != trackerColDeploy {
		t.Fatalf("test → build: %s", got)
	}
}

func TestTrackerShouldAdvanceFromResult(t *testing.T) {
	waiting := database.TrackerTask{
		Status:    "pending",
		DevColumn: trackerColTodo,
		Result:    "⏰ Задача #236 выполнена.\n\nГотово.\n- Подпись теперь только «сгорит через …».",
	}
	if !trackerShouldAdvanceFromResult(waiting) {
		t.Fatal("waiting + выполнена must leave Ожидает")
	}
	doing := waiting
	doing.Status = "running"
	doing.DevColumn = trackerColDoing
	if !trackerShouldAdvanceFromResult(doing) {
		t.Fatal("doing + выполнена must leave В работе")
	}
	review := waiting
	review.Status = "reviewing"
	review.DevColumn = trackerColReview
	if trackerShouldAdvanceFromResult(review) {
		t.Fatal("already on review must stay")
	}
	empty := database.TrackerTask{Status: "pending", DevColumn: trackerColTodo}
	if trackerShouldAdvanceFromResult(empty) {
		t.Fatal("no result must not move")
	}
}

func TestApplyBoardNotifyFindsOpenTaskWhenForeignID(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	now := time.Date(2026, 8, 20, 9, 49, 0, 0, time.UTC)
	b := &Bot{db: database.NewForTest(sqlDB)}

	mock.ExpectQuery(`WHERE t.id = \$1`).
		WithArgs(int64(236)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`WHERE t.num = \$1`).
		WithArgs(236).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`t.last_run_at DESC`).
		WillReturnRows(sqlmock.NewRows(trackerListColumns()).AddRow(
			int64(11), 1, "починить статус на доске", now, "20.08 09:19", "разово", "task",
			"running", "doing", nil, nil, false,
			false, false, false, true,
			"", "", []byte(`["Взяли в работу по расписанию"]`), int64(42),
			now, now, now, 0,
		))
	mock.ExpectExec(`UPDATE pack_tracker_tasks SET`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	id, ship, err := b.ApplyBoardNotify(236, "⏰ Задача #236 выполнена.\n\nГотово.")
	if err != nil {
		t.Fatal(err)
	}
	if id != 11 || ship {
		t.Fatalf("id=%d ship=%v", id, ship)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
