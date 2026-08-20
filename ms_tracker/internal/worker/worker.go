package worker

import (
	"fmt"
	"log"
	"strings"
	"time"

	"leo-tracker/internal/agent"
	"leo-tracker/internal/config"
	"leo-tracker/internal/notify"
	"leo-tracker/internal/store"
)

func Loop(cfg config.Config, st *store.Store, stop <-chan struct{}) {
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	runOnce(cfg, st)
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			runOnce(cfg, st)
		}
	}
}

func runOnce(cfg config.Config, st *store.Store) {
	due, err := st.ClaimDue(time.Now(), 3)
	if err != nil {
		log.Printf("трекер: не забрать задачи: %v", err)
		return
	}
	for _, job := range due {
		go finish(cfg, st, job)
	}
}

func finish(cfg config.Config, st *store.Store, job store.Job) {
	store.AppendStep(&job, "Агент: запустили")
	if job.Branch == "" && job.SourceTaskID > 0 {
		job.Branch = st.SourceBranch(job.SourceTaskID)
	}
	_ = st.Save(job)

	res, err := agent.Run(cfg, job)
	if err != nil {
		job.Status = "error"
		job.Error = "Агент не стартовал: " + err.Error()
		store.AppendStep(&job, "Агент не стартовал")
		_ = st.Save(job)
		_ = notify.JobDone(cfg, job, fmt.Sprintf("⚠️ Задача #%d: агент не стартовал.\n%s", job.SourceNum, job.Error))
		return
	}
	hasCode := res.Committed || st.SourceHasBranch(job.SourceTaskID)
	note := strings.TrimSpace(res.Note)
	if verdict := noCodeVerdict(job.Phase, hasCode); verdict != "" {
		note = verdict
	}
	job.Error = ""
	job.Result = note
	if res.Committed {
		job.Branch = res.Branch
	}
	job.Status = "done"
	store.AppendStep(&job, "Агент сдал результат")
	if res.Committed {
		label := "выполнение"
		switch strings.ToLower(strings.TrimSpace(job.Phase)) {
		case "review":
			label = "ревью"
		case "test":
			label = "тест"
		}
		store.AppendStep(&job, "коммит "+res.Commit+" "+label)
		store.AppendStep(&job, "ветка "+res.Branch)
	}
	if !hasCode {
		store.AppendStep(&job, "кода в репозитории нет")
	}
	_ = st.Save(job)

	if err := notify.JobDone(cfg, job, notifyText(job, note, res.Branch, res.Commit, hasCode)); err != nil {
		log.Printf("трекер: не уведомить #%d: %v", job.ID, err)
	}
}
