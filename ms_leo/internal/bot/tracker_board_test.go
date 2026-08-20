package bot

import (
	"testing"

	"leo-bot/internal/database"
)

func TestApplyTrackerColumnFlow(t *testing.T) {
	var task database.TrackerTask
	if err := applyTrackerColumn(&task, "todo"); err != nil {
		t.Fatal(err)
	}
	if task.Status != "pending" || task.DevColumn != "todo" || task.HandedToQa {
		t.Fatalf("todo: %+v", task)
	}
	if err := applyTrackerColumn(&task, "doing"); err != nil {
		t.Fatal(err)
	}
	if task.Status != "running" || !task.HasLastRun {
		t.Fatalf("doing: %+v", task)
	}
	if err := applyTrackerColumn(&task, "test"); err != nil {
		t.Fatal(err)
	}
	if !task.HandedToQa || task.QaColumn != "todo" || task.DevColumn != "test" {
		t.Fatalf("test: %+v", task)
	}
	if err := applyTrackerColumn(&task, "done"); err != nil {
		t.Fatal(err)
	}
	if task.Status != "done" || task.DevColumn != "done" {
		t.Fatalf("done: %+v", task)
	}
	if err := applyTrackerColumn(&task, "nope"); err == nil {
		t.Fatal("unknown column must fail")
	}
}

func TestTrackerStatusMeta(t *testing.T) {
	label, icon, phase := trackerStatusMeta("pending", "todo")
	if label != "Ожидает" || icon != "⏳" || phase != "todo" {
		t.Fatalf("pending: %s %s %s", label, icon, phase)
	}
	label, _, phase = trackerStatusMeta("holding", "test")
	if label != "Тест" || phase != "test" {
		t.Fatalf("test holding: %s %s", label, phase)
	}
	label, _, phase = trackerStatusMeta("holding", "deploy")
	if label != "Сборка" || phase != "deploy" {
		t.Fatalf("deploy: %s %s", label, phase)
	}
}

func TestTrackerQaMeta(t *testing.T) {
	if label, _ := trackerQaMeta("", "todo", false); label != "" {
		t.Fatalf("not handed: %q", label)
	}
	if label, _ := trackerQaMeta("pass", "done", true); label != "Принято" {
		t.Fatalf("pass: %q", label)
	}
	if label, _ := trackerQaMeta("", "doing", true); label != "В тестировании" {
		t.Fatalf("doing: %q", label)
	}
}

func TestTrackerTaskViewAuthorAndShipShape(t *testing.T) {
	trow := database.TrackerTask{
		ID:         7,
		Num:        3,
		Prompt:     "починить стрик",
		Status:     "holding",
		DevColumn:  "test",
		HandedToQa: true,
		QaColumn:   "todo",
		HasAuthor:  true,
		AuthorID:   database.TrackerLeoAuthorID,
		Steps:      []string{"Поставили", "На тест"},
	}
	view := trackerTaskView(trow, false)
	if view["num"] != 3 || view["author_id"] != database.TrackerLeoAuthorID {
		t.Fatalf("view: %#v", view)
	}
	if view["dev_column"] != "test" || view["handed_to_qa"] != true {
		t.Fatalf("qa flags: %#v", view)
	}
	if view["live_step"] != "На тест" {
		t.Fatalf("live: %#v", view["live_step"])
	}
	if _, ok := view["attachments"]; ok {
		t.Fatal("list view must not embed attachments")
	}
}

func TestTrackerNextColumnMap(t *testing.T) {
	if trackerNextColumn["todo"] != "doing" || trackerNextColumn["deploy"] != "done" {
		t.Fatalf("%#v", trackerNextColumn)
	}
	if _, ok := trackerNextColumn["done"]; ok {
		t.Fatal("done has no next")
	}
}

func TestPayloadHelpers(t *testing.T) {
	p := map[string]any{"prompt": "  hello ", "leo": true, "sprint": float64(2), "on": "true"}
	if payloadString(p, "prompt") != "hello" {
		t.Fatal(payloadString(p, "prompt"))
	}
	if !payloadBool(p, "leo") || !payloadBool(p, "on") {
		t.Fatal("bool")
	}
	if payloadInt(p, "sprint", 1) != 2 {
		t.Fatal(payloadInt(p, "sprint", 1))
	}
	if payloadInt(nil, "sprint", 4) != 4 {
		t.Fatal("fallback")
	}
}
