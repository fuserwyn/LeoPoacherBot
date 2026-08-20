package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
)

const trackerBoardSchema = `
CREATE TABLE IF NOT EXISTS pack_tracker_tasks (
	id            BIGSERIAL PRIMARY KEY,
	num           INT NOT NULL,
	prompt        TEXT NOT NULL,
	when_at       TIMESTAMP WITH TIME ZONE NOT NULL,
	when_label    TEXT NOT NULL DEFAULT '',
	repeat        TEXT NOT NULL DEFAULT 'разово',
	kind          TEXT NOT NULL DEFAULT 'task',
	status        TEXT NOT NULL DEFAULT 'pending',
	dev_column    TEXT NOT NULL DEFAULT 'todo',
	qa_column     TEXT,
	qa_status     TEXT,
	handed_to_qa  BOOLEAN NOT NULL DEFAULT FALSE,
	auto_review   BOOLEAN NOT NULL DEFAULT FALSE,
	manual_qa     BOOLEAN NOT NULL DEFAULT FALSE,
	fast_track    BOOLEAN NOT NULL DEFAULT FALSE,
	auto_push     BOOLEAN NOT NULL DEFAULT FALSE,
	error         TEXT NOT NULL DEFAULT '',
	result        TEXT NOT NULL DEFAULT '',
	steps         JSONB NOT NULL DEFAULT '[]'::jsonb,
	author_id     BIGINT,
	created_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
	last_run_at   TIMESTAMP WITH TIME ZONE,
	updated_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS pack_tracker_tasks_num_uidx
	ON pack_tracker_tasks (num);
CREATE INDEX IF NOT EXISTS pack_tracker_tasks_board_idx
	ON pack_tracker_tasks (dev_column, created_at DESC);

CREATE TABLE IF NOT EXISTS pack_tracker_attachments (
	id         TEXT PRIMARY KEY,
	task_id    BIGINT NOT NULL REFERENCES pack_tracker_tasks(id) ON DELETE CASCADE,
	name       TEXT NOT NULL,
	mime       TEXT NOT NULL,
	size       INT NOT NULL,
	data       BYTEA NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS pack_tracker_attachments_task_idx
	ON pack_tracker_attachments (task_id, created_at);

CREATE TABLE IF NOT EXISTS leo_autonomy (
	id            BOOLEAN PRIMARY KEY DEFAULT TRUE,
	active_until  TIMESTAMP WITH TIME ZONE,
	every_hours   INT NOT NULL DEFAULT 4,
	tasks_per_run INT NOT NULL DEFAULT 3,
	last_run_at   TIMESTAMP WITH TIME ZONE,
	last_note     TEXT,
	updated_by    BIGINT,
	updated_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ms_tracker_jobs (
	id              BIGSERIAL PRIMARY KEY,
	source_task_id  BIGINT NOT NULL DEFAULT 0,
	source_num      INT NOT NULL DEFAULT 0,
	author_id       BIGINT NOT NULL DEFAULT 0,
	prompt          TEXT NOT NULL,
	phase           TEXT NOT NULL DEFAULT 'doing',
	when_at         TIMESTAMPTZ NOT NULL,
	when_label      TEXT NOT NULL DEFAULT '',
	status          TEXT NOT NULL DEFAULT 'pending',
	error           TEXT NOT NULL DEFAULT '',
	result          TEXT NOT NULL DEFAULT '',
	steps           JSONB NOT NULL DEFAULT '[]'::jsonb,
	model           TEXT NOT NULL DEFAULT '',
	auto_push       BOOLEAN NOT NULL DEFAULT FALSE,
	branch          TEXT NOT NULL DEFAULT '',
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS ms_tracker_jobs_due_idx
	ON ms_tracker_jobs (status, when_at);
`

// trackerDB — пул доски. Если второй базы нет, тесты и локалка пишут в основную.
func (d *Database) trackerDB() *sql.DB {
	if d == nil {
		return nil
	}
	if d.tracker != nil {
		return d.tracker
	}
	return d.db
}

// AttachTrackerDatabase — подключить БД только трекера, перенести туда карточки
// и автономию с базы стаи, потом снести таблицы с Лео.
func (d *Database) AttachTrackerDatabase(databaseURL string) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("база недоступна")
	}
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil
	}
	if samePostgresDSN(d.dsn, databaseURL) {
		if d.logger != nil {
			d.logger.Warn("TRACKER_DATABASE_URL совпадает с DATABASE_URL — доска всё ещё на базе Лео")
		}
		return nil
	}
	u := ensureSessionTimeZone(databaseURL)
	tdb, err := sql.Open("postgres", u)
	if err != nil {
		return fmt.Errorf("tracker db: %w", err)
	}
	tdb.SetMaxOpenConns(8)
	tdb.SetMaxIdleConns(2)
	if err := tdb.Ping(); err != nil {
		_ = tdb.Close()
		return fmt.Errorf("tracker db ping: %w", err)
	}
	if _, err := tdb.Exec(trackerBoardSchema); err != nil {
		_ = tdb.Close()
		return fmt.Errorf("tracker schema: %w", err)
	}
	if err := copyTrackerTables(d.db, tdb); err != nil {
		_ = tdb.Close()
		return err
	}
	if err := dropTrackerTablesFromLeo(d.db); err != nil {
		_ = tdb.Close()
		return err
	}
	d.tracker = tdb
	if d.logger != nil {
		d.logger.Info("трекер пишет в отдельную БД, не в базу стаи")
	}
	return nil
}

