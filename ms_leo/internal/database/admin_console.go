package database

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Консоль админа: список таблиц, страница таблицы и произвольный SQL.
// По умолчанию только чтение — редактор открыт админам мини-аппа, и одна
// опечатка в UPDATE без WHERE стоила бы базы стаи.

// AdminTableInfo — таблица проекта и оценка числа строк из статистики Postgres.
type AdminTableInfo struct {
	Name string `json:"name"`
	Rows int64  `json:"rows"`
}

// AdminQueryResult — результат выборки: колонки и строки в текстовом виде.
type AdminQueryResult struct {
	Columns  []string   `json:"columns"`
	Rows     [][]string `json:"rows"`
	Truncate bool       `json:"truncated"`
	Took     string     `json:"took"`
}

// AdminTableNames — имена таблиц public-схемы.
func (d *Database) AdminTableNames() ([]string, error) {
	if d == nil || d.db == nil {
		return nil, fmt.Errorf("nil database")
	}
	rows, err := d.db.Query(`
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r'
		ORDER BY c.relname
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, 64)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// AdminListTables — таблицы с ТОЧНЫМ числом строк.
//
// reltuples из pg_class врёт: у таблицы, которую ещё не трогал autovacuum, там
// ноль, и в списке «migrations» выглядела бы пустой. База стаи небольшая,
// поэтому считаем честно — одним запросом COUNT(*) по всем таблицам сразу.
func (d *Database) AdminListTables() ([]AdminTableInfo, error) {
	names, err := d.AdminTableNames()
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	parts := make([]string, 0, len(names))
	for _, n := range names {
		if !identRe.MatchString(n) {
			continue
		}
		parts = append(parts, fmt.Sprintf("SELECT %s AS t, COUNT(*) AS c FROM %q", quoteLiteral(n), n))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rows, err := d.db.QueryContext(ctx, strings.Join(parts, " UNION ALL ")+" ORDER BY t")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AdminTableInfo, 0, len(names))
	for rows.Next() {
		var t AdminTableInfo
		if err := rows.Scan(&t.Name, &t.Rows); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func quoteLiteral(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

// AdminColumnInfo — колонка таблицы для вкладки «Структура».
type AdminColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default"`
	PK       bool   `json:"pk"`
}

// AdminTableColumns — структура таблицы: тип, NULL, значение по умолчанию, ключ.
func (d *Database) AdminTableColumns(table string) ([]AdminColumnInfo, error) {
	if d == nil || d.db == nil {
		return nil, fmt.Errorf("nil database")
	}
	if !identRe.MatchString(table) {
		return nil, fmt.Errorf("недопустимое имя таблицы")
	}
	rows, err := d.db.Query(`
		SELECT a.attname,
		       format_type(a.atttypid, a.atttypmod),
		       NOT a.attnotnull,
		       COALESCE(pg_get_expr(ad.adbin, ad.adrelid), ''),
		       COALESCE(i.indisprimary, false)
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attrdef ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
		LEFT JOIN pg_index i ON i.indrelid = c.oid AND a.attnum = ANY(i.indkey) AND i.indisprimary
		WHERE n.nspname = 'public' AND c.relname = $1 AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AdminColumnInfo, 0, 16)
	for rows.Next() {
		var c AdminColumnInfo
		if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default, &c.PK); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// AdminTablePage — страница таблицы. Имя проверяем по белому списку из
// pg_class: подставлять его в SQL иначе нельзя, параметром имя не передать.
func (d *Database) AdminTablePage(table string, limit, offset int, orderBy string, desc bool) (AdminQueryResult, error) {
	var out AdminQueryResult
	if !identRe.MatchString(table) {
		return out, fmt.Errorf("недопустимое имя таблицы")
	}
	names, err := d.AdminTableNames()
	if err != nil {
		return out, err
	}
	found := false
	for _, n := range names {
		if n == table {
			found = true
			break
		}
	}
	if !found {
		return out, fmt.Errorf("нет такой таблицы")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	order := ""
	if strings.TrimSpace(orderBy) != "" {
		// Имя колонки в ORDER BY параметром не передать, поэтому сверяем его со
		// структурой таблицы: подставлять пришедшее с клиента нельзя.
		cols, err := d.AdminTableColumns(table)
		if err != nil {
			return out, err
		}
		ok := false
		for _, c := range cols {
			if c.Name == orderBy {
				ok = true
				break
			}
		}
		if !ok {
			return out, fmt.Errorf("нет такой колонки")
		}
		dir := "ASC"
		if desc {
			dir = "DESC"
		}
		order = fmt.Sprintf(" ORDER BY %q %s", orderBy, dir)
	}
	return d.AdminRunQuery(fmt.Sprintf("SELECT * FROM %q%s LIMIT %d OFFSET %d", table, order, limit, offset))
}

var readOnlyPrefixes = []string{"select", "with", "explain", "show", "table", "values"}

// adminStripSQLNoise убирает комментарии и лишние пробелы: без этого
// «-- заметка\nSELECT …» не проходил бы проверку на чтение.
func adminStripSQLNoise(sql string) string {
	s := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(sql, " ")
	lines := make([]string, 0, 8)
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// AdminQueryIsReadOnly — запрос только читает?
func AdminQueryIsReadOnly(sql string) bool {
	s := strings.ToLower(adminStripSQLNoise(sql))
	if s == "" {
		return false
	}
	// Несколько запросов через ';' не пропускаем: второй мог бы быть DELETE.
	if strings.Contains(strings.TrimSuffix(s, ";"), ";") {
		return false
	}
	for _, p := range readOnlyPrefixes {
		if strings.HasPrefix(s, p+" ") || s == p {
			return true
		}
	}
	return false
}

// AdminRunQuery выполняет читающий запрос в read-only транзакции с таймаутом.
// Возвращает не больше 200 строк: результат уезжает в телефон.
func (d *Database) AdminRunQuery(query string) (AdminQueryResult, error) {
	var out AdminQueryResult
	if d == nil || d.db == nil {
		return out, fmt.Errorf("nil database")
	}
	if !AdminQueryIsReadOnly(query) {
		return out, fmt.Errorf("только чтение: SELECT / WITH / EXPLAIN / SHOW / TABLE / VALUES, один запрос без «;»")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	tx, err := d.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback() }()

	started := time.Now()
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return out, err
	}
	out.Columns = cols
	const maxRows = 200
	for rows.Next() {
		if len(out.Rows) >= maxRows {
			out.Truncate = true
			break
		}
		cells := make([]any, len(cols))
		for i := range cells {
			cells[i] = new(sql.NullString)
		}
		if err := rows.Scan(cells...); err != nil {
			return out, err
		}
		row := make([]string, len(cols))
		for i, c := range cells {
			if v, ok := c.(*sql.NullString); ok && v.Valid {
				row[i] = v.String
			} else {
				row[i] = "∅"
			}
		}
		out.Rows = append(out.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	out.Took = time.Since(started).Round(time.Millisecond).String()
	return out, nil
}

// AdminPaymentSum — сколько получено в одной валюте за период.
type AdminPaymentSum struct {
	Currency    string `json:"currency"`
	Count       int64  `json:"count"`
	AmountMinor int64  `json:"amount_minor"`
}

// AdminSumCompletedPayments — оплаченные заявки с момента since, по валютам.
func (d *Database) AdminSumCompletedPayments(packChatID int64, since time.Time) ([]AdminPaymentSum, error) {
	if d == nil || d.db == nil || packChatID == 0 {
		return nil, nil
	}
	rows, err := d.db.Query(`
		SELECT COALESCE(NULLIF(BTRIM(currency), ''), 'RUB') AS cur,
		       COUNT(*),
		       COALESCE(SUM(total_amount_minor), 0)
		FROM paywall_access_requests
		WHERE monetized_chat_id = $1
		  AND status = 'completed'
		  AND COALESCE(completed_at, created_at) >= $2
		GROUP BY cur
		ORDER BY cur
	`, packChatID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AdminPaymentSum, 0, 4)
	for rows.Next() {
		var s AdminPaymentSum
		if err := rows.Scan(&s.Currency, &s.Count, &s.AmountMinor); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
