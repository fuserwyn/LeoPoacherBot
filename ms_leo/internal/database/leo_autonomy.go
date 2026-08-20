package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Автономный режим Лео: он сам придумывает задачи трекера и ставит их, пока
// админ не выключит или пока не выйдет срок. Состояние — одна строка на
// окружение (миграция 76): «включён до», как часто и по сколько задач.

// LeoAutonomy — состояние режима. Нулевое значение = выключен.
type LeoAutonomy struct {
	ActiveUntil time.Time
	EveryHours  int
	TasksPerRun int
	LastRunAt   time.Time
	LastNote    string
	UpdatedBy   int64
	UpdatedAt   time.Time
}

// Active — работает ли режим прямо сейчас.
func (s LeoAutonomy) Active() bool {
	return !s.ActiveUntil.IsZero() && s.ActiveUntil.After(time.Now())
}

// DueAt — когда Лео возьмётся за дело в следующий раз. Нулевое время, если режим выключен.
func (s LeoAutonomy) DueAt() time.Time {
	if !s.Active() {
		return time.Time{}
	}
	if s.LastRunAt.IsZero() {
		return time.Now()
	}
	every := s.EveryHours
	if every <= 0 {
		every = 4
	}
	return s.LastRunAt.Add(time.Duration(every) * time.Hour)
}

// GetLeoAutonomy — текущее состояние. Нет строки — режим выключен.
func (d *Database) GetLeoAutonomy() (LeoAutonomy, error) {
	var s LeoAutonomy
	if d == nil || d.trackerDB() == nil {
		return s, fmt.Errorf("база недоступна")
	}
	var until, lastRun sql.NullTime
	var note sql.NullString
	var by sql.NullInt64
	err := d.trackerDB().QueryRow(`
		SELECT active_until, every_hours, tasks_per_run, last_run_at, last_note,
		       updated_by, updated_at
		FROM leo_autonomy WHERE id = TRUE
	`).Scan(&until, &s.EveryHours, &s.TasksPerRun, &lastRun, &note, &by, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return LeoAutonomy{EveryHours: 4, TasksPerRun: 3}, nil
	}
	if err != nil {
		return s, err
	}
	if until.Valid {
		s.ActiveUntil = until.Time
	}
	if lastRun.Valid {
		s.LastRunAt = lastRun.Time
	}
	s.LastNote = note.String
	s.UpdatedBy = by.Int64
	return s, nil
}

// SetLeoAutonomy — включить до указанного момента (нулевое время — выключить).
func (d *Database) SetLeoAutonomy(until time.Time, everyHours, tasksPerRun int, by int64) error {
	if d == nil || d.trackerDB() == nil {
		return fmt.Errorf("база недоступна")
	}
	if everyHours <= 0 {
		everyHours = 4
	}
	if tasksPerRun <= 0 {
		tasksPerRun = 3
	}
	var until_ any
	if !until.IsZero() {
		until_ = until
	}
	_, err := d.trackerDB().Exec(`
		INSERT INTO leo_autonomy (id, active_until, every_hours, tasks_per_run, updated_by, updated_at)
		VALUES (TRUE, $1, $2, $3, $4, NOW())
		ON CONFLICT (id) DO UPDATE SET
			active_until = EXCLUDED.active_until,
			every_hours = EXCLUDED.every_hours,
			tasks_per_run = EXCLUDED.tasks_per_run,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
	`, until_, everyHours, tasksPerRun, by)
	return err
}

// MarkLeoAutonomyRun — отметить прогон: время и что именно он придумал.
func (d *Database) MarkLeoAutonomyRun(note string) error {
	if d == nil || d.trackerDB() == nil {
		return fmt.Errorf("база недоступна")
	}
	note = strings.TrimSpace(note)
	if len([]rune(note)) > 1000 {
		note = string([]rune(note)[:1000])
	}
	_, err := d.trackerDB().Exec(`
		INSERT INTO leo_autonomy (id, last_run_at, last_note, updated_at)
		VALUES (TRUE, NOW(), $1, NOW())
		ON CONFLICT (id) DO UPDATE SET last_run_at = NOW(), last_note = $1, updated_at = NOW()
	`, note)
	return err
}