func samePostgresDSN(a, b string) bool {
	ha, na := postgresHostDB(a)
	hb, nb := postgresHostDB(b)
	return ha != "" && ha == hb && na != "" && na == nb
}

func postgresHostDB(raw string) (host, name string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", ""
	}
	name = strings.TrimPrefix(u.Path, "/")
	if i := strings.IndexByte(name, '?'); i >= 0 {
		name = name[:i]
	}
	return strings.ToLower(u.Host), strings.ToLower(name)
}

func copyTrackerTables(src, dst *sql.DB) error {
	if err := copyIfDestEmpty(src, dst, "pack_tracker_tasks", `
		INSERT INTO pack_tracker_tasks (
			id, num, prompt, when_at, when_label, repeat, kind, status, dev_column,
			qa_column, qa_status, handed_to_qa, auto_review, manual_qa, fast_track,
			auto_push, error, result, steps, author_id, created_at, last_run_at, updated_at
		) SELECT
			id, num, prompt, when_at, when_label, repeat, kind, status, dev_column,
			qa_column, qa_status, handed_to_qa, auto_review, manual_qa, fast_track,
			auto_push, error, result, steps, author_id, created_at, last_run_at, updated_at
		FROM pack_tracker_tasks
	`); err != nil {
		return err
	}
	if err := copyIfDestEmpty(src, dst, "pack_tracker_attachments", `
		INSERT INTO pack_tracker_attachments (id, task_id, name, mime, size, data, created_at)
		SELECT id, task_id, name, mime, size, data, created_at FROM pack_tracker_attachments
	`); err != nil {
		return err
	}
	if err := copyIfDestEmpty(src, dst, "leo_autonomy", `
		INSERT INTO leo_autonomy (id, active_until, every_hours, tasks_per_run, last_run_at, last_note, updated_by, updated_at)
		SELECT id, active_until, every_hours, tasks_per_run, last_run_at, last_note, updated_by, updated_at
		FROM leo_autonomy
		ON CONFLICT (id) DO NOTHING
	`); err != nil {
		return err
	}
	if err := copyIfDestEmpty(src, dst, "ms_tracker_jobs", `
		INSERT INTO ms_tracker_jobs (
			id, source_task_id, source_num, author_id, prompt, phase, when_at, when_label,
			status, error, result, steps, model, auto_push, branch, created_at, updated_at
		) SELECT
			id, source_task_id, source_num, author_id, prompt, phase, when_at, when_label,
			status, error, result, steps, model, auto_push, branch, created_at, updated_at
		FROM ms_tracker_jobs
	`); err != nil {
		return err
	}
	if _, err := dst.Exec(`
		SELECT setval(pg_get_serial_sequence('pack_tracker_tasks', 'id'),
			COALESCE((SELECT MAX(id) FROM pack_tracker_tasks), 1), true)
	`); err != nil {
		return err
	}
	if _, err := dst.Exec(`
		SELECT setval(pg_get_serial_sequence('ms_tracker_jobs', 'id'),
			COALESCE((SELECT MAX(id) FROM ms_tracker_jobs), 1), true)
	`); err != nil {
		return err
	}
	return nil
}

func copyIfDestEmpty(src, dst *sql.DB, table, insertSQL string) error {
	if !tableExists(src, table) {
		return nil
	}
	var destN, srcN int
	if tableExists(dst, table) {
		if err := dst.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&destN); err != nil {
			return err
		}
	}
	if destN > 0 {
		return nil
	}
	if err := src.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&srcN); err != nil {
		return err
	}
	if srcN == 0 {
		return nil
	}
	return copyTableRows(src, dst, table, insertSQL)
}

func copyTableRows(src, dst *sql.DB, table, insertSQL string) error {
	_ = table
	rows, err := src.Query(selectSQLFromInsert(insertSQL))
	if err != nil {
		return fmt.Errorf("читать %s: %w", table, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	tx, err := dst.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ","), strings.Join(placeholders, ","))
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		if _, err := tx.Exec(q, vals...); err != nil {
			return fmt.Errorf("писать %s: %w", table, err)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return tx.Commit()
}

func selectSQLFromInsert(insertSQL string) string {
	low := strings.ToLower(insertSQL)
	i := strings.Index(low, " select ")
	if i < 0 {
		i = strings.Index(low, "select ")
	}
	if i < 0 {
		return insertSQL
	}
	sel := strings.TrimSpace(insertSQL[i:])
	sel = strings.TrimSuffix(sel, "ON CONFLICT (id) DO NOTHING")
	sel = strings.TrimSpace(sel)
	return sel
}

func tableExists(db *sql.DB, name string) bool {
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = $1
	`, name).Scan(&n)
	return err == nil && n > 0
}

func dropTrackerTablesFromLeo(db *sql.DB) error {
	_, err := db.Exec(`
		DROP TABLE IF EXISTS pack_tracker_attachments;
		DROP TABLE IF EXISTS pack_tracker_tasks;
		DROP TABLE IF EXISTS leo_autonomy;
		DROP TABLE IF EXISTS ms_tracker_jobs;
	`)
	return err
}
