package database

import (
	"database/sql"
	"fmt"
	"time"

	"leo-bot/internal/logger"
	"leo-bot/internal/models"
	"leo-bot/internal/utils"

	_ "github.com/lib/pq"
)

type Database struct {
	db     *sql.DB
	logger logger.Logger
}

func New(databaseURL string) (*Database, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Настраиваем пул соединений
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &Database{
		db: db,
	}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

// CreateTables создает таблицы в базе данных, если они не существуют
func (d *Database) CreateTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS message_log (
			user_id BIGINT,
			username TEXT DEFAULT '',
			chat_id BIGINT,
			calories INTEGER DEFAULT 0,
			streak_days INTEGER DEFAULT 0,
			last_training_date TEXT,
			last_message TEXT NOT NULL,
			has_training_done BOOLEAN DEFAULT FALSE,
			has_sick_leave BOOLEAN DEFAULT FALSE,
			has_healthy BOOLEAN DEFAULT FALSE,
			is_deleted BOOLEAN DEFAULT FALSE,
			timer_start_time TEXT,
			sick_leave_start_time TEXT,
			sick_leave_end_time TEXT,
			sick_time TEXT,
			rest_time_till_del TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, chat_id)
		)`,
		`CREATE TABLE IF NOT EXISTS training_log (
			user_id BIGINT PRIMARY KEY,
			username TEXT DEFAULT '',
			last_report TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Запускаем миграции для обновления схемы
	if err := d.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// SaveMessageLog сохраняет информацию о сообщении
func (d *Database) SaveMessageLog(msg *models.MessageLog) error {
	query := `
		INSERT INTO message_log (user_id, username, chat_id, calories, streak_days, last_training_date, last_message, has_training_done, has_sick_leave, has_healthy, is_deleted, timer_start_time, sick_leave_start_time, sick_leave_end_time, sick_time, rest_time_till_del, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (user_id, chat_id) 
		DO UPDATE SET 
			username = EXCLUDED.username,
			calories = EXCLUDED.calories,
			streak_days = EXCLUDED.streak_days,
			last_training_date = EXCLUDED.last_training_date,
			last_message = EXCLUDED.last_message,
			has_training_done = EXCLUDED.has_training_done,
			has_sick_leave = EXCLUDED.has_sick_leave,
			has_healthy = EXCLUDED.has_healthy,
			is_deleted = EXCLUDED.is_deleted,
			timer_start_time = EXCLUDED.timer_start_time,
			sick_leave_start_time = EXCLUDED.sick_leave_start_time,
			sick_leave_end_time = EXCLUDED.sick_leave_end_time,
			sick_time = EXCLUDED.sick_time,
			rest_time_till_del = EXCLUDED.rest_time_till_del,
			updated_at = $17
	`

	// Используем московское время
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())

	// Временное логирование для отладки
	fmt.Printf("DEBUG: Saving to DB - UserID: %d, TimerStartTime: %v, SickLeaveStartTime: %v, RestTimeTillDel: %v\n",
		msg.UserID, msg.TimerStartTime, msg.SickLeaveStartTime, msg.RestTimeTillDel)

	result, err := d.db.Exec(query,
		msg.UserID, msg.Username, msg.ChatID, msg.Calories, msg.StreakDays, msg.LastTrainingDate, msg.LastMessage, msg.HasTrainingDone,
		msg.HasSickLeave, msg.HasHealthy, msg.IsDeleted, msg.TimerStartTime, msg.SickLeaveStartTime, msg.SickLeaveEndTime, msg.SickTime, msg.RestTimeTillDel, moscowTime)

	if err != nil {
		fmt.Printf("DEBUG: Save error: %v\n", err)
		return err
	}

	// Проверяем, что именно произошло (INSERT или UPDATE)
	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("DEBUG: Rows affected: %d\n", rowsAffected)

	return err
}

