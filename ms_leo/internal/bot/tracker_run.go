package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/database"
)

// Как часто смотрим, не пора ли снять карточку с «Ожидает».
const trackerDueTick = 15 * time.Second

// trackerTaskDueForStart — та же развилка, что в ClaimDueTrackerTasks:
// только очередь и только если when_at уже наступил.
func trackerTaskDueForStart(t database.TrackerTask, now time.Time) bool {
	status := strings.ToLower(strings.TrimSpace(t.Status))
	if status != "pending" && status != "scheduled" {
		return false
	}
	col := strings.ToLower(strings.TrimSpace(t.DevColumn))
	if col == "" {
		col = trackerColTodo
	}
	if col != trackerColTodo {
		return false
	}
	if t.WhenAt.IsZero() {
		return true
	}
	return !t.WhenAt.After(now)
}

func trackerDueStartedNote(t database.TrackerTask) string {
	n := t.Num
	if n <= 0 {
		n = int(t.ID)
	}
	prompt := strings.TrimSpace(t.Prompt)
	if runes := []rune(prompt); len(runes) > 180 {
		prompt = string(runes[:180]) + "…"
	}
	text := fmt.Sprintf("🔧 Задача #%d началась.", n)
	if prompt != "" {
		text += "\n" + prompt
	}
	return text
}

// startTrackerDueScheduler — фоновый цикл: созревшие карточки сами уходят
// из «Ожидает» в «В работе». Первый прогон сразу после старта, чтобы уже
// просроченные задачи (как #1 с 09:19) не ждали ещё один тик.
func (b *Bot) startTrackerDueScheduler(ctx context.Context) {
	b.runDueTrackerTasks()
	ticker := time.NewTicker(trackerDueTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.runDueTrackerTasks()
		}
	}
}

func (b *Bot) runDueTrackerTasks() {
	_, _ = b.claimAndNotifyDueTrackerTasks()
}

// claimAndNotifyDueTrackerTasks — то, что делает «Обновить» на доске:
// снимает созревшие с «Ожидает» и пишет автору. Ошибка наружу — кнопка
// не должна притворяться, что доска обновилась, если забрать не вышло.
func (b *Bot) claimAndNotifyDueTrackerTasks() (int, error) {
	if b == nil || b.db == nil {
		return 0, fmt.Errorf("база недоступна")
	}
	due, err := b.db.ClaimDueTrackerTasks(time.Now())
	if err != nil {
		if b.logger != nil {
			b.logger.Warnf("трекер: не забрать созревшие задачи: %v", err)
		}
		return 0, err
	}
	healed, herr := b.healTrackerCardsFromStoredResult()
	if herr != nil && b.logger != nil {
		b.logger.Warnf("трекер: не додвинуть карточки по сохранённому результату: %v", herr)
	}
	for _, t := range due {
		if b.logger != nil {
			b.logger.Infof("трекер: задача #%d началась по расписанию", trackerDueNum(t))
		}
		// В личку не пишем: старт — промежуточный статус.
		b.dispatchTrackerAgent(t, "doing")
	}
	kicked := b.kickStuckTrackerAgents(false)
	return len(due) + healed + kicked, nil
}

func (b *Bot) claimAndKickTrackerTasks(forceStuck bool) (int, error) {
	started, err := b.claimAndNotifyDueTrackerTasks()
	if err != nil {
		return started, err
	}
	if !forceStuck {
		return started, nil
	}
	// claimAndNotify already kicked with force=false. Обновить — ещё раз
	// принудительно, чтобы HTTP 405 и прочие фейлы не висели до кулдауна.
	extra := b.kickStuckTrackerAgents(true)
	return started + extra, nil
}

func (b *Bot) kickStuckTrackerAgents(force bool) int {
	if b == nil || b.db == nil {
		return 0
	}
	list, err := b.db.ListTrackerTasks()
	if err != nil {
		if b.logger != nil {
			b.logger.Warnf("трекер: не прочитать зависшие задачи: %v", err)
		}
		return 0
	}
	now := time.Now()
	n := 0
	for _, t := range list {
		if !trackerNeedsAgentKick(t, now, force) {
			continue
		}
		t.Error = ""
		appendTrackerStep(&t, "Снова запускаем агента")
		if err := b.db.SaveTrackerTask(t); err != nil {
			if b.logger != nil {
				b.logger.Warnf("трекер: не перезапустить #%d: %v", trackerDueNum(t), err)
			}
			continue
		}
		if b.logger != nil {
			b.logger.Infof("трекер: снова запускаем агента #%d", trackerDueNum(t))
		}
		b.dispatchTrackerAgent(t, "doing")
		n++
	}
	return n
}

func trackerDueNum(t database.TrackerTask) int {
	if t.Num > 0 {
		return t.Num
	}
	return int(t.ID)
}

// kickTrackerDueIfReady — после постановки/переноса: если срок уже сейчас,
// не ждём тик (иначе «дата в прошлом» висела бы до 15 секунд).
func (b *Bot) kickTrackerDueIfReady(at time.Time) {
	if b == nil || at.After(time.Now()) {
		return
	}
	go b.runDueTrackerTasks()
}
