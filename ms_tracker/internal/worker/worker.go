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
	job.Error = ""
	job.Result = strings.TrimSpace(res.Note)
	job.Branch = res.Branch
	job.Status = "done"
	store.AppendStep(&job, "Агент сдал результат")
	if res.Pushed {
		store.AppendStep(&job, "ветка "+res.Branch)
	}
	_ = st.Save(job)

	n := job.SourceNum
	if n <= 0 {
		n = int(job.ID)
	}
	text := fmt.Sprintf("✅ Задача #%d выполнена.\n\n%s", n, clip(res.Note, 1200))
	if res.Branch != "" {
		text += "\n\nВетка: " + res.Branch
	}
	text += "\nЧтобы выкатить на сервер, напиши «запушь»."
	if err := notify.JobDone(cfg, job, text); err != nil {
		log.Printf("трекер: не уведомить #%d: %v", job.ID, err)
	}
}

func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
