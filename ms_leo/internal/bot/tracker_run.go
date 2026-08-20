package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"leo-bot/internal/database"
)

// Как часто смотрим, не пора ли снять карточку с «Ожидает». Форма «Сейчас»
// ставит запуск через минуту — тик короче минуты, иначе задача снова висела бы
// после срока, как на скрине: 09:19 поставлена, 09:20 ещё в очереди.
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
	for _, t := range due {
		if b.logger != nil {
			b.logger.Infof("трекер: задача #%d началась по расписанию", trackerDueNum(t))
		}
		author := int64(0)
		if t.HasAuthor {
			author = t.AuthorID
		}
		if err := b.NotifyTrackerResult(author, trackerDueStartedNote(t)); err != nil && b.logger != nil {
			b.logger.Warnf("трекер: не сообщить о старте #%d: %v", trackerDueNum(t), err)
		}
	}
	return len(due), nil
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