// GetMessageLog получает информацию о сообщении пользователя
func (d *Database) GetMessageLog(userID, chatID int64) (*models.MessageLog, error) {
	query := `
		SELECT user_id, username, chat_id, calories, streak_days, last_training_date, last_message, has_training_done, has_sick_leave, has_healthy, is_deleted,
		       timer_start_time, sick_leave_start_time, sick_leave_end_time, sick_time, rest_time_till_del, created_at, updated_at
		FROM message_log 
		WHERE user_id = $1 AND chat_id = $2
	`

	var msg models.MessageLog
	err := d.db.QueryRow(query, userID, chatID).Scan(
		&msg.UserID, &msg.Username, &msg.ChatID, &msg.Calories, &msg.StreakDays, &msg.LastTrainingDate, &msg.LastMessage, &msg.HasTrainingDone,
		&msg.HasSickLeave, &msg.HasHealthy, &msg.IsDeleted, &msg.TimerStartTime, &msg.SickLeaveStartTime, &msg.SickLeaveEndTime, &msg.SickTime, &msg.RestTimeTillDel,
		&msg.CreatedAt, &msg.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// Временное логирование для отладки
	fmt.Printf("DEBUG: Retrieved from DB - UserID: %d, HasSickLeave: %t, TimerStartTime: %v, SickLeaveStartTime: %v, RestTimeTillDel: %v\n",
		msg.UserID, msg.HasSickLeave, msg.TimerStartTime, msg.SickLeaveStartTime, msg.RestTimeTillDel)
	fmt.Printf("DEBUG: Retrieved from DB - StreakDays: %d, LastTrainingDate: %v\n",
		msg.StreakDays, msg.LastTrainingDate)

	return &msg, nil
}

// GetUsersByChatID получает всех пользователей в чате
func (d *Database) GetUsersByChatID(chatID int64) ([]*models.MessageLog, error) {
	query := `
		SELECT user_id, username, chat_id, calories, streak_days, last_training_date, last_message, has_training_done, has_sick_leave, has_healthy, is_deleted,
		       timer_start_time, sick_leave_start_time, sick_leave_end_time, sick_time, rest_time_till_del, created_at, updated_at
		FROM message_log 
		WHERE chat_id = $1 AND is_deleted = FALSE
		ORDER BY calories DESC, last_message DESC
	`

	rows, err := d.db.Query(query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.MessageLog
	for rows.Next() {
		var msg models.MessageLog
		err := rows.Scan(
			&msg.UserID, &msg.Username, &msg.ChatID, &msg.Calories, &msg.StreakDays, &msg.LastTrainingDate, &msg.LastMessage, &msg.HasTrainingDone,
			&msg.HasSickLeave, &msg.HasHealthy, &msg.IsDeleted, &msg.TimerStartTime, &msg.SickLeaveStartTime, &msg.SickLeaveEndTime, &msg.SickTime, &msg.RestTimeTillDel,
			&msg.CreatedAt, &msg.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, &msg)
	}

	return users, nil
}

// SaveTrainingLog сохраняет отчет о тренировке
func (d *Database) SaveTrainingLog(training *models.TrainingLog) error {
	query := `
		INSERT INTO training_log (user_id, username, last_report, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) 
		DO UPDATE SET 
			username = EXCLUDED.username,
			last_report = EXCLUDED.last_report,
			updated_at = $4
	`

	// Используем московское время
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(query, training.UserID, training.Username, training.LastReport, moscowTime)
	return err
}

// GetDatabaseStats получает статистику базы данных
func (d *Database) GetDatabaseStats() (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_users,
			COUNT(CASE WHEN has_training_done = true THEN 1 END) as training_done,
			COUNT(CASE WHEN has_sick_leave = true THEN 1 END) as sick_leave,
			COUNT(CASE WHEN has_healthy = true THEN 1 END) as healthy
		FROM message_log
	`

	var stats struct {
		TotalUsers   int `db:"total_users"`
		TrainingDone int `db:"training_done"`
		SickLeave    int `db:"sick_leave"`
		Healthy      int `db:"healthy"`
	}

	err := d.db.QueryRow(query).Scan(&stats.TotalUsers, &stats.TrainingDone, &stats.SickLeave, &stats.Healthy)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_users":   stats.TotalUsers,
		"training_done": stats.TrainingDone,
		"sick_leave":    stats.SickLeave,
		"healthy":       stats.Healthy,
	}, nil
}

// AddCalories добавляет калории пользователю
func (d *Database) AddCalories(userID, chatID int64, calories int) error {
	query := `
		UPDATE message_log 
		SET calories = calories + $3, updated_at = $4
		WHERE user_id = $1 AND chat_id = $2
	`
	// Используем московское время
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(query, userID, chatID, calories, moscowTime)
	return err
}

// GetUserCalories получает калории пользователя
func (d *Database) GetUserCalories(userID, chatID int64) (int, error) {
	query := `
		SELECT calories FROM message_log 
		WHERE user_id = $1 AND chat_id = $2
	`
	var calories int
	err := d.db.QueryRow(query, userID, chatID).Scan(&calories)
	if err != nil {
		return 0, err
	}
	return calories, nil
}

// UpdateStreak обновляет серию тренировок пользователя
func (d *Database) UpdateStreak(userID, chatID int64, streakDays int, lastTrainingDate string) error {
	query := `
		UPDATE message_log 
		SET streak_days = $3, last_training_date = $4, updated_at = $5
		WHERE user_id = $1 AND chat_id = $2
	`
	// Используем московское время
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(query, userID, chatID, streakDays, lastTrainingDate, moscowTime)
	return err
}

// MarkUserAsDeleted помечает пользователя как удаленного
func (d *Database) MarkUserAsDeleted(userID, chatID int64) error {
	query := `
		UPDATE message_log 
		SET is_deleted = TRUE, updated_at = $3
		WHERE user_id = $1 AND chat_id = $2
	`
	// Используем московское время
	moscowTime := utils.FormatMoscowTime(utils.GetMoscowTime())
	_, err := d.db.Exec(query, userID, chatID, moscowTime)
	return err
}

// GetTopUsers получает топ пользователей по калориям
func (d *Database) GetTopUsers(chatID int64, limit int) ([]*models.MessageLog, error) {
	query := `
		SELECT user_id, username, chat_id, calories, streak_days, last_training_date, last_message, has_training_done, has_sick_leave, has_healthy, is_deleted,
		       timer_start_time, sick_leave_start_time, sick_leave_end_time, sick_time, rest_time_till_del, created_at, updated_at
		FROM message_log 
		WHERE chat_id = $1 AND calories > 0 AND is_deleted = FALSE
		ORDER BY calories DESC, last_message DESC
		LIMIT $2
	`

	rows, err := d.db.Query(query, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.MessageLog
	for rows.Next() {
		var msg models.MessageLog
		err := rows.Scan(
			&msg.UserID, &msg.Username, &msg.ChatID, &msg.Calories, &msg.StreakDays, &msg.LastTrainingDate, &msg.LastMessage, &msg.HasTrainingDone,
			&msg.HasSickLeave, &msg.HasHealthy, &msg.IsDeleted, &msg.TimerStartTime, &msg.SickLeaveStartTime, &msg.SickLeaveEndTime, &msg.SickTime, &msg.RestTimeTillDel,
			&msg.CreatedAt, &msg.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, &msg)
	}

	return users, nil
}
