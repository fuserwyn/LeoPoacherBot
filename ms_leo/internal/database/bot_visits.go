package database

import (
	"fmt"
	"time"
)

// BotVisit — одна запись посещения бота.
type BotVisit struct {
	ID        int64
	UserID    int64
	Username  string
	FirstName string
	LastName  string
	VisitedAt time.Time
}

// BotVisitStats — агрегированная статистика посещений.
type BotVisitStats struct {
	TotalVisits   int64
	UniqueUsers   int64
	TodayVisits   int64
	WeekVisits    int64
	MonthVisits   int64
	TopVisitors   []BotVisitUser
	RecentVisits  []BotVisit
}

// BotVisitUser — пользователь с кол-вом визитов (для топа).
type BotVisitUser struct {
	UserID    int64
	Username  string
	FirstName string
	Visits    int64
	LastVisit time.Time
}

// RecordBotVisit — фиксирует посещение бота пользователем.
func (d *Database) RecordBotVisit(userID int64, username, firstName, lastName string) error {
	if userID == 0 {
		return nil
	}
	_, err := d.db.Exec(`
		INSERT INTO bot_visits (user_id, username, first_name, last_name, visited_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, userID, username, firstName, lastName)
	if err != nil {
		return fmt.Errorf("record bot visit: %w", err)
	}
	return nil
}

// GetBotVisitStats — возвращает сводную статистику посещений.
func (d *Database) GetBotVisitStats() (BotVisitStats, error) {
	var stats BotVisitStats

	// Всего визитов и уникальных юзеров
	err := d.db.QueryRow(`
		SELECT COUNT(*), COUNT(DISTINCT user_id)
		FROM bot_visits
	`).Scan(&stats.TotalVisits, &stats.UniqueUsers)
	if err != nil {
		return stats, fmt.Errorf("visit totals: %w", err)
	}

	// Сегодня (по UTC)
	err = d.db.QueryRow(`
		SELECT COUNT(*) FROM bot_visits
		WHERE visited_at >= date_trunc('day', NOW() AT TIME ZONE 'Europe/Moscow') AT TIME ZONE 'Europe/Moscow'
	`).Scan(&stats.TodayVisits)
	if err != nil {
		return stats, fmt.Errorf("visit today: %w", err)
	}

	// За 7 дней
	err = d.db.QueryRow(`
		SELECT COUNT(*) FROM bot_visits
		WHERE visited_at >= NOW() - INTERVAL '7 days'
	`).Scan(&stats.WeekVisits)
	if err != nil {
		return stats, fmt.Errorf("visit week: %w", err)
	}

	// За 30 дней
	err = d.db.QueryRow(`
		SELECT COUNT(*) FROM bot_visits
		WHERE visited_at >= NOW() - INTERVAL '30 days'
	`).Scan(&stats.MonthVisits)
	if err != nil {
		return stats, fmt.Errorf("visit month: %w", err)
	}

	// Топ-10 по количеству визитов
	rows, err := d.db.Query(`
		SELECT user_id,
		       COALESCE(MAX(username), '') AS username,
		       COALESCE(MAX(first_name), '') AS first_name,
		       COUNT(*) AS visits,
		       MAX(visited_at) AS last_visit
		FROM bot_visits
		GROUP BY user_id
		ORDER BY visits DESC, last_visit DESC
		LIMIT 10
	`)
	if err != nil {
		return stats, fmt.Errorf("visit top: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var u BotVisitUser
		if err := rows.Scan(&u.UserID, &u.Username, &u.FirstName, &u.Visits, &u.LastVisit); err != nil {
			continue
		}
		stats.TopVisitors = append(stats.TopVisitors, u)
	}

	// Последние 20 визитов
	stats.RecentVisits, err = d.GetRecentBotVisits(20)
	if err != nil {
		return stats, fmt.Errorf("visit recent: %w", err)
	}

	return stats, nil
}

// GetRecentBotVisits — последние N посещений.
func (d *Database) GetRecentBotVisits(limit int) ([]BotVisit, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.Query(`
		SELECT id, user_id,
		       COALESCE(username, '') AS username,
		       COALESCE(first_name, '') AS first_name,
		       COALESCE(last_name, '') AS last_name,
		       visited_at
		FROM bot_visits
		ORDER BY visited_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent bot visits: %w", err)
	}
	defer rows.Close()
	var out []BotVisit
	for rows.Next() {
		var v BotVisit
		if err := rows.Scan(&v.ID, &v.UserID, &v.Username, &v.FirstName, &v.LastName, &v.VisitedAt); err != nil {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// GetDailyBotVisitsLast30Days — визиты по дням за последние 30 дней (для графика/таблицы).
func (d *Database) GetDailyBotVisitsLast30Days() ([]DailyVisitCount, error) {
	rows, err := d.db.Query(`
		SELECT date_trunc('day', visited_at AT TIME ZONE 'Europe/Moscow')::date AS day,
		       COUNT(*) AS visits,
		       COUNT(DISTINCT user_id) AS unique_users
		FROM bot_visits
		WHERE visited_at >= NOW() - INTERVAL '30 days'
		GROUP BY day
		ORDER BY day DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("daily bot visits: %w", err)
	}
	defer rows.Close()
	var out []DailyVisitCount
	for rows.Next() {
		var d DailyVisitCount
		if err := rows.Scan(&d.Day, &d.Visits, &d.UniqueUsers); err != nil {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// DailyVisitCount — посещения за один день.
type DailyVisitCount struct {
	Day        time.Time
	Visits     int64
	UniqueUsers int64
}
