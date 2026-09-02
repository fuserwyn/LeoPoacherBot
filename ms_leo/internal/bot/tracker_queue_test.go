package bot

import (
	"testing"

	"leo-bot/internal/database"
)

func TestTrackerTaskInPipeline(t *testing.T) {
	cases := []struct {
		name string
		task database.TrackerTask
		want bool
	}{
		{
			name: "todo waits",
			task: database.TrackerTask{Status: "pending", DevColumn: trackerColTodo},
			want: false,
		},
		{
			name: "doing runs",
			task: database.TrackerTask{Status: "running", DevColumn: trackerColDoing},
			want: true,
		},
		{
			name: "review runs",
			task: database.TrackerTask{Status: "reviewing", DevColumn: trackerColReview},
			want: true,
		},
		{
			name: "test runs",
			task: database.TrackerTask{Status: "holding", DevColumn: trackerColTest},
			want: true,
		},
		{
			name: "deploy runs",
			task: database.TrackerTask{Status: "holding", DevColumn: trackerColDeploy},
			want: true,
		},
		{
			name: "done finished",
			task: database.TrackerTask{Status: "done", DevColumn: trackerColDone},
			want: false,
		},
		{
			name: "canceled in doing",
			task: database.TrackerTask{Status: "canceled", DevColumn: trackerColDoing},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := trackerTaskInPipeline(tc.task); got != tc.want {
				t.Fatalf("trackerTaskInPipeline(%+v) = %v, want %v", tc.task, got, tc.want)
			}
		})
	}
}

func TestTrackerPipelineBusy(t *testing.T) {
	b := &Bot{}
	if b.trackerPipelineBusy(1) {
		t.Fatal("nil db must not block")
	}
}
